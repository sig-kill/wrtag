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

type Diff struct {
	Field         string
	Before, After string
	Changes       []diffmatchpatch.Diff
}

func DiffRelease(release *musicbrainz.Release, files []tagcommon.File) (float64, []Diff) {
	dmp := diffmatchpatch.New()

	var charsTotal int
	var charsDiff int
	add := func(f, a, b string) Diff {
		diffs := dmp.DiffMain(a, b, false)
		charsTotal += len([]rune(b))
		charsDiff += dmp.DiffLevenshtein(diffs)
		return Diff{Field: f, Changes: diffs, Before: a, After: b}
	}

	if len(files) == 0 {
		return 0, nil
	}
	fone := files[0]

	var diffs []Diff
	diffs = append(diffs,
		add("release", fone.Album(), release.Title),
		add("artist", fone.AlbumArtist(), musicbrainz.CreditString(release.Artists)),
		add("label", fone.Label(), first(release.LabelInfo).Label.Name),
		add("catalogue num", fone.CatalogueNum(), first(release.LabelInfo).CatalogNumber),
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
			strings.Join(filter(musicbrainz.CreditString(rtracks[i].Artists), rtracks[i].Title), " – "),
		))
	}

	score := 100 - (float64(charsDiff) * 100 / float64(charsTotal))

	return score, diffs
}

func WriteRelease(release *musicbrainz.Release, files []tagcommon.File) {
	releaseTracks := musicbrainz.FlatTracks(release.Media)
	if len(releaseTracks) != len(files) {
		panic("provoded tracks and release track count mismatch")
	}

	for i, f := range files {
		f.WriteAlbum(release.Title)
		f.WriteAlbumArtist(musicbrainz.CreditString(release.Artists))
		f.WriteAlbumArtists(mapp(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name }))
		f.WriteDate(release.Date.Format(time.DateOnly))
		f.WriteOriginalDate(release.ReleaseGroup.FirstReleaseDate.Format(time.DateOnly))
		f.WriteMediaFormat(release.Media[0].Format)
		f.WriteLabel(first(release.LabelInfo).Label.Name)
		f.WriteCatalogueNum(first(release.LabelInfo).CatalogNumber)

		f.WriteMBReleaseID(release.ID)
		f.WriteMBReleaseGroupID("")
		f.WriteMBAlbumArtistID(mapp(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID }))

		f.WriteTitle(releaseTracks[i].Title)
		f.WriteArtist(musicbrainz.CreditString(releaseTracks[i].Artists))
		f.WriteArtists(mapp(releaseTracks[i].Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name }))
		f.WriteGenre("")
		f.WriteGenres(nil)
		f.WriteTrackNumber(i)
		f.WriteDiscNumber(1)

		f.WriteMBRecordingID(releaseTracks[i].Recording.ID)
		f.WriteMBArtistID(mapp(releaseTracks[i].Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID }))
	}
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
