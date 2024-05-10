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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.senan.xyz/flagconf"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flagcommon"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/notifications"
)

var mb = flagcommon.MusicBrainz()
var keepFiles = flagcommon.KeepFiles()
var notifs = flagcommon.Notifications()
var pathFormat = flagcommon.PathFormat()
var tagWeights = flagcommon.TagWeights()
var configPath = flagcommon.ConfigPath()

var interval = flag.Duration("interval", 0, "max duration a release should be left unsynced")
var dryRun = flag.Bool("dry-run", false, "dry run")

func main() {
	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	// walk the whole root dir by default, or some user provided dirs
	var dirs = []string{pathFormat.Root()}
	if flag.NArg() > 0 {
		dirs = flag.Args()
	}

	leaves := make(chan string)
	go func() {
		for _, d := range dirs {
			err := fileutil.WalkLeaves(d, func(path string, _ fs.DirEntry) error {
				leaves <- path
				return nil
			})
			if err != nil {
				slog.Error("walking leaves", "err", err)
				continue
			}
		}
		close(leaves)
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
		if _, err := wrtag.ProcessDir(ctx, mb, pathFormat, tagWeights, nil, keepFiles, wrtag.Move{DryRun: *dryRun}, dir, "", false); err != nil {
			return err
		}
		if err := os.Chtimes(dir, time.Time{}, importTime); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("chtimes %q: %v", dir, err)
		}
		slog.InfoContext(ctx, "processed dir", "dir", dir)
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
				case dir, ok := <-leaves:
					if !ok {
						return
					}
					if err := processDir(ctx, dir); err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
						slog.ErrorContext(ctx, "processing dir", "dir", dir, "err", err)
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
	slog.Log(ctx, level, "sync finished", "took", time.Since(start), "dirs", numDone.Load(), "errs", numError.Load())

	notifs.Send(ctx, notifications.SyncComplete, "sync complete")
	if numError.Load() > 0 {
		notifs.Send(ctx, notifications.SyncError, "sync errors")
	}
}
