package main

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"metadata": func() int { main(); return 0 },
		"flac":     func() int { mainFlac(); return 0 },
		"mp3":      func() int { mainMP3(); return 0 },
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

func mainFlac() {
	_, _ = io.Copy(os.Stdout, bytes.NewReader(emptyFlac))
}

//go:embed testdata/empty.mp3
var emptyMP3 []byte

func mainMP3() {
	_, _ = io.Copy(os.Stdout, bytes.NewReader(emptyMP3))
}
