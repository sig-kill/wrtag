package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/flagconf"
	"go.senan.xyz/table/table"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flagcommon"
)

var dmp = diffmatchpatch.New()

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
		log.Fatalf("unknown command %q", command)
	}

	subflag := flag.NewFlagSet(command, flag.ExitOnError)
	yes := subflag.Bool("yes", false, "use the found release anyway despite a low score")
	useMBID := subflag.String("mbid", "", "overwrite matched mbid")
	subflag.Parse(flag.Args()[1:])

	dir := subflag.Arg(0)
	if dir == "" {
		log.Fatalf("need a dir")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	r, err := wrtag.ProcessDir(ctx, mb, pathFormat, tagWeights, researchLinkQuerier, keepFiles, op, dir, *useMBID, *yes)
	if err != nil && !errors.Is(err, wrtag.ErrScoreTooLow) {
		log.Fatalf("error processing %q: %v", dir, err)
	}

	log.Printf("matched %.2f%% with https://musicbrainz.org/release/%s", r.Score, r.Release.ID)

	t := table.NewStringWriter()
	for _, d := range r.Diff {
		fmt.Fprintf(t, "%s\t%s\t%s\n", d.Field, fmtDiff(d.Before), fmtDiff(d.After))
	}
	for _, row := range strings.Split(strings.TrimRight(t.String(), "\n"), "\n") {
		log.Print(row)
	}

	if err != nil {
		log.Fatalln(err)
	}
}

func fmtDiff(diff []diffmatchpatch.Diff) string {
	if d := dmp.DiffPrettyText(diff); d != "" {
		return d
	}
	return "[empty]"
}
