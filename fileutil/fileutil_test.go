package fileutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/fileutil"
)

func TestSafePath(t *testing.T) {
	assert.Equal(t, "hello", fileutil.SafePath("hello"))
	assert.Equal(t, "hello", fileutil.SafePath("hello/"))
	assert.Equal(t, "hello a", fileutil.SafePath("hello/a"))
	assert.Equal(t, "hello a", fileutil.SafePath("hello / a"))
	assert.Equal(t, "hello", fileutil.SafePath("hel\x00lo"))
	assert.Equal(t, "a b", fileutil.SafePath("a  b"))
	assert.Equal(t, "(2004) Kesto (234.484)", fileutil.SafePath("(2004) Kesto (234.48:4)"))
	assert.Equal(t, "01.33 Rahina I Mayhem I", fileutil.SafePath("01.33 Rähinä I Mayhem I"))
}
