package musicbrainz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUID(t *testing.T) {
	assert.False(t, uuidExpr.MatchString(""))
	assert.False(t, uuidExpr.MatchString("123"))
	assert.False(t, uuidExpr.MatchString("uhh dd720ac8-1c68-4484-abb7-0546413a55e3"))
	assert.True(t, uuidExpr.MatchString("dd720ac8-1c68-4484-abb7-0546413a55e3"))
	assert.True(t, uuidExpr.MatchString("DD720AC8-1C68-4484-ABB7-0546413A55E3"))
}

func TestMergeAndSortGenres(t *testing.T) {
	require.Equal(t,
		[]Genre{
			{ID: "a psychedelic", Name: "a psychedelic", Count: 3},
			{ID: "psy trance", Name: "psy trance", Count: 3},
			{ID: "techno", Name: "techno", Count: 2},
			{ID: "electronic a", Name: "electronic a", Count: 1},
			{ID: "electronic b", Name: "electronic b", Count: 1},
		},
		mergeAndSortGenres([]Genre{
			{ID: "electronic b", Name: "electronic b", Count: 1},
			{ID: "electronic a", Name: "electronic a", Count: 1},
			{ID: "psy trance", Name: "psy trance", Count: 3},
			{ID: "a psychedelic", Name: "a psychedelic", Count: 2},
			{ID: "a psychedelic", Name: "a psychedelic", Count: 1},
			{ID: "techno", Name: "techno", Count: 2},
		}),
	)
}
