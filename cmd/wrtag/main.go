package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

func main() {
	flag.Parse()

	tg := taglib.TagLib{}
	mb := musicbrainz.NewClient()

	for _, dir := range flag.Args() {
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

	var tracks []tagcommon.Info
	for _, entry := range entries {
		if path := filepath.Join(dir, entry.Name()); tg.CanRead(path) {
			info, err := tg.Read(path)
			if err != nil {
				return fmt.Errorf("read track info: %w", err)
			}
			tracks = append(tracks, info)
		}
	}
	if len(tracks) == 0 {
		return fmt.Errorf("no tracks in dir")
	}
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].TrackNumber() < tracks[j].TrackNumber()
	})
	releaseInfo := tracks[0]

	var query musicbrainz.Query
	query.MBReleaseID = releaseInfo.MBReleaseID()
	query.MBArtistID = first(releaseInfo.MBArtistID())
	query.MBReleaseGroupID = releaseInfo.MBReleaseGroupID()
	query.Release = releaseInfo.Album()
	query.Artist = releaseInfo.AlbumArtist()
	query.Format = releaseInfo.Media()
	query.Date = releaseInfo.Date()
	query.Label = releaseInfo.Label()
	query.CatalogueNum = releaseInfo.CatalogueNum()
	query.NumTracks = len(tracks)

	score, release, err := mb.SearchRelease(context.Background(), query)
	if err != nil {
		return fmt.Errorf("search release: %w", err)
	}

	var flatTracks []musicbrainz.Track
	for _, ts := range release.Media {
		flatTracks = append(flatTracks, ts.Tracks...)
	}
	if len(tracks) == 0 {
		return fmt.Errorf("search release: %w", err)
	}
	if score < 90 {
		return fmt.Errorf("score too low")
	}

	fmt.Println()
	fmt.Printf("dir: %q\n", dir)
	fmt.Printf("score: %d\n", score)
	fmt.Printf("release:\n")
	fmt.Printf("  name      : %q -> %q\n", releaseInfo.Album(), release.Title)
	fmt.Printf("  artist    : %q -> %q\n", releaseInfo.AlbumArtist(), creditString(release.ArtistCredit))
	fmt.Printf("  label     : %q -> %q\n", releaseInfo.Label(), first(release.LabelInfo).Label.Name)
	fmt.Printf("  catalogue : %q -> %q\n", releaseInfo.CatalogueNum(), first(release.LabelInfo).CatalogNumber)
	fmt.Printf("  media     : %q -> %q\n", releaseInfo.Media(), release.Media[0].Format)
	fmt.Printf("tracks:\n")
	for i := range tracks {
		fmt.Printf("  %02d  : %q %q\n     -> %q %q\n",
			i,
			tracks[i].Artist(), tracks[i].Title(),
			creditString(flatTracks[i].ArtistCredit), flatTracks[i].Title)
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

func creditString(artists []musicbrainz.ArtistCredit) string {
	var sb strings.Builder
	for _, ar := range artists {
		fmt.Fprintf(&sb, "%s%s", ar.Name, ar.JoinPhrase)
	}
	return sb.String()
}
