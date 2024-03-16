package fileutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/fileutil"
)

func TestSafePath(t *testing.T) {
	assert.Equal(t, fileutil.SafePath("hello"), "hello")
	assert.Equal(t, fileutil.SafePath("hello/"), "hello")
	assert.Equal(t, fileutil.SafePath("hello/a"), "hello a")
	assert.Equal(t, fileutil.SafePath("hello / a"), "hello a")
	assert.Equal(t, fileutil.SafePath("hel\x00lo"), "hello")
	assert.Equal(t, fileutil.SafePath("a  b"), "a b")
}
