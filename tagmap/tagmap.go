package tagmap

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

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

type TagWeights map[string]float64

func (tw TagWeights) For(field string) float64 {
	if field == "" {
		return 1
	}
	for f, w := range tw {
		if strings.HasPrefix(field, f) {
			return w
		}
	}
	return 1
}

func DiffRelease(weights TagWeights, release *musicbrainz.Release, files []tagcommon.File) (float64, []Diff) {
	if len(files) == 0 {
		return 0, nil
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)
	first := files[0]

	var score float64
	diff := differ(weights, &score)

	var diffs []Diff
	diffs = append(diffs,
		diff("release", first.Album(), release.Title),
		diff("artist", first.AlbumArtist(), musicbrainz.ArtistsString(release.Artists)),
		diff("label", first.Label(), labelInfo.Label.Name),
		diff("catalogue num", first.CatalogueNum(), labelInfo.CatalogNumber),
		diff("media format", first.MediaFormat(), release.Media[0].Format),
	)

	rtracks := musicbrainz.FlatTracks(release.Media)
	if len(rtracks) != len(files) {
		panic(fmt.Errorf("len(rtracks) != len(files)"))
	}

	for i, f := range files {
		diffs = append(diffs, diff(
			fmt.Sprintf("track %d", i+1),
			strings.Join(filter(f.Artist(), f.Title()), " – "),
			strings.Join(filter(musicbrainz.ArtistsString(rtracks[i].Artists), rtracks[i].Title), " – "),
		))
	}

	return score, diffs
}

func WriteFile(
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []musicbrainz.Genre,
	releaseTrack *musicbrainz.Track, i int, f tagcommon.File,
) {
	if ru, ok := f.(interface{ RemoveUnknown() }); ok {
		ru.RemoveUnknown()
	}

	var genreNames []string
	for _, g := range genres[:min(6, len(genres))] { // top 6 genre strings
		genreNames = append(genreNames, g.Name)
	}
	var anyGenre string
	if len(genreNames) > 0 {
		anyGenre = genreNames[0]
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
	f.WriteGenres(genreNames)
	f.WriteTrackNumber(i + 1)
	f.WriteDiscNumber(1)

	f.WriteMBRecordingID(releaseTrack.Recording.ID)
	f.WriteMBArtistID(mapp(releaseTrack.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID }))
}

func differ(weights TagWeights, score *float64) func(field string, a, b string) Diff {
	var total float64
	var diff float64

	return func(field, a, b string) Diff {
		diffs := dmp.DiffMain(norm(a), norm(b), false)
		dist := float64(dmp.DiffLevenshtein(diffs))
		distWeighted := dist * weights.For(field)

		diff += distWeighted
		total += float64(len([]rune(b)))

		*score = 100 - (diff * 100 / total)

		diffsPresented := dmp.DiffMain(a, b, false)
		distPresented := float64(dmp.DiffLevenshtein(diffsPresented))
		return Diff{
			Field:  field,
			Before: filterDiff(diffsPresented, func(d diffmatchpatch.Diff) bool { return d.Type <= diffmatchpatch.DiffEqual }),
			After:  filterDiff(diffsPresented, func(d diffmatchpatch.Diff) bool { return d.Type >= diffmatchpatch.DiffEqual }),
			Equal:  distPresented == 0,
		}
	}
}

func norm(input string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
		if unicode.IsNumber(r) {
			return r
		}
		return -1
	}, input)
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
