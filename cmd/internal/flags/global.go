package flags

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/clientutil"
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

var httpClient *http.Client

func init() {
	httpClient = &http.Client{Transport: clientutil.Chain(
		clientutil.WithLogging(slog.Default()),
		clientutil.WithUserAgent(fmt.Sprintf(`wrtag/%s`, wrtag.Version)),
	)(http.DefaultTransport)}

	http.DefaultClient = httpClient
}
