package wrtag

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

var (
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

func DestDir(pathFormat *texttemplate.Template, release musicbrainz.Release) (string, error) {
	var buff strings.Builder
	if err := pathFormat.Execute(&buff, PathFormatData{Release: release}); err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	path := buff.String()
	dir := filepath.Dir(path)
	return dir, nil
}

func MoveFiles(pathFormat *texttemplate.Template, release *musicbrainz.Release, paths []string) error {
	releaseTracks := musicbrainz.FlatTracks(release.Media)
	if len(releaseTracks) != len(paths) {
		return fmt.Errorf("%d/%d: %w", len(releaseTracks), len(paths), ErrTrackCountMismatch)
	}

	for i := range releaseTracks {
		releaseTrack, path := releaseTracks[i], paths[i]
		data := PathFormatData{Release: *release, Track: releaseTrack, TrackNum: i + 1, Ext: filepath.Ext(path)}

		var buff strings.Builder
		if err := pathFormat.Execute(&buff, data); err != nil {
			return fmt.Errorf("create path: %w", err)
		}
	}
	return nil
}

type SearchLinkTemplate struct {
	Name  string
	Templ *texttemplate.Template
}

type JobSearchLink struct {
	Name, URL string
}

var TemplateFuncMap = texttemplate.FuncMap{
	"now":   func() int64 { return time.Now().UnixMilli() },
	"file":  func(p string) string { ur, _ := url.Parse("file://"); ur.Path = p; return ur.String() },
	"url":   func(u string) htmltemplate.URL { return htmltemplate.URL(u) },
	"query": htmltemplate.URLQueryEscaper,
	"join":  func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0":  func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
	"or":    or[string],

	"flatTracks":   musicbrainz.FlatTracks,
	"artistCredit": musicbrainz.CreditString,
	"artistNames": func(ar []musicbrainz.ArtistCredit) []string {
		return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name })
	},
	"artistMBIDs": func(ar []musicbrainz.ArtistCredit) []string {
		return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })
	},
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

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}
