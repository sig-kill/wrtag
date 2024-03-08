package wrtag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
)

const rmAllSizeThreshold uint64 = 20 * 1e6 // 20 MB

type FileSystemOperation interface {
	ProcessFile(src, dest string) error
	CleanDir(src string) error
}

type Move struct{}

func (Move) ProcessFile(src, dest string) error {
	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func (Move) CleanDir(src string) error {
	dirSize, err := dirSize(src)
	if err != nil {
		return fmt.Errorf("get dir size for sanity check: %w", err)
	}
	if dirSize > rmAllSizeThreshold {
		return fmt.Errorf("folder was too big for clean up: %d/%d", dirSize, rmAllSizeThreshold)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("error cleaning up folder: %w", err)
	}
	return nil
}

type Copy struct{}

func (Copy) ProcessFile(src, dest string) error {
	if filepath.Clean(src) == filepath.Clean(dest) {
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

func (Copy) CleanDir(src string) error {
	return nil
}

type DryRun struct{}

func (DryRun) ProcessFile(src, dest string) error {
	log.Printf("[dry run] %q -> %q", src, dest)
	return nil
}

func (DryRun) CleanDir(src string) error {
	log.Printf("[dry run] remove all %q", src)
	return nil
}

var _ FileSystemOperation = (*Move)(nil)
var _ FileSystemOperation = (*Copy)(nil)
var _ FileSystemOperation = (*DryRun)(nil)

func ReadDir(tg tagcommon.Reader, path string) (string, []string, []tagcommon.File, error) {
	allPaths, err := fileutil.GlobBase(path, "*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("glob dir: %w", err)
	}
	sort.Strings(allPaths)

	var cover string
	var paths []string
	var files []tagcommon.File
	for _, path := range allPaths {
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
			paths = append(paths, path)
			files = append(files, file)
			_ = file.Close()
		}
	}
	if len(files) == 0 {
		return "", nil, nil, ErrNoTracks
	}

	return cover, paths, files, nil
}

type MusicbrainzClient interface {
	SearchRelease(ctx context.Context, q musicbrainz.ReleaseQuery) (*musicbrainz.Release, error)
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
	pathFormat *pathformat.Format, researchLinkQuerier *researchlink.Querier,
	op FileSystemOperation, srcDir string,
	useMBID string, yes bool,
) (*SearchResult, error) {
	cover, paths, tagFiles, err := ReadDir(tg, srcDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	searchFile := tagFiles[0]
	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      searchFile.MBReleaseID(),
		MBArtistID:       first(searchFile.MBArtistID()),
		MBReleaseGroupID: searchFile.MBReleaseGroupID(),
		Release:          searchFile.Album(),
		Artist:           or(searchFile.AlbumArtist(), searchFile.Artist()),
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

	researchLinks, err := researchLinkQuerier.Search(searchFile)
	if err != nil {
		return nil, fmt.Errorf("research querier search: %w", err)
	}

	releaseTracks := musicbrainz.FlatTracks(release.Media)
	if len(tagFiles) != len(releaseTracks) {
		return &SearchResult{Release: release, ResearchLinks: researchLinks, OriginFile: originFile}, fmt.Errorf("%w: %d/%d", ErrTrackCountMismatch, len(tagFiles), len(releaseTracks))
	}

	score, diff := tagmap.DiffRelease(release, tagFiles)
	if !yes && score < 95 {
		return &SearchResult{Release: release, Score: score, Diff: diff, ResearchLinks: researchLinks, OriginFile: originFile}, ErrScoreTooLow
	}

	destDir, err := DestDir(pathFormat, release)
	if err != nil {
		return nil, fmt.Errorf("gen dest dir: %w", err)
	}

	// lock both source and destination directories
	unlock := nLock(
		dirLocks.Get(srcDir),
		dirLocks.Get(destDir),
	)
	defer unlock()

	labelInfo := musicbrainz.AnyLabelInfo(release)
	genres := musicbrainz.AnyGenres(release)

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
		if err := op.ProcessFile(path, destPath); err != nil {
			return nil, fmt.Errorf("op to dest %q: %w", destPath, err)
		}

		if _, ok := op.(DryRun); ok {
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

	if cover != "" {
		// use local cover
		coverDest := filepath.Join(destDir, "cover"+filepath.Ext(cover))
		if err := op.ProcessFile(cover, coverDest); err != nil {
			return nil, fmt.Errorf("move file to dest: %w", err)
		}
	} else {
		// get mb cover
		coverURL, err := musicbrainz.GetCoverURL(ctx, release)
		if err != nil {
			return nil, fmt.Errorf("request cover url: %w", err)
		}
		if coverURL != "" {
			coverDest := filepath.Join(destDir, "cover"+filepath.Ext(coverURL))
			coverf, err := os.Create(coverDest)
			if err != nil {
				return nil, fmt.Errorf("create cover destination file: %w", err)
			}
			resp, err := http.Get(coverURL)
			if err != nil {
				return nil, fmt.Errorf("create cover destination file: %w", err)
			}
			n, _ := io.Copy(coverf, resp.Body)
			resp.Body.Close()
			coverf.Close()

			log.Printf("wrote cover to %s (%d bytes)", coverDest, n)
		}
	}

	if filepath.Clean(srcDir) != filepath.Clean(destDir) {
		if err := op.CleanDir(srcDir); err != nil {
			return nil, fmt.Errorf("clean src dir: %w", err)
		}
	}

	return &SearchResult{Release: release, Score: score, Diff: diff, OriginFile: originFile}, nil
}

func DestDir(pathFormat *pathformat.Format, release *musicbrainz.Release) (string, error) {
	path, err := pathFormat.Execute(pathformat.Data{Release: *release})
	if err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	dir := filepath.Dir(path)
	return dir, nil
}

func dirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		return err
	})
	return size, err
}

func first[T comparable](is []T) T {
	var z T
	for _, i := range is {
		if i != z {
			return i
		}
	}
	return z
}

func or[T comparable](items ...T) T {
	var zero T
	for _, i := range items {
		if i != zero {
			return i
		}
	}
	return zero
}

var dirLocks keyedMutex

type keyedMutex struct {
	sync.Map
}

func (km *keyedMutex) Get(key string) *sync.Mutex {
	value, _ := km.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	return mu
}

func nLock(locks ...sync.Locker) func() {
	for _, l := range locks {
		l.Lock()
	}
	return func() {
		for _, l := range locks {
			l.Unlock()
		}
	}
}
