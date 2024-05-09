package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
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
