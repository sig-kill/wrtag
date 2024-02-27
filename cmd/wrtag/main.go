package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	texttemplate "text/template"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/table/table"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/conf"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/tagmap"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

var dmp = diffmatchpatch.New()

// replaced while testing
var mb musicbrainzClient = musicbrainz.NewClient()

func main() {
	yes := flag.Bool("yes", false, "use the found release anyway despite a low score")
	conf.Parse()

	var tg tagcommon.Reader = taglib.TagLib{}

	pathFormat, err := pathformat.New(conf.PathFormat)
	if err != nil {
		log.Fatalf("gen path format: %v", err)
	}

	if flag.NArg() != 1 {
		log.Fatalf("need a path")
	}

	if err := processJob(context.Background(), mb, tg, pathFormat, flag.Arg(0), *yes); err != nil {
		log.Fatalf("error processing dir: %v", err)
	}
}

type musicbrainzClient interface {
	SearchRelease(ctx context.Context, q musicbrainz.ReleaseQuery) (*musicbrainz.Release, error)
}

func processJob(
	ctx context.Context, mb musicbrainzClient, tg tagcommon.Reader,
	pathFormat *texttemplate.Template,
	path string,
	yes bool,
) (err error) {
	tagFiles, err := wrtag.ReadDir(tg, path)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", path, err)
	}

	searchFile := tagFiles[0]
	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      searchFile.MBReleaseID(),
		MBArtistID:       first(searchFile.MBArtistID()),
		MBReleaseGroupID: searchFile.MBReleaseGroupID(),
		Release:          searchFile.Album(),
		Artist:           or(searchFile.AlbumArtist(), searchFile.Artist()),
		Date:             searchFile.Date(),
		Format:           searchFile.MediaFormat(),
		Label:            searchFile.Label(),
		CatalogueNum:     searchFile.CatalogueNum(),
		NumTracks:        len(tagFiles),
	}

	release, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	score, diff := tagmap.DiffRelease(release, tagFiles)
	log.Printf("matched %.2f%% with https://musicbrainz.org/release/%s", score, release.ID)

	t := table.NewStringWriter()
	for _, d := range diff {
		fmt.Fprintf(t, "%s\t%s\t%s\n", d.Field, fmtDiff(d.Before), fmtDiff(d.After))
	}
	for _, row := range strings.Split(strings.TrimRight(t.String(), "\n"), "\n") {
		log.Print(row)
	}

	if releaseTracks := musicbrainz.FlatTracks(release.Media); len(tagFiles) != len(releaseTracks) {
		return fmt.Errorf("%w: %d/%d", wrtag.ErrTrackCountMismatch, len(tagFiles), len(releaseTracks))
	}

	if !yes && score < 95 {
		return fmt.Errorf("score too low")
	}

	// write release to tags. files are saved by defered Close()
	tagmap.WriteRelease(release, tagFiles)

	if err := wrtag.MoveFiles(pathFormat, release, nil); err != nil {
		return fmt.Errorf("move files: %w", err)
	}

	return nil
}

func first[T comparable](is []T) T {
	var z T
	for _, i := range is {
		if i != z {
			return i
		}
	}
	return z
}

func or[T comparable](items ...T) T {
	var zero T
	for _, i := range items {
		if i != zero {
			return i
		}
	}
	return zero
}

func fmtDiff(diff []diffmatchpatch.Diff) string {
	if d := dmp.DiffPrettyText(diff); d != "" {
		return d
	}
	return "[empty]"
}
