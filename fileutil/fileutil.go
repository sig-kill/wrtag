package fileutil

import (
	"path/filepath"
	"strings"
)

func GlobEscape(path string) string {
	var r strings.Builder
	for _, c := range path {
		switch c {
		case '*', '?', '[':
			r.WriteRune('[')
			r.WriteRune(c)
			r.WriteRune(']')
		default:
			r.WriteRune(c)
		}
	}
	return r.String()
}

func GlobBase(dir, pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(GlobEscape(dir), pattern))
}

var safePathReplacer = strings.NewReplacer(
	"\x00", "",
	string(filepath.Separator), " ",
)

func SafePath(path string) string {
	path = safePathReplacer.Replace(path)
	path = strings.Join(strings.Fields(path), " ")
	return path
}
