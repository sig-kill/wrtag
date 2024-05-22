package wrtag

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/originfile"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
	"go.senan.xyz/wrtag/tags"
)

var (
	ErrScoreTooLow        = errors.New("score too low")
	ErrTrackCountMismatch = errors.New("track count mismatch")
	ErrNoTracks           = errors.New("no tracks in dir")
	ErrSelfCopy           = errors.New("can't copy self to self")
)

const minScore = 95

const (
	thresholdSizeClean uint64 = 20 * 1e6   // 20 MB
	thresholdSizeTrim  uint64 = 3000 * 1e6 // 3000 MB
)

type MusicbrainzClient interface {
	SearchRelease(ctx context.Context, q musicbrainz.ReleaseQuery) (*musicbrainz.Release, error)
	GetCover(ctx context.Context, release *musicbrainz.Release) ([]byte, string, error)
}

type SearchResult struct {
	Release       *musicbrainz.Release
	Score         float64
	DestDir       string
	Diff          []tagmap.Diff
	ResearchLinks []researchlink.SearchResult
	OriginFile    *originfile.OriginFile
}

type ImportCondition uint8

const (
	HighScore ImportCondition = iota
	HighScoreOrMBID
	Confirm
)

type Addon = tagmap.Addon

func ProcessDir(
	ctx context.Context,
	mb MusicbrainzClient, pathFormat *pathformat.Format, tagWeights tagmap.TagWeights, researchLinkQuerier *researchlink.Querier, keepFiles map[string]struct{}, addons []Addon,
	op FileSystemOperation, srcDir string,
	useMBID string, importCondition ImportCondition,
) (*SearchResult, error) {
	srcDir, _ = filepath.Abs(srcDir)

	cover, paths, tagFiles, err := ReadAlbumDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	searchFile := tagFiles[0]

	var mbid = searchFile.Read(tags.MBReleaseID)
	if useMBID != "" {
		mbid = useMBID
	}

	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      mbid,
		MBArtistID:       searchFile.Read(tags.MBArtistID),
		MBReleaseGroupID: searchFile.Read(tags.MBReleaseGroupID),
		Release:          searchFile.Read(tags.Album),
		Artist:           cmp.Or(searchFile.Read(tags.AlbumArtist), searchFile.Read(tags.Artist)),
		Date:             searchFile.ReadTime(tags.Date),
		Format:           searchFile.Read(tags.MediaFormat),
		Label:            searchFile.Read(tags.Label),
		CatalogueNum:     searchFile.Read(tags.CatalogueNum),
		NumTracks:        len(tagFiles),
	}

	// parse https://github.com/x1ppy/gazelle-origin files, if one exists
	originFile, err := originfile.Find(srcDir)
	if err != nil {
		return nil, fmt.Errorf("find origin file: %w", err)
	}

	if mbid == "" {
		if err := extendQueryWithOriginFile(&query, originFile); err != nil {
			return nil, fmt.Errorf("use origin file: %w", err)
		}
	}

	release, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search musicbrainz: %w", err)
	}

	var researchLinks []researchlink.SearchResult
	if researchLinkQuerier != nil {
		var artist = searchFile.Read(tags.AlbumArtist)
		if artist == "" {
			artist = searchFile.Read(tags.Artist)
		}
		researchLinks, err = researchLinkQuerier.Search(researchlink.Query{
			Artist: artist,
			Album:  searchFile.Read(tags.Album),
			Date:   searchFile.ReadTime(tags.Date),
		})
		if err != nil {
			return nil, fmt.Errorf("research querier search: %w", err)
		}
	}

	releaseTracks := musicbrainz.FlatTracks(release.Media)
	if len(releaseTracks) != len(tagFiles) {
		return &SearchResult{release, 0, "", nil, researchLinks, originFile}, fmt.Errorf("%w: %d remote / %d local", ErrTrackCountMismatch, len(releaseTracks), len(tagFiles))
	}

	score, diff := tagmap.DiffRelease(tagWeights, release, tagFiles)

	var shouldImport bool
	switch importCondition {
	case HighScoreOrMBID:
		shouldImport = score >= minScore || mbid != ""
	case HighScore:
		shouldImport = score >= minScore
	case Confirm:
		shouldImport = true
	}

	if !shouldImport {
		return &SearchResult{release, score, "", diff, researchLinks, originFile}, ErrScoreTooLow
	}

	destDir, err := DestDir(pathFormat, release)
	if err != nil {
		return nil, fmt.Errorf("gen dest dir: %w", err)
	}
	destDir, _ = filepath.Abs(destDir)

	// lock both source and destination directories
	defer dirMu.Lock(srcDir)()
	if srcDir != destDir {
		defer dirMu.Lock(destDir)()
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)
	genres := musicbrainz.AnyGenres(release)
	isCompilation := musicbrainz.IsCompilation(release.ReleaseGroup)

	dc := NewDirContext()

	for i := range releaseTracks {
		releaseTrack, path := releaseTracks[i], paths[i]

		pathFormatData := pathformat.Data{Release: *release, Track: releaseTrack, TrackNum: i + 1, Ext: strings.ToLower(filepath.Ext(path)), IsCompilation: isCompilation}
		destPath, err := pathFormat.Execute(pathFormatData)
		if err != nil {
			return nil, fmt.Errorf("create path: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return nil, fmt.Errorf("create dest path: %w", err)
		}
		if err := op.ProcessFile(dc, path, destPath); err != nil {
			return nil, fmt.Errorf("process path %q: %w", filepath.Base(destPath), err)
		}

		if op.ReadOnly() {
			continue
		}

		tagFile, err := tags.Read(destPath)
		if err != nil {
			return nil, fmt.Errorf("read tag file: %w", err)
		}
		if err := tagmap.WriteFile(ctx, addons, release, labelInfo, genres, &releaseTrack, i, tagFile); err != nil {
			return nil, fmt.Errorf("write tag file: %w", err)
		}

		tagFile.Close()
	}

	if cover == "" {
		cover, err = tryDownloadMusicbrainzCover(ctx, mb, destDir, release)
		if err != nil {
			return nil, fmt.Errorf("try download mb cover: %w", err)
		}
		defer os.Remove(cover)
	}
	if cover != "" {
		coverDest := filepath.Join(destDir, "cover"+filepath.Ext(cover))
		if err := op.ProcessFile(dc, cover, coverDest); err != nil {
			return nil, fmt.Errorf("move file to dest: %w", err)
		}
	}

	for kf := range keepFiles {
		if err := op.ProcessFile(dc, filepath.Join(srcDir, kf), filepath.Join(destDir, kf)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("process keep file %q: %w", kf, err)
		}
	}

	if err := trimDir(dc, destDir, op.ReadOnly()); err != nil {
		return nil, fmt.Errorf("trim: %w", err)
	}

	if srcDir != destDir {
		if err := op.CleanDir(dc, pathFormat.Root(), srcDir); err != nil {
			return nil, fmt.Errorf("clean: %w", err)
		}
	}

	return &SearchResult{release, score, destDir, diff, researchLinks, originFile}, nil
}

