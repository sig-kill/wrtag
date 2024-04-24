package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.senan.xyz/flagconf"

	"go.senan.xyz/wrtag/cmd/internal/flagcommon"
	"go.senan.xyz/wrtag/lyrics"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

var tg tagcommon.Reader = taglib.TagLib{}

var source = flagcommon.Lyrics()
var configPath = flagcommon.ConfigPath()

func main() {
	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	dirs := flag.Args()

	paths := make(chan string)
	go func() {
		for _, d := range dirs {
			err := filepath.WalkDir(d, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if tg.CanRead(path) {
					paths <- path
				}
				return nil
			})
			if err != nil {
				log.Printf("error walking paths: %v", err)
				continue
			}
		}
		close(paths)
	}()

	processTrack := func(ctx context.Context, path string) error {
		f, err := tg.Read(path)
		if err != nil {
			return err
		}
		defer f.Close()

		lyricData, err := source.Search(ctx, f.Artist(), f.Title())
		if err != nil && !errors.Is(err, lyrics.ErrLyricsNotFound) {
			return err
		}
		f.WriteLyrics(lyricData)

		log.Printf("searched %q (%d bytes)", path, len(lyricData))
		return nil
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var start = time.Now()
	var numDone, numError atomic.Uint32

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case dir, ok := <-paths:
					if !ok {
						return
					}
					if err := processTrack(ctx, dir); err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
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

	log.Printf("sync finished in %s with %d paths, %d err", time.Since(start).Truncate(time.Millisecond), numDone.Load(), numError.Load())
}
