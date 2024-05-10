package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
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
	"go.senan.xyz/wrtag/tags"
)

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
				if tags.CanRead(path) {
					paths <- path
				}
				return nil
			})
			if err != nil {
				slog.Error("walking paths", "err", err)
				continue
			}
		}
		close(paths)
	}()

	processTrack := func(ctx context.Context, path string) error {
		f, err := tags.Read(path)
		if err != nil {
			return err
		}
		defer f.Close()

		lyricData, err := source.Search(ctx, f.Read(tags.Artist), f.Read(tags.Title))
		if err != nil && !errors.Is(err, lyrics.ErrLyricsNotFound) {
			return err
		}
		f.Write(tags.Lyrics, lyricData)
		if err := f.Save(); err != nil {
			return fmt.Errorf("save: %w", err)
		}

		slog.InfoContext(ctx, "processed track", "path", path, "lyric_bytes", len(lyricData))
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
				case path, ok := <-paths:
					if !ok {
						return
					}
					if err := processTrack(ctx, path); err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
						slog.ErrorContext(ctx, "processing track", "path", path, "err", err)
						numError.Add(1)
						continue
					}
					numDone.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	var level slog.Level
	if numError.Load() > 0 {
		level = slog.LevelError
	}
	slog.Log(ctx, level, "sync finished", "took", time.Since(start), "tracks", numDone.Load(), "errs", numDone.Load())
}
