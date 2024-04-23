package lyrics_test

import (
	"embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/lyrics"
)

//go:embed testdata
var responses embed.FS

func TestMusixmatch(t *testing.T) {
	var mm lyrics.Musixmatch
	mm.HTTPClient = clientutil.FSClient(responses, "testdata/musixmatch")

	resp, err := mm.Search("The Fall", "Wings")
	require.NoError(t, err)
	assert.True(t, strings.Contains(resp, `I paid them off with stuffing from my wings.`))
	assert.True(t, strings.Contains(resp, `They had some fun with those cheapo airline snobs.`))
	assert.True(t, strings.Contains(resp, `The stuffing loss made me hit a timelock.`))

	resp, err = mm.Search("The Fall", "Uhh yeah - uh greath")
	require.ErrorIs(t, err, lyrics.ErrLyricsNotFound)
	assert.Empty(t, resp)
}