func ReadAlbumDir(path string) (string, []string, []*tags.File, error) {
	mainPaths, err := fileutil.GlobDir(path, "*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("glob dir: %w", err)
	}
	discPaths, err := fileutil.GlobDir(path, "*/*") // recurse once for any disc1/ disc2/ dirs
	if err != nil {
		return "", nil, nil, fmt.Errorf("glob dir for discs: %w", err)
	}

	type pathFile struct {
		path string
		f    *tags.File
	}

	var cover string
	var pathFiles []pathFile
	for _, path := range append(mainPaths, discPaths...) {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".bmp", ".gif":
			cover = path
			continue
		}

		if tags.CanRead(path) {
			file, err := tags.Read(path)
			if err != nil {
				return "", nil, nil, fmt.Errorf("read track: %w", err)
			}
			pathFiles = append(pathFiles, pathFile{path, file})
			file.Close()
		}
	}
	if len(pathFiles) == 0 {
		return "", nil, nil, ErrNoTracks
	}

	{
		// validate we aren't accidentally importing something like an artist folder, which may look
		// like a multi disc album to us, but will have all its tracks in one subdirectory
		discDirs := map[string]struct{}{}
		for _, pf := range pathFiles {
			discDirs[filepath.Dir(pf.path)] = struct{}{}
		}
		if len(discDirs) == 1 && filepath.Dir(pathFiles[0].path) != filepath.Clean(path) {
			return "", nil, nil, fmt.Errorf("validate tree: %w", ErrNoTracks)
		}
	}

	slices.SortFunc(pathFiles, func(a, b pathFile) int {
		return cmp.Or(
			cmp.Compare(a.f.ReadNum(tags.DiscNumber), b.f.ReadNum(tags.DiscNumber)),
			cmp.Compare(a.f.ReadNum(tags.TrackNumber), b.f.ReadNum(tags.TrackNumber)),
			cmp.Compare(a.path, b.path),
		)
	})

	paths := make([]string, 0, len(pathFiles))
	files := make([]*tags.File, 0, len(pathFiles))
	for _, pf := range pathFiles {
		paths = append(paths, pf.path)
		files = append(files, pf.f)
	}

	return cover, paths, files, nil
}

