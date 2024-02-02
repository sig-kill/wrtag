package wrtag

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
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
	if err := pathFormat.Execute(&buff, pathformat.Data{Release: release}); err != nil {
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
		data := pathformat.Data{Release: *release, Track: releaseTrack, TrackNum: i + 1, Ext: filepath.Ext(path)}

		var buff strings.Builder
		if err := pathFormat.Execute(&buff, data); err != nil {
			return fmt.Errorf("create path: %w", err)
		}
	}
	return nil
}
