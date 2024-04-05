package wrtag

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
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
	"go.senan.xyz/wrtag/tags/tagcommon"
)

var (
	ErrScoreTooLow        = errors.New("score too low")
	ErrTrackCountMismatch = errors.New("track count mismatch")
	ErrNoTracks           = errors.New("no tracks in dir")
	ErrSelfCopy           = errors.New("can't copy self to self")
)

const minScore = 92

const (
	thresholdSizeClean uint64 = 20 * 1e6   // 20 MB
	thresholdSizeTrim  uint64 = 1000 * 1e6 // 1000 MB
)

type FileSystemOperation interface {
	ReadOnly() bool
	ProcessFile(dc DirContext, src, dest string) error
	TrimDir(dc DirContext, src string) error                // remove unknown files
	CleanDir(dc DirContext, limit string, src string) error // remove dir and its parents
}

type DirContext struct {
	knownDestPaths map[string]struct{}
}

func NewDirContext() DirContext {
	return DirContext{knownDestPaths: map[string]struct{}{}}
}

var _ FileSystemOperation = (*Move)(nil)
var _ FileSystemOperation = (*Copy)(nil)

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
		log.Printf("[dry run] move %q -> %q", src, dest)
		return nil
	}

	if err := os.Rename(src, dest); err != nil {
		var errNo syscall.Errno
		if errors.As(err, &errNo) && errNo == 18 /*  invalid cross-device link */ {
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

func (m Move) TrimDir(dc DirContext, src string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var toDelete []string
	var size uint64
	for _, entry := range entries {
		path := filepath.Join(src, entry.Name())
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
		return fmt.Errorf("extra files were too big remove: %d/%d", size, thresholdSizeClean)
	}

	var deleteErrs []error
	for _, p := range toDelete {
		if m.DryRun {
			log.Printf("[dry run] delete extra file %q", p)
			continue
		}
		log.Printf("deleting extra file %q", p)
		if err := os.Remove(p); err != nil {
			deleteErrs = append(deleteErrs, err)
		}
	}
	if err := errors.Join(deleteErrs...); err != nil {
		return fmt.Errorf("delete extra files: %w", err)
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
	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.DryRun {
		log.Printf("[dry run] copy %q -> %q", src, dest)
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

func (Copy) TrimDir(dc DirContext, src string) error {
	return nil
}

func (Copy) CleanDir(dc DirContext, limit string, src string) error {
	return nil
}

func ReadDir(tg tagcommon.Reader, path string) (string, []string, []tagcommon.File, error) {
	mainPaths, err := fileutil.GlobBase(path, "*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("glob dir: %w", err)
	}
	discPaths, err := fileutil.GlobBase(path, "*/*") // recurse once for any disc1/ disc2/ dirs
	if err != nil {
		return "", nil, nil, fmt.Errorf("glob dir for discs: %w", err)
	}
	mainPaths = append(mainPaths, discPaths...)

	type pathFile struct {
		path string
		tagcommon.File
	}

	var cover string
	var pathFiles []pathFile
	for _, path := range mainPaths {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".bmp", ".gif":
			cover = path
			continue
		}

		if tg.CanRead(path) {
			file, err := tg.Read(path)
			if err != nil {
				return "", nil, nil, fmt.Errorf("read track: %w", err)
			}
			pathFiles = append(pathFiles, pathFile{path, file})
			_ = file.Close()
		}
	}
	if len(pathFiles) == 0 {
		return "", nil, nil, ErrNoTracks
	}

	slices.SortStableFunc(pathFiles, func(a, b pathFile) int {
		return cmp.Or(
			cmp.Compare(a.DiscNumber(), b.DiscNumber()),
			cmp.Compare(a.TrackNumber(), b.TrackNumber()),
			cmp.Compare(a.path, b.path),
		)
	})

	paths := make([]string, 0, len(pathFiles))
	files := make([]tagcommon.File, 0, len(pathFiles))
	for _, pf := range pathFiles {
		paths = append(paths, pf.path)
		files = append(files, pf.File)
	}

	return cover, paths, files, nil
}

type MusicbrainzClient interface {
	SearchRelease(ctx context.Context, q musicbrainz.ReleaseQuery) (*musicbrainz.Release, error)
	GetCoverURL(ctx context.Context, release *musicbrainz.Release) (string, error)
}

type SearchResult struct {
	Release       *musicbrainz.Release
	Score         float64
	Diff          []tagmap.Diff
	ResearchLinks []researchlink.SearchResult
	OriginFile    *originfile.OriginFile
}

func ProcessDir(
	ctx context.Context, mb MusicbrainzClient, tg tagcommon.Reader,
	pathFormat *pathformat.Format, tagWeights tagmap.TagWeights, researchLinkQuerier *researchlink.Querier, keepFiles map[string]struct{},
	op FileSystemOperation, srcDir string,
	useMBID string, yes bool,
) (*SearchResult, error) {
	srcDir, _ = filepath.Abs(srcDir)

	cover, paths, tagFiles, err := ReadDir(tg, srcDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	searchFile := tagFiles[0]
	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      searchFile.MBReleaseID(),
		MBArtistID:       cmp.Or(searchFile.MBArtistID()...),
		MBReleaseGroupID: searchFile.MBReleaseGroupID(),
		Release:          searchFile.Album(),
		Artist:           cmp.Or(searchFile.AlbumArtist(), searchFile.Artist()),
		Date:             searchFile.Date(),
		Format:           searchFile.MediaFormat(),
		Label:            searchFile.Label(),
		CatalogueNum:     searchFile.CatalogueNum(),
		NumTracks:        len(tagFiles),
	}
	if useMBID != "" {
		query.MBReleaseID = useMBID
	}

	// parse https://github.com/x1ppy/gazelle-origin files, if one exists
	originFile, err := originfile.Find(srcDir)
	if err != nil {
		return nil, fmt.Errorf("find origin file: %w", err)
	}
	if originFile != nil {
		log.Printf("using origin file: %s", originFile)

		if originFile.RecordLabel != "" {
			query.Label = originFile.RecordLabel
		}
		if originFile.CatalogueNumber != "" {
			query.CatalogueNum = originFile.CatalogueNumber
		}
		if originFile.Media != "" {
			media := originFile.Media
			media = strings.ReplaceAll(media, "WEB", "Digital Media")
			query.Format = media
		}
		if originFile.EditionYear > 0 {
			query.Date = time.Date(originFile.EditionYear, 0, 0, 0, 0, 0, 0, time.UTC)
		}
	}

	release, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search musicbrainz: %w", err)
	}

	var researchLinks []researchlink.SearchResult
	if researchLinkQuerier != nil {
		researchLinks, err = researchLinkQuerier.Search(searchFile)
		if err != nil {
			return nil, fmt.Errorf("research querier search: %w", err)
		}
	}

	releaseTracks := musicbrainz.FlatTracks(release.Media)
	if len(tagFiles) != len(releaseTracks) {
		return &SearchResult{Release: release, ResearchLinks: researchLinks, OriginFile: originFile}, fmt.Errorf("%w: %d/%d", ErrTrackCountMismatch, len(tagFiles), len(releaseTracks))
	}

	score, diff := tagmap.DiffRelease(tagWeights, release, tagFiles)
	if !yes && score < minScore {
		return &SearchResult{Release: release, Score: score, Diff: diff, ResearchLinks: researchLinks, OriginFile: originFile}, ErrScoreTooLow
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

	dc := NewDirContext()

	for i := range releaseTracks {
		releaseTrack, path := releaseTracks[i], paths[i]

		pathFormatData := pathformat.Data{Release: *release, Track: releaseTrack, TrackNum: i + 1, Ext: filepath.Ext(path)}
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
		tagFile, err := tg.Read(destPath)
		if err != nil {
			return nil, fmt.Errorf("read tag file: %w", err)
		}
		tagmap.WriteFile(release, labelInfo, genres, &releaseTrack, i, tagFile)
		if err := tagFile.Close(); err != nil {
			return nil, fmt.Errorf("close tag file after write: %w", err)
		}
	}

	if cover == "" {
		cover, err = tryDownloadMusicbrainzCover(ctx, mb, release)
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

	if err := op.TrimDir(dc, destDir); err != nil {
		return nil, fmt.Errorf("trim: %w", err)
	}

	if srcDir != destDir {
		if err := op.CleanDir(dc, pathFormat.Root(), srcDir); err != nil {
			return nil, fmt.Errorf("clean: %w", err)
		}
	}

	return &SearchResult{Release: release, Score: score, Diff: diff, OriginFile: originFile}, nil
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

func tryDownloadMusicbrainzCover(ctx context.Context, mb MusicbrainzClient, release *musicbrainz.Release) (string, error) {
	coverURL, err := mb.GetCoverURL(ctx, release)
	if err != nil {
		return "", fmt.Errorf("request cover url: %w", err)
	}
	if coverURL == "" {
		return "", nil
	}

	// TODO: make sure we put this on the same filesystem as the dest dir?
	tmpf, err := os.CreateTemp("", "wrtag-cover-*"+path.Ext(coverURL))
	if err != nil {
		return "", err
	}
	defer tmpf.Close()

	resp, err := http.Get(coverURL)
	if err != nil {
		return "", fmt.Errorf("create cover destination file: %w", err)
	}
	defer resp.Body.Close()

	n, _ := io.Copy(tmpf, resp.Body)
	log.Printf("wrote cover to tmp (%d bytes)", n)

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
		log.Printf("[dry run] remove all %q", src)
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
