package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"go.senan.xyz/flagconf"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flagparse"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

var mb wrtag.MusicbrainzClient = musicbrainz.NewClient()
var tg tagcommon.Reader = taglib.TagLib{}

func main() {
	var pathFormat pathformat.Format
	flag.Var(flagparse.PathFormat{&pathFormat}, "path-format", "path format")
	var keepFiles = map[string]struct{}{}
	flag.Func("keep-file", "files to keep from source directories",
		func(s string) error { keepFiles[s] = struct{}{}; return nil })
	configPath := flag.String("config-path", flagparse.DefaultConfigPath, "path config file")

	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	leafDirs := map[string]struct{}{}
	err := filepath.WalkDir(pathFormat.Root(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		path = filepath.Clean(path)
		leafDirs[path] = struct{}{}
		delete(leafDirs, filepath.Dir(path)) // parent is not a leaf anymore
		return nil
	})
	if err != nil {
		log.Fatalf("error walking: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	todo := make(chan string)
	go func() {
		for d := range leafDirs {
			todo <- d
		}
		cancel()
	}()

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case dir := <-todo:
					if _, err := wrtag.ProcessDir(ctx, mb, tg, &pathFormat, nil, keepFiles, wrtag.Move{}, dir, "", false); err != nil {
						log.Printf("error processing %q: %v", dir, err)
						continue
					}
					log.Printf("done %s", dir)
				}
			}
		}()
	}

	wg.Wait()
}
