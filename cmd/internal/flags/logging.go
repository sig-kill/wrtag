package flags

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

var logLevel slog.LevelVar

func init() {
	h := &slogHandler{
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &logLevel}),
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelError)
}

var hadSlogError atomic.Bool

func ExitError() {
	if hadSlogError.Load() {
		os.Exit(1)
	}
	os.Exit(0)
}

type slogHandler struct {
	slog.Handler
}

func (n *slogHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelError {
		hadSlogError.Store(true)
	}
	return n.Handler.Handle(ctx, r)
}
