package wrtag

import (
	"context"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"log"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	texttemplate "text/template"
	"time"

	"go.senan.xyz/wrtag/diff"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tagmap"
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

func DestDir(pathFormat *texttemplate.Template, releaseMB musicbrainz.Release) (string, error) {
	var buff strings.Builder
	if err := pathFormat.Execute(&buff, releasePathFormatData{R: releaseMB}); err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	path := buff.String()

	return filepath.Dir(path), nil
}

func MoveFiles(pathFormat *texttemplate.Template, releaseMB *musicbrainz.Release, paths []string) error {
	for i, t := range musicbrainz.FlatTracks(releaseMB.Media) {
		path := paths[i]
		data := releasePathFormatData{R: *releaseMB, T: t, Ext: filepath.Ext(path)}

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
type JobSearchLink struct {
	Name, URL string
}
type Job struct {
	mu sync.Mutex

	ID                   string
	SourcePath, DestPath string

	MBID        string
	Score       float64
	Diff        []diff.Diff
	SearchLinks []JobSearchLink

	Loading bool
	Error   error
}

func ProcessJob(
	ctx context.Context, mb *musicbrainz.Client, tg tagcommon.Reader,
	pathFormat *texttemplate.Template, searchLinksTemplates []SearchLinkTemplate,
	job *Job, jobC JobConfig,
) (err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.Loading = true
	job.Score = 0
	job.Diff = nil
	job.SearchLinks = nil
	job.Error = nil

	defer func() {
		job.Loading = false
		job.Error = err
	}()

	tagFiles, err := ReadDir(tg, job.SourcePath)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	defer func() {
		var fileErrs []error
		for _, f := range tagFiles {
			fileErrs = append(fileErrs, f.Close())
		}
		if err != nil {
			return
		}
		err = errors.Join(fileErrs...)
	}()

	searchFile := tagFiles[0]
	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      searchFile.MBReleaseID(),
		MBArtistID:       first(searchFile.MBArtistID()),
		MBReleaseGroupID: searchFile.MBReleaseGroupID(),
		Release:          searchFile.Album(),
		Artist:           or(searchFile.AlbumArtist(), searchFile.AlbumArtist()),
		Date:             searchFile.Date(),
		Format:           searchFile.MediaFormat(),
		Label:            searchFile.Label(),
		CatalogueNum:     searchFile.CatalogueNum(),
		NumTracks:        len(tagFiles),
	}
	if jobC.UseMBID != "" {
		query.MBReleaseID = jobC.UseMBID
	}

	release, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	job.MBID = release.ID
	job.Score, job.Diff = tagmap.DiffRelease(tagFiles, release)

	job.DestPath, err = DestDir(pathFormat, *release)
	if err != nil {
		return fmt.Errorf("gen dest dir: %w", err)
	}

	for _, v := range searchLinksTemplates {
		var buff strings.Builder
		if err := v.Templ.Execute(&buff, searchFile); err != nil {
			log.Printf("error parsing search link template: %v", err)
			continue
		}
		job.SearchLinks = append(job.SearchLinks, JobSearchLink{Name: v.Name, URL: buff.String()})
	}

	if releaseTracks := musicbrainz.FlatTracks(release.Media); len(tagFiles) != len(releaseTracks) {
		return fmt.Errorf("%w: %d/%d", ErrTrackCountMismatch, len(tagFiles), len(releaseTracks))
	}
	if !jobC.ConfirmAnyway && job.Score < 95 {
		return ErrNoMatch
	}

	// write release to tags. saved by defered Close()
	tagmap.WriteRelease(release, tagFiles)

	job.Score, job.Diff = diff.DiffReleases(tagFiles, release)
	job.SourcePath = job.DestPath

	if err := MoveFiles(pathFormat, release, nil); err != nil {
		return fmt.Errorf("move files: %w", err)
	}

	return nil
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}

var TemplateFuncMap = texttemplate.FuncMap{
	"now":     func() int64 { return time.Now().UnixMilli() },
	"file":    func(p string) string { ur, _ := url.Parse("file://"); ur.Path = p; return ur.String() },
	"url":     func(u string) htmltemplate.URL { return htmltemplate.URL(u) },
	"query":   htmltemplate.URLQueryEscaper,
	"nomatch": func(err error) bool { return errors.Is(err, ErrNoMatch) },
	"title": func(ar []musicbrainz.ArtistCredit) []string {
		return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name })
	},
	"join": func(delim string, items []string) string { return strings.Join(items, delim) },
	"year": func(t time.Time) string { return fmt.Sprintf("%d", t.Year()) },
	"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
}

var PathFormat = texttemplate.
	New("template").
	Funcs(TemplateFuncMap)

type releasePathFormatData struct {
	R   musicbrainz.Release
	T   musicbrainz.Track
	Ext string
}

type SearchLinkTemplates []SearchLinkTemplate
type SearchLinkTemplate struct {
	Name  string
	Templ *texttemplate.Template
}

func (sls SearchLinkTemplates) String() string {
	var names []string
	for _, sl := range sls {
		names = append(names, sl.Name)
	}
	return strings.Join(names, ", ")
}

func (sls *SearchLinkTemplates) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	templ, err := texttemplate.New("").Funcs(TemplateFuncMap).Parse(value)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	*sls = append(*sls, SearchLinkTemplate{Name: name, Templ: templ})
	return nil
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
