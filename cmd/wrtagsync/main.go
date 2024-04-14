package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.senan.xyz/flagconf"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flagparse"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/tagmap"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

var mb wrtag.MusicbrainzClient = musicbrainz.NewClient(http.DefaultClient)
var tg tagcommon.Reader = taglib.TagLib{}

func main() {
	var pathFormat pathformat.Format
	flag.Var(flagparse.PathFormat{&pathFormat}, "path-format", "path format")
	var tagWeights tagmap.TagWeights
	flag.Var(flagparse.TagWeights{&tagWeights}, "tag-weight", "tag weight")
	var keepFiles = map[string]struct{}{}
	flag.Func("keep-file", "files to keep from source directories",
		func(s string) error { keepFiles[s] = struct{}{}; return nil })

	interval := flag.Duration("interval", 0, "max duration a release should be left unsynced")
	dryRun := flag.Bool("dry-run", false, "dry run")

	configPath := flag.String("config-path", flagparse.DefaultConfigPath, "path config file")

	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	// walk the whole root dir by default, or some user provided dirs
	var dirs = []string{pathFormat.Root()}
	if flag.NArg() > 0 {
		dirs = flag.Args()
	}

	leafDirs := map[string]struct{}{}
	for _, dir := range dirs {
		dir, _ := filepath.Abs(dir)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	todo := make(chan string)
	go func() {
		for d := range leafDirs {
			todo <- d
		}
		close(todo)
	}()

	importTime := time.Now()
	processDir := func(ctx context.Context, dir string) error {
		if *interval > 0 {
			info, err := os.Stat(dir)
			if err != nil {
				return fmt.Errorf("stat dir: %w", err)
			}
			if time.Since(info.ModTime()) < *interval {
				return nil
			}
		}
		if _, err := wrtag.ProcessDir(ctx, mb, tg, &pathFormat, tagWeights, nil, keepFiles, wrtag.Move{DryRun: *dryRun}, dir, "", false); err != nil {
			return err
		}
		if err := os.Chtimes(dir, time.Time{}, importTime); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("chtimes %q: %v", dir, err)
		}
		log.Printf("done %q", dir)
		return nil
	}

	start := time.Now()
	var numDone, numError atomic.Uint32
	defer func() {
		log.Printf("finished in %s with %d/%d dirs, %d err",
			time.Since(start).Truncate(time.Millisecond), numDone.Load(), len(leafDirs), numError.Load())
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
				case dir, ok := <-todo:
					if !ok {
						return
					}
					if err := processDir(ctx, dir); err != nil {
						log.Printf("error processing %q: %v", dir, err)
						numError.Add(1)
						continue
					}
					numDone.Add(1)
				}
			}
		}()
	}

	wg.Wait()
}
