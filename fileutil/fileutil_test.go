package fileutil_test

import (
	"testing"

	"go.senan.xyz/wrtag/fileutil"
)

func TestSafePath(t *testing.T) {
	eq(t, fileutil.SafePath("hello"), "hello")
	eq(t, fileutil.SafePath("hello/"), "hello")
	eq(t, fileutil.SafePath("hello/a"), "hello a")
	eq(t, fileutil.SafePath("hello / a"), "hello a")
	eq(t, fileutil.SafePath("hel\x00lo"), "hello")
	eq(t, fileutil.SafePath("a  b"), "a b")
}

func eq[T comparable](t *testing.T, a, b T) {
	t.Helper()
	if a != b {
		t.Errorf("%v != %v", a, b)
	}
}
