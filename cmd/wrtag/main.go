package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/taglib"
)

func main() {
	flag.Parse()

	dir := flag.Arg(0)
	if dir == "" {
		return
	}

	var tg taglib.TagLib

	entries, err := os.ReadDir(dir)
	cerr(err)

	var tracks []string
	for _, entry := range entries {
		if path := filepath.Join(dir, entry.Name()); tg.CanRead(path) {
			tracks = append(tracks, path)
		}
	}
	if len(tracks) == 0 {
		return
	}

	info, err := tg.Read(tracks[0])
	cerr(err)

	var query musicbrainz.Query
	query.MBReleaseID = info.MBReleaseID()
	query.MBArtistID = first(info.MBArtistID())
	query.MBReleaseGroupID = info.MBReleaseGroupID()
	query.Release = info.Album()
	query.Artist = info.AlbumArtist()
	query.Format = info.Media()
	query.Date = info.Date()
	query.Label = info.Label()
	query.CatalogueNum = info.CatalogueNum()
	query.NumTracks = len(tracks)

	var mb musicbrainz.Client
	score, release, err := mb.SearchRelease(query)
	cerr(err)

	fmt.Printf("score: %d\n", score)
	fmt.Printf("release:\n")
	fmt.Printf("  name      : %q -> %q\n", info.Album(), release.Title)
	fmt.Printf("  artist    : %q -> %q\n", info.AlbumArtist(), creditString(release.ArtistCredit))
	fmt.Printf("  label     : %q -> %q\n", info.Label(), first(release.LabelInfo).Label.Name)
	fmt.Printf("  catalogue : %q -> %q\n", info.CatalogueNum(), first(release.LabelInfo).CatalogNumber)
	fmt.Printf("  media     : %q -> %q\n", info.Media(), release.Media[0].Format)
	fmt.Printf("tracks:\n")
}

func cerr(err error) {
	if err != nil {
		panic(err)
	}
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
