package tagmap

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

var dmp = diffmatchpatch.New()

type Diff struct {
	Field         string
	Before, After []diffmatchpatch.Diff
	Equal         bool
}

func DiffRelease(release *musicbrainz.Release, files []tagcommon.File) (float64, []Diff) {
	var charsTotal int
	var charsDiff int
	add := func(f, a, b string) Diff {
		diffs := dmp.DiffMain(a, b, false)
		charsTotal += len([]rune(b))

		charDiff := dmp.DiffLevenshtein(diffs)
		charsDiff += charDiff

		return Diff{
			Field:  f,
			Before: filterDiff(diffs, func(d diffmatchpatch.Diff) bool { return d.Type <= diffmatchpatch.DiffEqual }),
			After:  filterDiff(diffs, func(d diffmatchpatch.Diff) bool { return d.Type >= diffmatchpatch.DiffEqual }),
			Equal:  charDiff == 0,
		}
	}

	if len(files) == 0 {
		return 0, nil
	}
	fone := files[0]

	labelInfo := musicbrainz.AnyLabelInfo(release)

	var diffs []Diff
	diffs = append(diffs,
		add("release", fone.Album(), release.Title),
		add("artist", fone.AlbumArtist(), musicbrainz.ArtistsString(release.Artists)),
		add("label", fone.Label(), labelInfo.Label.Name),
		add("catalogue num", fone.CatalogueNum(), labelInfo.CatalogNumber),
		add("media format", fone.MediaFormat(), release.Media[0].Format),
	)

	rtracks := musicbrainz.FlatTracks(release.Media)
	for i, f := range files {
		if i > len(rtracks)-1 {
			diffs = append(diffs, add(
				fmt.Sprintf("track %d", i+1),
				strings.Join(filter(f.Artist(), f.Title()), " – "),
				"",
			))
			continue
		}
		diffs = append(diffs, add(
			fmt.Sprintf("track %d", i+1),
			strings.Join(filter(f.Artist(), f.Title()), " – "),
			strings.Join(filter(musicbrainz.ArtistsString(rtracks[i].Artists), rtracks[i].Title), " – "),
		))
	}

	score := 100 - (float64(charsDiff) * 100 / float64(charsTotal))

	return score, diffs
}

func WriteFile(
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []string,
	releaseTrack *musicbrainz.Track, i int, f tagcommon.File,
) {
	if ru, ok := f.(interface{ RemoveUnknown() }); ok {
		ru.RemoveUnknown()
	}

	var anyGenre string
	if len(genres) > 0 {
		anyGenre = genres[0]
	}

	f.WriteAlbum(release.Title)
	f.WriteAlbumArtist(musicbrainz.ArtistsString(release.Artists))
	f.WriteAlbumArtists(musicbrainz.ArtistsNames(release.Artists))
	f.WriteAlbumArtistCredit(musicbrainz.ArtistsCreditString(release.Artists))
	f.WriteAlbumArtistsCredit(musicbrainz.ArtistsCreditNames(release.Artists))
	f.WriteDate(release.Date.Format(time.DateOnly))
	f.WriteOriginalDate(release.ReleaseGroup.FirstReleaseDate.Format(time.DateOnly))
	f.WriteMediaFormat(release.Media[0].Format)
	f.WriteLabel(labelInfo.Label.Name)
	f.WriteCatalogueNum(labelInfo.CatalogNumber)

	f.WriteMBReleaseID(release.ID)
	f.WriteMBReleaseGroupID(release.ReleaseGroup.ID)
	f.WriteMBAlbumArtistID(mapp(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID }))

	f.WriteTitle(releaseTrack.Title)
	f.WriteArtist(musicbrainz.ArtistsString(releaseTrack.Artists))
	f.WriteArtists(musicbrainz.ArtistsNames(releaseTrack.Artists))
	f.WriteArtistCredit(musicbrainz.ArtistsCreditString(releaseTrack.Artists))
	f.WriteArtistsCredit(musicbrainz.ArtistsCreditNames(releaseTrack.Artists))
	f.WriteGenre(anyGenre)
	f.WriteGenres(genres)
	f.WriteTrackNumber(i + 1)
	f.WriteDiscNumber(1)

	f.WriteMBRecordingID(releaseTrack.Recording.ID)
	f.WriteMBArtistID(mapp(releaseTrack.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID }))
}

func filter[T comparable](elms ...T) []T {
	var zero T
	return slices.DeleteFunc(elms, func(t T) bool {
		return t == zero
	})
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}

func filterDiff(diffs []diffmatchpatch.Diff, f func(diffmatchpatch.Diff) bool) []diffmatchpatch.Diff {
	var r []diffmatchpatch.Diff
	for _, diff := range diffs {
		if f(diff) {
			r = append(r, diff)
		}
	}
	return r
}
