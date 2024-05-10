package main

import (
	_ "embed"
	"flag"
	"log"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"metadata": func() int { main(); return 0 },
		"flac":     func() int { mainFile(); return 0 },
		"mp3":      func() int { mainFile(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
}

//go:embed testdata/empty.flac
var emptyFlac []byte

//go:embed testdata/empty.mp3
var emptyMP3 []byte

func mainFile() {
	var d []byte
	switch name := os.Args[0]; name {
	case "mp3":
		d = emptyMP3
	case "flac":
		d = emptyFlac
	default:
		log.Fatalf("unknown filetype %q\n", name)
	}

	flag.Parse()

	path := flag.Arg(0)
	if path == "" {
		log.Fatalf("no path provided")
	}

	if err := os.WriteFile(path, d, 0666); err != nil {
		log.Fatalf("write file: %v\n", err)
	}
}

func TestParseTagMap(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string][]string{}, parseTagMap(nil))
	assert.Equal(t, map[string][]string{}, parseTagMap([]string{","}))
	assert.Equal(t, map[string][]string{}, parseTagMap([]string{",", ","}))
	assert.Equal(t, map[string][]string{"genres": nil}, parseTagMap([]string{"genres"}))
	assert.Equal(t, map[string][]string{"genres": {"a"}}, parseTagMap([]string{"genres", "a"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b"}}, parseTagMap([]string{"genres", "a", "b"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}}, parseTagMap([]string{"genres", "a", "b", "c"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}}, parseTagMap([]string{"genres", "a", "b", "c", ","}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}, "artists": nil}, parseTagMap([]string{"genres", "a", "b", "c", ",", "artists"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}, "artists": {"a", "b"}}, parseTagMap([]string{"genres", "a", "b", "c", ",", "artists", "a", "b"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}, "artists": {"a", "b", "c"}}, parseTagMap([]string{"genres", "a", "b", "c", ",", "artists", "a", "b", "c"}))
}