func DestDir(pathFormat *pathformat.Format, release *musicbrainz.Release) (string, error) {
	dummyTrack := musicbrainz.Track{Title: "track"}
	path, err := pathFormat.Execute(pathformat.Data{Release: *release, Track: dummyTrack, TrackNum: 1, Ext: ".eg"})
	if err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	dir := filepath.Dir(path)
	return dir, nil
}

var _ FileSystemOperation = (*Move)(nil)
var _ FileSystemOperation = (*Copy)(nil)

type FileSystemOperation interface {
	ReadOnly() bool
	ProcessFile(dc DirContext, src, dest string) error
	CleanDir(dc DirContext, limit string, src string) error
}

type DirContext struct {
	knownDestPaths map[string]struct{}
}

func NewDirContext() DirContext {
	return DirContext{knownDestPaths: map[string]struct{}{}}
}

type Move struct {
	DryRun bool
}

func (m Move) ReadOnly() bool {
	return m.DryRun
}

func (m Move) ProcessFile(dc DirContext, src, dest string) error {
	dest, _ = filepath.Abs(dest)
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if m.DryRun {
		slog.Info("[dry run] move", "src", src, "dest", dest)
		return nil
	}

	if err := os.Rename(src, dest); err != nil {
		if errNo := syscall.Errno(0); errors.As(err, &errNo) && errNo == 18 /*  invalid cross-device link */ {
			// we tried to rename across filesystems, copy and delete instead
			if err := (Copy{}).ProcessFile(dc, src, dest); err != nil {
				return fmt.Errorf("copy from move: %w", err)
			}
			if err := os.Remove(src); err != nil {
				return fmt.Errorf("remove from move: %w", err)
			}
			return nil
		}
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func (m Move) CleanDir(dc DirContext, limit string, src string) error {
	if limit == "" {
		panic("empty limit dir")
	}

	if err := safeRemoveAll(src, m.DryRun); err != nil {
		return fmt.Errorf("src dir: %w", err)
	}

	if !strings.HasPrefix(src, limit) {
		return nil
	}

	// clean all src's parents if this path is in our control

	cleanMu.Lock()
	defer cleanMu.Unlock()

	for d := filepath.Dir(src); d != filepath.Clean(limit); d = filepath.Dir(d) {
		if err := safeRemoveAll(d, m.DryRun); err != nil {
			return fmt.Errorf("src dir parent: %w", err)
		}
	}
	return nil
}

type Copy struct {
	DryRun bool
}

func (c Copy) ReadOnly() bool {
	return c.DryRun
}

func (c Copy) ProcessFile(dc DirContext, src, dest string) error {
	dest, _ = filepath.Abs(dest)
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.DryRun {
		slog.Info("[dry run] copy", "src", src, "dest", dest)
		return nil
	}

	srcf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer srcf.Close()

	destf, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("open dest: %w", err)
	}
	defer destf.Close()

	if _, err := io.Copy(destf, srcf); err != nil {
		return fmt.Errorf("do copy: %w", err)
	}
	return nil
}

func (Copy) CleanDir(dc DirContext, limit string, src string) error {
	return nil
}

func trimDir(dc DirContext, dest string, dryRun bool) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var toDelete []string
	var size uint64
	for _, entry := range entries {
		path := filepath.Join(dest, entry.Name())
		path, _ = filepath.Abs(path)
		if _, ok := dc.knownDestPaths[path]; ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get info: %w", err)
		}
		size += uint64(info.Size())
		toDelete = append(toDelete, path)
	}
	if size > thresholdSizeTrim {
		return fmt.Errorf("extra files were too big remove: %d/%d", size, thresholdSizeTrim)
	}

	var deleteErrs []error
	for _, p := range toDelete {
		if dryRun {
			slog.Info("[dry run] delete extra file", "file", p)
			continue
		}
		slog.Info("deleting extra file", "file", p)
		if err := os.Remove(p); err != nil {
			deleteErrs = append(deleteErrs, err)
		}
	}
	if err := errors.Join(deleteErrs...); err != nil {
		return fmt.Errorf("delete extra files: %w", err)
	}

	return nil
}

