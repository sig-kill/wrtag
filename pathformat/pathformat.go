package pathformat

import (
	"fmt"
	"strings"
	texttemplate "text/template"
	"time"

	"go.senan.xyz/wrtag/musicbrainz"
)

type Data struct {
	Release  musicbrainz.Release
	Track    musicbrainz.Track
	TrackNum int
	Ext      string
}

func New(pfStr string) (*texttemplate.Template, error) {
	return texttemplate.
		New("template").
		Funcs(texttemplate.FuncMap{
			"title": func(ar []musicbrainz.ArtistCredit) []string {
				return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name })
			},
			"join": func(delim string, items []string) string { return strings.Join(items, delim) },
			"year": func(t time.Time) string { return fmt.Sprintf("%d", t.Year()) },
			"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
		}).
		Parse(pfStr)
}

var funcMap = texttemplate.FuncMap{
	"join": func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },

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
