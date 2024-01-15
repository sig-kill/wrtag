package wrtag

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	texttemplate "text/template"
	"time"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/release"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

var (
	ErrNoMatch            = errors.New("no match or score too low")
	ErrTrackCountMismatch = errors.New("track count mismatch")
)

func ReadDir(tg tagcommon.Reader, dir string) ([]tagcommon.File, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return nil, fmt.Errorf("glob dir: %w", err)
	}
	sort.Strings(paths)

	var files []tagcommon.File
	for _, path := range paths {
		if tg.CanRead(path) {
			file, err := tg.Read(path)
			if err != nil {
				return nil, fmt.Errorf("read track: %w", err)
			}
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no tracks in dir")
	}

	return files, nil
}

func SearchReleaseMusicBrainz(ctx context.Context, mb *musicbrainz.Client, releaseTags *release.Release, useMBID string) (*release.Release, error) {
	var query musicbrainz.Query
	query.MBReleaseID = releaseTags.MBID
	query.MBReleaseGroupID = releaseTags.ReleaseGroupMBID
	query.Release = releaseTags.Title
	query.Artist = releaseTags.ArtistCredit
	query.Format = releaseTags.MediaFormat
	query.Date = fmt.Sprint(releaseTags.Date.Year())
	query.Label = releaseTags.Label
	query.CatalogueNum = releaseTags.CatalogueNum
	query.NumTracks = len(releaseTags.Tracks)

	for _, artist := range releaseTags.Artists {
		query.MBArtistID = artist.MBID
		break
	}

	if useMBID != "" {
		query.MBReleaseID = filepath.Base(useMBID)
	}

	score, resp, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search release: %w", err)
	}
	if score < 100 {
		return nil, ErrNoMatch
	}

	releaseMB := release.FromMusicBrainz(resp)

	return releaseMB, nil
}

var PathFormat = template.
	New("template").
	Funcs(template.FuncMap{
		"title": func(ar []release.Artist) []string {
			return mapp(ar, func(_ int, v release.Artist) string { return v.Title })
		},
		"join": func(delim string, items []string) string { return strings.Join(items, delim) },
		"year": func(t time.Time) string { return fmt.Sprintf("%d", t.Year()) },
		"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
	})

type pathFormatData struct {
	R   release.Release
	T   release.Track
	Ext string
}

func DestDir(pathFormat *template.Template, releaseMB release.Release) (string, error) {
	var buff strings.Builder
	if err := pathFormat.Execute(&buff, pathFormatData{R: releaseMB}); err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	path := buff.String()

	return filepath.Dir(path), nil
}

func MoveFiles(pathFormat *template.Template, releaseMB release.Release, paths []string) error {
	for i, t := range releaseMB.Tracks {
		path := paths[i]
		data := pathFormatData{R: releaseMB, T: t, Ext: filepath.Ext(path)}

		var buff strings.Builder
		if err := pathFormat.Execute(&buff, data); err != nil {
			return fmt.Errorf("create path: %w", err)
		}
	}
	return nil
}

type JobConfig struct {
	Path          string
	UseMBID       string
	ConfirmAnyway bool
}

// TODO: split with web Requirements
type Job struct {
	mu sync.Mutex

	ID                   string
	SourcePath, DestPath string

	MBID  string
	Score float64
	Diff  []release.Diff

	Loading bool
	Error   error
}

func ProcessJob(
	ctx context.Context, mb *musicbrainz.Client, tg tagcommon.Reader,
	pathFormat *texttemplate.Template, job *Job, jobC JobConfig,
) (err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.Loading = true
	job.Score = 0
	job.Diff = nil
	job.Error = nil

	defer func() {
		job.Loading = false
		job.Error = err
	}()

	releaseFiles, err := ReadDir(tg, job.SourcePath)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	defer func() {
		var fileErrs []error
		for _, f := range releaseFiles {
			fileErrs = append(fileErrs, f.Close())
		}
		if err != nil {
			return
		}
		err = errors.Join(fileErrs...)
	}()

	var prevMBID = job.MBID
	if jobC.UseMBID != "" {
		prevMBID = jobC.UseMBID
	}

	releaseTags := release.FromTags(releaseFiles)
	releaseMB, err := SearchReleaseMusicBrainz(ctx, mb, releaseTags, prevMBID)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	job.MBID = releaseMB.MBID
	job.Score, job.Diff = release.DiffReleases(releaseTags, releaseMB)

	job.DestPath, err = DestDir(pathFormat, *releaseMB)
	if err != nil {
		return fmt.Errorf("gen dest dir: %w", err)
	}

	if len(releaseTags.Tracks) != len(releaseMB.Tracks) {
		return fmt.Errorf("%w: %d/%d", ErrTrackCountMismatch, len(releaseTags.Tracks), len(releaseMB.Tracks))
	}
	if !jobC.ConfirmAnyway && job.Score < 95 {
		return ErrNoMatch
	}

	release.ToTags(releaseMB, releaseFiles)
	job.Score, job.Diff = release.DiffReleases(releaseMB, releaseMB)
	job.SourcePath = job.DestPath

	return nil
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}
