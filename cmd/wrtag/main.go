package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/table/table"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/conf"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

// replaced while testing
var mb wrtag.MusicbrainzClient = musicbrainz.NewClient()

var tg tagcommon.Reader = taglib.TagLib{}
var dmp = diffmatchpatch.New()

func main() {
	yes := flag.Bool("yes", false, "use the found release anyway despite a low score")
	useMBID := flag.String("mbid", "", "overwrite matched mbid")
	conf.Parse()

	pathFormat, err := pathformat.New(conf.PathFormat)
	if err != nil {
		log.Fatalf("gen path format: %v", err)
	}

	var researchLinkQuerier = &researchlink.Querier{}
	for _, r := range conf.ResearchLinks {
		if err := researchLinkQuerier.AddSource(r.Name, r.Template); err != nil {
			log.Fatalf("add researchlink querier source: %v", err)
		}
	}

	command, dir := flag.Arg(0), flag.Arg(1)

	var op wrtag.FileSystemOperation
	switch command {
	case "move":
		op = wrtag.Move{}
	case "copy":
		op = wrtag.Copy{}
	case "dry-run":
		op = wrtag.DryRun{}
	default:
		log.Fatalf("unknown command %q", command)
	}
	if dir == "" {
		log.Fatalf("need a dir")
	}

	r, err := wrtag.ProcessDir(context.Background(), mb, tg, pathFormat, researchLinkQuerier, op, dir, *useMBID, *yes)
	if err != nil && !errors.Is(err, wrtag.ErrScoreTooLow) {
		log.Fatalln(err)
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
