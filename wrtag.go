package wrtag

import (
	"errors"
	"fmt"
	"io"
	"log"
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

const rmAllSizeThreshold uint64 = 20 * 1e6 // 20 MB

type Operation interface {
	Move(src, dest string) error
	Clean(src string) error
}

type Move struct{}

func (Move) Move(src, dest string) error {
	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func (Move) Clean(src string) error {
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

func (Copy) Move(src, dest string) error {
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
	return err
}

func (Copy) Clean(src string) error {
	return nil
}

type DryRun struct{}

func (DryRun) Move(src, dest string) error {
	log.Printf("[dry run] %q -> %q", src, dest)
	return nil
}

func (DryRun) Clean(src string) error {
	log.Printf("[dry run] remove all %q", src)
	return nil
}

var _ Operation = (*Move)(nil)
var _ Operation = (*Copy)(nil)
var _ Operation = (*DryRun)(nil)

func ReadDir(tg tagcommon.Reader, path string) (string, []string, []tagcommon.File, error) {
	allPaths, err := filepath.Glob(filepath.Join(path, "*"))
	if err != nil {
		return "", nil, nil, fmt.Errorf("glob dir: %w", err)
	}
	sort.Strings(allPaths)

	var cover string
	var paths []string
	var files []tagcommon.File
	for _, path := range allPaths {
		switch filepath.Ext(path) {
		case ".jpg", ".jpeg", ".png", "bmp", "gif":
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
		}
	}
	if len(files) == 0 {
		return "", nil, nil, fmt.Errorf("no tracks in dir")
	}

	return cover, paths, files, nil
}

func DestDir(pathFormat *texttemplate.Template, release *musicbrainz.Release) (string, error) {
	var buff strings.Builder
	if err := pathFormat.Execute(&buff, pathformat.Data{Release: *release}); err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	path := buff.String()
	dir := filepath.Dir(path)
	return dir, nil
}

func MoveFiles(
	pathFormat *texttemplate.Template, release *musicbrainz.Release,
	op Operation, srcDir string, paths []string, cover string,
) error {
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
		if err := op.Move(path, destPath); err != nil {
			return fmt.Errorf("op to dest %q: %w", destPath, err)
		}
	}

	destDir, err := DestDir(pathFormat, release)
	if err != nil {
		return fmt.Errorf("gen dest dir: %w", err)
	}

	if cover != "" {
		coverDest := filepath.Join(destDir, "cover"+filepath.Ext(cover))
		if err := op.Move(cover, coverDest); err != nil {
			return fmt.Errorf("move file to dest: %w", err)
		}
	}

	if filepath.Clean(srcDir) != filepath.Clean(destDir) {
		if err := op.Clean(srcDir); err != nil {
			return fmt.Errorf("clean src dir: %w", err)
		}
	}

	return nil
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
