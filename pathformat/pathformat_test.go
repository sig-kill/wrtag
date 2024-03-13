package pathformat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/pathformat"
)

func TestPathFormat(t *testing.T) {
	var pf pathformat.Format
	_, err := pf.Execute(pathformat.Data{})
	assert.Error(t, err) // we didn't initalise with Parse() yet

	// bad/ambiguous format
	assert.ErrorIs(t, pf.Parse(""), pathformat.ErrInvalidFormat)
	assert.ErrorIs(t, pf.Parse(" "), pathformat.ErrInvalidFormat)
	assert.ErrorIs(t, pf.Parse("🤤"), pathformat.ErrInvalidFormat)

	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artistCredit .Release.Artists }}/{{ .Release.Title }}`), pathformat.ErrAmbiguousFormat)
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ .Track.Title }}`), pathformat.ErrAmbiguousFormat)
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ .TrackNum }}`), pathformat.ErrAmbiguousFormat)

	// bad data
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artistCredit .Release.Artists }}/{{ .Release.ID }}/`), pathformat.ErrBadData)                   // test case is missing ID
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artistCredit .Release.Artists }}//`), pathformat.ErrBadData)                                    // double slash anyway
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ artistCredit .Release.Artists }}/{{ .Release.Title }}/{{ .Track.ID }}`), pathformat.ErrBadData) // implicit trailing slash from missing ID
	assert.ErrorIs(t, pf.Parse(`/albums/test/{{ .Track.ID }}/`), pathformat.ErrBadData)                                                         //

	// good
	assert.NoError(t, pf.Parse(`/albums/{{ artistCredit .Release.Artists }}/{{ .Release.Title }}/{{ .TrackNum }}`))
}
