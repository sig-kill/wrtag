package tagmap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/tagmap"
)

func TestDiffer(t *testing.T) {
	var score float64
	diff := tagmap.Differ(tagmap.TagWeights{}, &score)

	diff("x", ".....", ".....")
	diff("x", ".....", "....X")
	assert.Equal(t, 90.0, score) // 9 of 10 chars the same
}

func TestDiffWeightsLowerBound(t *testing.T) {
	weights := tagmap.TagWeights{
		"label":         0,
		"catalogue num": 0,
	}

	var score float64
	diff := tagmap.Differ(weights, &score)

	// all the same, but label/catalogue num mismatch
	diff("label", "Columbia", "uh some other label")
	diff("catalogue num", "Columbia", "not the same catalogue num")

	diff("track 1", "The Day I Met God", "The Day I Met God")
	diff("track 2", "Catholic Day", "Catholic Day")
	diff("track 3", "Nine Plan Failed", "Nine Plan Failed")
	diff("track 4", "Family of Noise", "Family of Noise")
	diff("track 5", "Digital Tenderness", "Digital Tenderness")

	// but that's fine since we gave those 0 weight
	assert.Equal(t, 100.0, score)
}

func TestDiffWeightsUpperBound(t *testing.T) {
	weights := tagmap.TagWeights{
		"label":         2,
		"catalogue num": 2,
	}

	var score float64
	diff := tagmap.Differ(weights, &score)

	// all the same, but label/catalogue num mismatch
	diff("label", "Columbia", "uh some other label")
	diff("catalogue num", "Columbia", "not the same catalogue num")

	diff("track 1", "The Day I Met God", "The Day I Met God")
	diff("track 2", "Catholic Day", "Catholic Day")
	diff("track 3", "Nine Plan Failed", "Nine Plan Failed")
	diff("track 4", "Family of Noise", "Family of Noise")
	diff("track 5", "Digital Tenderness", "Digital Tenderness")

	// bad score since we really care about label / catalogue num
	assert.InDelta(t, 41.0, score, 1)
}
