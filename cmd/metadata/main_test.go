package main

import (
	_ "embed"
	"fmt"
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
	var err error
	switch name, path := os.Args[0], os.Args[1]; name {
	case "mp3":
		err = os.WriteFile(path, emptyMP3, 0666)
	case "flac":
		err = os.WriteFile(path, emptyFlac, 0666)
	default:
		err = fmt.Errorf("unknown file type %q", name)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func TestParseTagMap(t *testing.T) {
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
