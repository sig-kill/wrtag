package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"go.senan.xyz/wrtag/tags/taglib"
)

func main() {
	var tg taglib.TagLib

	var errs []error
	for _, path := range os.Args[1:] {
		file, err := tg.Read(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", path, err))
			continue
		}

		fmt.Printf("%s\tAlbum\t%s\n", path, format(file.Album()))
		fmt.Printf("%s\tAlbumArtist\t%s\n", path, format(file.AlbumArtist()))
		fmt.Printf("%s\tAlbumArtists\t%s\n", path, format(file.AlbumArtists()))
		fmt.Printf("%s\tAlbumArtistCredit\t%s\n", path, format(file.AlbumArtistCredit()))
		fmt.Printf("%s\tAlbumArtistsCredit\t%s\n", path, format(file.AlbumArtistsCredit()))
		fmt.Printf("%s\tDate\t%s\n", path, format(file.Date()))
		fmt.Printf("%s\tOriginalDate\t%s\n", path, format(file.OriginalDate()))
		fmt.Printf("%s\tMediaFormat\t%s\n", path, format(file.MediaFormat()))
		fmt.Printf("%s\tLabel\t%s\n", path, format(file.Label()))
		fmt.Printf("%s\tCatalogueNum\t%s\n", path, format(file.CatalogueNum()))

		fmt.Printf("%s\tMBReleaseID\t%s\n", path, format(file.MBReleaseID()))
		fmt.Printf("%s\tMBReleaseGroupID\t%s\n", path, format(file.MBReleaseGroupID()))
		fmt.Printf("%s\tMBAlbumArtistID\t%s\n", path, format(file.MBAlbumArtistID()))

		fmt.Printf("%s\tTitle\t%s\n", path, format(file.Title()))
		fmt.Printf("%s\tArtist\t%s\n", path, format(file.Artist()))
		fmt.Printf("%s\tArtists\t%s\n", path, format(file.Artists()))
		fmt.Printf("%s\tArtistCredit\t%s\n", path, format(file.ArtistCredit()))
		fmt.Printf("%s\tArtistsCredit\t%s\n", path, format(file.ArtistsCredit()))
		fmt.Printf("%s\tGenre\t%s\n", path, format(file.Genre()))
		fmt.Printf("%s\tGenres\t%s\n", path, format(file.Genres()))
		fmt.Printf("%s\tTrackNumber\t%v\n", path, format(file.TrackNumber()))
		fmt.Printf("%s\tDiscNumber\t%v\n", path, format(file.DiscNumber()))

		fmt.Printf("%s\tMBRecordingID\t%s\n", path, format(file.MBRecordingID()))
		fmt.Printf("%s\tMBArtistID\t%s\n", path, format(file.MBArtistID()))

		fmt.Printf("%s\tLyrics\t%s\n", path, format(file.Lyrics()))

		if err := file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", path, err))
			continue
		}
	}

	if err := errors.Join(errs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func format(v any) string {
	r, _ := json.Marshal(v)
	return string(r)
}
