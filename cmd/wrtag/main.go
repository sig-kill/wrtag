package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/release"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"

	"github.com/peterbourgon/ff/v4"
)

func main() {
	fs := ff.NewFlagSet("wrtag")
	_ = fs.StringLong("path-format", "", "path format")
	_ = fs.StringLong("config", "", "config file (optional)")

	userConfig, _ := os.UserConfigDir()
	configPath := filepath.Join(userConfig, "wrtag", "config")

	ffopt := []ff.Option{
		ff.WithEnvVarPrefix("WRTAG"),
		ff.WithConfigFileFlag("config"),
	}
	if stat, err := os.Stat(configPath); err == nil && stat.Mode().IsRegular() {
		ffopt = append(ffopt,
			ff.WithConfigFile(configPath),
			ff.WithConfigFileParser(ff.PlainParser),
		)
	}
	if err := ff.Parse(fs, os.Args[1:], ffopt...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tg := taglib.TagLib{}
	mb := musicbrainz.NewClient()

	for _, dir := range fs.GetArgs() {
		if err := processDir(tg, mb, dir); err != nil {
			log.Printf("error processing dir %q: %v", dir, err)
		}
	}
}

func processDir(tg taglib.TagLib, mb *musicbrainz.Client, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var tags []tagcommon.Info
	for _, entry := range entries {
		if path := filepath.Join(dir, entry.Name()); tg.CanRead(path) {
			info, err := tg.Read(path)
			if err != nil {
				return fmt.Errorf("read track info: %w", err)
			}
			tags = append(tags, info)
		}
	}
	if len(tags) == 0 {
		return fmt.Errorf("no tracks in dir")
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].TrackNumber() < tags[j].TrackNumber()
	})
	tag := tags[0]

	var query musicbrainz.Query
	query.MBReleaseID = tag.MBReleaseID()
	query.MBArtistID = first(tag.MBArtistID())
	query.MBReleaseGroupID = tag.MBReleaseGroupID()
	query.Release = tag.Album()
	query.Artist = tag.AlbumArtist()
	query.Format = tag.MediaFormat()
	query.Date = tag.Date()
	query.Label = tag.Label()
	query.CatalogueNum = tag.CatalogueNum()
	query.NumTracks = len(tags)

	score, resp, err := mb.SearchRelease(context.Background(), query)
	if err != nil {
		return fmt.Errorf("search release: %w", err)
	}
	if score < 90 {
		return fmt.Errorf("score too low")
	}

	releaseA := release.FromTagInfo(tags)
	releaseB := release.FromMusicBrainz(resp)
	if len(releaseA.Tracks) != len(releaseB.Tracks) {
		return fmt.Errorf("track count mismatch %d/%d", len(releaseA.Tracks), len(releaseB.Tracks))
	}

	fmt.Println()
	fmt.Printf("dir: %q\n", dir)
	fmt.Print(release.Diff(releaseA, releaseB))

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
