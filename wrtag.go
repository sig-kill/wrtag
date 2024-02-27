package wrtag

import (
	"errors"
	"fmt"
	"os"
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

func ReadDir(tg tagcommon.Reader, path string) ([]string, []tagcommon.File, error) {
	allPaths, err := filepath.Glob(filepath.Join(path, "*"))
	if err != nil {
		return nil, nil, fmt.Errorf("glob dir: %w", err)
	}
	sort.Strings(allPaths)

	var paths []string
	var files []tagcommon.File
	for _, path := range allPaths {
		if tg.CanRead(path) {
			file, err := tg.Read(path)
			if err != nil {
				return nil, nil, fmt.Errorf("read track: %w", err)
			}
			paths = append(paths, path)
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no tracks in dir")
	}

	return paths, files, nil
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

func MoveFiles(pathFormat *texttemplate.Template, release *musicbrainz.Release, dir string, paths []string) error {
	releaseTracks := musicbrainz.FlatTracks(release.Media)
	if len(releaseTracks) != len(paths) {
		return fmt.Errorf("%d/%d: %w", len(releaseTracks), len(paths), ErrTrackCountMismatch)
	}

	for i := range releaseTracks {
		releaseTrack, path := releaseTracks[i], paths[i]
		data := pathformat.Data{Release: *release, Track: releaseTrack, TrackNum: i + 1, Ext: filepath.Ext(path)}

		var destPathBuff strings.Builder
		if err := pathFormat.Execute(&destPathBuff, data); err != nil {
			return fmt.Errorf("create path: %w", err)
		}
		destPath := destPathBuff.String()

		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return fmt.Errorf("create dest path: %w", err)
		}
		if err := os.Rename(path, destPath); err != nil {
			return fmt.Errorf("move file to dest: %w", err)
		}
	}
	return nil
}
