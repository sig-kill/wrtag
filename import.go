package wrtag

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/release"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

var ErrNoMatch = errors.New("no match or score too low")

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

func SearchReleaseMusicBrainz(ctx context.Context, mb *musicbrainz.Client, releaseTags *release.Release) (*release.Release, error) {
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

	score, resp, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search release: %w", err)
	}
	if score < 100 {
		return nil, ErrNoMatch
	}

	releaseMB := release.FromMusicBrainz(resp)
	if len(releaseTags.Tracks) != len(releaseMB.Tracks) {
		return nil, fmt.Errorf("%w: track count mismatch %d/%d", ErrNoMatch, len(releaseTags.Tracks), len(releaseMB.Tracks))
	}

	return releaseMB, nil
}

// func MoveFiles(pathFormat *template.Template, releaseMB *release.Release, paths []string) error {
// 	for i, t := range releaseMB.Tracks {
// 		path := paths[i]
// 		pathFormatData := struct {
// 			R   *release.Release
// 			T   *release.Track
// 			Ext string
// 		}{
// 			R:   releaseMB,
// 			T:   &t,
// 			Ext: filepath.Ext(path),
// 		}
//
// 		var newPathBuilder strings.Builder
// 		if err := pathFormat.Execute(&newPathBuilder, pathFormatData); err != nil {
// 			return fmt.Errorf("create path: %w", err)
// 		}
// 	}
// 	return nil
// }
