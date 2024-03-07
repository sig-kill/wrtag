package pathformat

import (
	"fmt"
	"sort"
	"strings"
	texttemplate "text/template"

	"go.senan.xyz/wrtag/musicbrainz"
)

type Data struct {
	Release  musicbrainz.Release
	Track    musicbrainz.Track
	TrackNum int
	Ext      string
}

type Format struct{ texttemplate.Template }

func (pf *Format) Parse(str string) error {
	if str == "" {
		return fmt.Errorf("empty format")
	}
	tmpl, err := texttemplate.
		New("template").
		Funcs(funcMap).
		Parse(str)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	*pf = Format{*tmpl}
	return err
}

var funcMap = texttemplate.FuncMap{
	"join": func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
	"sort": func(strings []string) []string { sort.Strings(strings); return strings },

	"flatTracks":   musicbrainz.FlatTracks,
	"artistCredit": musicbrainz.CreditString,
	"artistNames": func(ar []musicbrainz.ArtistCredit) []string {
		return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name })
	},
	"artistMBIDs": func(ar []musicbrainz.ArtistCredit) []string {
		return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })
	},
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}
