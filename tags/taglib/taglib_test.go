package taglib_test

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/tags/taglib"
)

//go:embed testdata/empty.flac
var empty []byte

func TestTrackNum(t *testing.T) {
	t.Parallel()

	tmpf, err := os.CreateTemp("", "*.flac")
	require.NoError(t, err)
	defer os.Remove(tmpf.Name())

	_, err = io.Copy(tmpf, bytes.NewReader(empty))
	require.NoError(t, err)

	{
		var tg taglib.TagLib
		f, err := tg.Read(tmpf.Name())
		require.NoError(t, err)

		f.WriteTrackNumber(69)
		require.NoError(t, f.Close())
	}

	{
		var tg taglib.TagLib
		f, err := tg.Read(tmpf.Name())
		require.NoError(t, err)

		f.WriteTrackNumber(69)
		require.NoError(t, f.Close())
	}

	{
		var tg taglib.TagLib
		f, err := tg.Read(tmpf.Name())
		require.NoError(t, err)

		require.Equal(t, 69, f.TrackNumber())
		require.NoError(t, f.Close())
	}
}

//go:embed testdata/koloss.flac
var koloss []byte

func TestNoWrite(t *testing.T) {
	t.Parallel()

	tmpf, err := os.CreateTemp("", "*.flac")
	require.NoError(t, err)
	defer os.Remove(tmpf.Name())

	info, err := tmpf.Stat()
	require.NoError(t, err)
	modTime := info.ModTime()

	_, err = io.Copy(tmpf, bytes.NewReader(koloss))
	require.NoError(t, err)

	for range 2 {
		var tg taglib.TagLib
		f, err := tg.Read(tmpf.Name())
		require.NoError(t, err)

		ru, ok := f.(interface{ RemoveUnknown() })
		require.True(t, ok)
		ru.RemoveUnknown()

		// should all be no-ops
		f.WriteTitle("Koloss")
		f.WriteAlbum("Stellar Transmissions")
		f.WriteAlbumArtists([]string{"Various Artists"})
		f.WriteGenres([]string{"psychedelic", "psytrance", "techno"})
		f.WriteTrackNumber(1)
		f.WriteGenre("psychedelic")

		// make sure we have enough precision in fs timestamp
		time.Sleep(200 * time.Millisecond)

		require.NoError(t, f.Close())

		info, err = tmpf.Stat()
		require.NoError(t, err)
		modTimeAfter := info.ModTime()

		require.Equal(t, modTime, modTimeAfter) // we didn't write
	}
}
