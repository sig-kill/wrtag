package main

import (
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

		fmt.Printf("%s\tAlbum\t%s\n", path, file.Album())
		fmt.Printf("%s\tAlbumArtist\t%s\n", path, file.AlbumArtist())
		fmt.Printf("%s\tAlbumArtists\t%s\n", path, file.AlbumArtists())
		fmt.Printf("%s\tDate\t%s\n", path, file.Date())
		fmt.Printf("%s\tOriginalDate\t%s\n", path, file.OriginalDate())
		fmt.Printf("%s\tMediaFormat\t%s\n", path, file.MediaFormat())
		fmt.Printf("%s\tLabel\t%s\n", path, file.Label())
		fmt.Printf("%s\tCatalogueNum\t%s\n", path, file.CatalogueNum())

		fmt.Printf("%s\tMBReleaseID\t%s\n", path, file.MBReleaseID())
		fmt.Printf("%s\tMBReleaseGroupID\t%s\n", path, file.MBReleaseGroupID())
		fmt.Printf("%s\tMBAlbumArtistID\t%s\n", path, file.MBAlbumArtistID())

		fmt.Printf("%s\tTitle\t%s\n", path, file.Title())
		fmt.Printf("%s\tArtist\t%s\n", path, file.Artist())
		fmt.Printf("%s\tArtists\t%s\n", path, file.Artists())
		fmt.Printf("%s\tGenre\t%s\n", path, file.Genre())
		fmt.Printf("%s\tGenres\t%s\n", path, file.Genres())
		fmt.Printf("%s\tTrackNumber\t%v\n", path, file.TrackNumber())
		fmt.Printf("%s\tDiscNumber\t%v\n", path, file.DiscNumber())

		fmt.Printf("%s\tMBRecordingID\t%s\n", path, file.MBRecordingID())
		fmt.Printf("%s\tMBArtistID\t%s\n", path, file.MBArtistID())

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