func tryDownloadMusicbrainzCover(ctx context.Context, mb MusicbrainzClient, tmpDir string, release *musicbrainz.Release) (string, error) {
	cover, ext, err := mb.GetCover(ctx, release)
	if err != nil {
		return "", fmt.Errorf("request cover url: %w", err)
	}
	if len(cover) == 0 {
		return "", nil
	}

	tmpf, err := os.CreateTemp(tmpDir, ".wrtag-cover-tmp-*"+ext)
	if err != nil {
		return "", err
	}
	defer tmpf.Close()

	n, _ := io.Copy(tmpf, bytes.NewReader(cover))
	slog.Debug("wrote cover to tmp", "size_bytes", n)

	return tmpf.Name(), nil
}

func dirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("get info %w", err)
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		return err
	})
	return size, err
}

func safeRemoveAll(src string, dryRun bool) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		// skip if we have any child directories
		if entry.IsDir() {
			return nil
		}
	}

	if dryRun {
		slog.Info("[dry run] remove all", "path", src)
		return nil
	}

	size, err := dirSize(src)
	if err != nil {
		return fmt.Errorf("get dir size for sanity check: %w", err)
	}
	if size > thresholdSizeClean {
		return fmt.Errorf("folder was too big for clean up: %d/%d", size, thresholdSizeClean)
	}

	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("error cleaning up folder: %w", err)
	}
	return nil
}

func extendQueryWithOriginFile(q *musicbrainz.ReleaseQuery, originFile *originfile.OriginFile) error {
	if originFile == nil {
		return nil
	}
	slog.Debug("using origin file", "file", originFile)

	if originFile.RecordLabel != "" {
		q.Label = originFile.RecordLabel
	}
	if originFile.CatalogueNumber != "" {
		q.CatalogueNum = originFile.CatalogueNumber
	}
	if originFile.Media != "" {
		media := originFile.Media
		media = strings.ReplaceAll(media, "WEB", "Digital Media")
		q.Format = media
	}
	if originFile.EditionYear > 0 {
		q.Date = time.Date(originFile.EditionYear, 0, 0, 0, 0, 0, 0, time.UTC)
	}
	return nil
}

var cleanMu sync.Mutex
var dirMu keyedMutex

type keyedMutex struct {
	sync.Map
}

func (km *keyedMutex) Lock(key string) func() {
	value, _ := km.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return func() { mu.Unlock() }
}
