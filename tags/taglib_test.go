package tags

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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

	withf := func(fn func(*File)) {
		t.Helper()

		f, err := Read(tmpf.Name())
		require.NoError(t, err)
		fn(f)
		require.NoError(t, f.Save())
		f.Close()
	}

	withf(func(f *File) {
		f.WriteNum(TrackNumber, 69)
	})
	withf(func(f *File) {
		f.WriteNum(TrackNumber, 69)
	})
	withf(func(f *File) {
		require.Equal(t, 69, f.ReadNum(TrackNumber))
	})
}

func TestNormalise(t *testing.T) {
	raw := map[string][]string{
		"Some":                {"a", "b"},
		"other Test":          {"x"},
		"media":               {"CD"},
		"trackc":              {"14"},
		"year":                {"1967"},
		"album artist credit": {"Steve"},
	}
	normalise(raw, replacements)

	exp := map[string][]string{
		"some":       {"a", "b"}, // convert lower
		"other_test": {"x"},      // replace spaces
		"media":      {"CD"},     // leave alone

		// replacements
		"tracknumber":        {"14"},
		"date":               {"1967"},
		"albumartist_credit": {"Steve"},
	}
	require.Equal(t, exp, raw)
}
