package diff

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

type Diff struct {
	Field         string
	Before, After string
	Changes       []diffmatchpatch.Diff
}

func DiffReleases(files []tagcommon.File, release *musicbrainz.Release) (float64, []Diff) {
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
