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

	"go.senan.xyz/wrtag/cmd/internal/flags"
	"go.senan.xyz/wrtag/lyrics"
	"go.senan.xyz/wrtag/tags"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
	}
}

func main() {
	defer flags.ExitError()
	var (
		source = flags.Lyrics()
	)
	flags.EnvPrefix("wrtag") // reuse main binary's namespace
	flags.Parse()

	dirs := flag.Args()

	start := time.Now()
	paths := make(chan string)
	go func() {
		for _, d := range dirs {
			d, _ = filepath.Abs(d)
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var doneN, errN atomic.Uint32

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctxConsume(ctx, paths, func(dir string) {
				if err := processTrack(ctx, source, dir); err != nil {
					slog.ErrorContext(ctx, "processing dir", "dir", dir, "err", err)
					errN.Add(1)
					return
				}
				doneN.Add(1)
			})
		}()
	}

	wg.Wait()

	slog := slog.With("took", time.Since(start), "dirs", doneN.Load(), "errs", errN.Load())
	if errN.Load() > 0 {
		slog.Error("sync finished with errors")
		return
	}
	slog.Info("sync finished")
}

func processTrack(ctx context.Context, source lyrics.Source, path string) error {
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

func ctxConsume[T any](ctx context.Context, work <-chan T, f func(T)) {
	for {
		select { // prority select for ctx.Done()
		case <-ctx.Done():
			return
		default:
			select {
			case <-ctx.Done():
				return
			case w, ok := <-work:
				if !ok {
					return
				}
				f(w)
			}
		}
	}
}
