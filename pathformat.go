package wrtag

import (
	texttemplate "text/template"

	"go.senan.xyz/wrtag/musicbrainz"
)

type PathFormatData struct {
	Release  musicbrainz.Release
	Track    musicbrainz.Track
	TrackNum int
	Ext      string
}

func PathFormatTemplate(pathFormat string) (*texttemplate.Template, error) {
	return texttemplate.
		New("template").
		Funcs(TemplateFuncMap).
		Parse(pathFormat)
}
