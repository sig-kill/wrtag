package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/flagconf"
	"go.senan.xyz/table/table"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flagcommon"
)

var mb = flagcommon.MusicBrainz()
var keepFiles = flagcommon.KeepFiles()
var pathFormat = flagcommon.PathFormat()
var researchLinkQuerier = flagcommon.Querier()
var tagWeights = flagcommon.TagWeights()
var configPath = flagcommon.ConfigPath()

var dryRun = flag.Bool("dry-run", false, "dry run")

func main() {
	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	command := flag.Arg(0)
	var op wrtag.FileSystemOperation
	switch command {
	case "move":
		op = wrtag.Move{DryRun: *dryRun}
	case "copy":
		op = wrtag.Copy{DryRun: *dryRun}
	default:
		slog.Error("unknown command", "command", command)
		os.Exit(1)
	}

	subflag := flag.NewFlagSet(command, flag.ExitOnError)
	yes := subflag.Bool("yes", false, "use the found release anyway despite a low score")
	useMBID := subflag.String("mbid", "", "overwrite matched mbid")
	subflag.Parse(flag.Args()[1:])

	dir := subflag.Arg(0)
	if dir == "" {
		slog.Error("need a dir")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	r, err := wrtag.ProcessDir(ctx, mb, pathFormat, tagWeights, researchLinkQuerier, keepFiles, op, dir, *useMBID, *yes)
	if err != nil && !errors.Is(err, wrtag.ErrScoreTooLow) {
		slog.Error("processing", "dir", dir, "err", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "matched",
		"score", fmt.Sprintf("%.2f%%", r.Score),
		"url", fmt.Sprintf("https://musicbrainz.org/release/%s", r.Release.ID),
	)

	t := table.NewStringWriter()
	for _, d := range r.Diff {
		fmt.Fprintf(t, "%s\t%s\t%s\n", d.Field, fmtDiff(d.Before), fmtDiff(d.After))
	}
	for _, row := range strings.Split(strings.TrimRight(t.String(), "\n"), "\n") {
		fmt.Fprintf(os.Stderr, "\t%s\n", row)
	}

	for _, link := range r.ResearchLinks {
		slog.InfoContext(ctx, "search with", "name", link.Name, "url", link.URL)
	}

	if err != nil {
		slog.Error("processing", "dir", dir, "err", err)
		os.Exit(1)
	}
}

var dm = dmp.New()

func fmtDiff(diff []dmp.Diff) string {
	if d := dm.DiffPrettyText(diff); d != "" {
		return d
	}
	return "[empty]"
}
