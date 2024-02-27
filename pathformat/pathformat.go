package pathformat

import (
	"fmt"
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

func New(pfStr string) (*texttemplate.Template, error) {
	if pfStr == "" {
		return nil, fmt.Errorf("empty format")
	}
	return texttemplate.
		New("template").
		Funcs(funcMap).
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
