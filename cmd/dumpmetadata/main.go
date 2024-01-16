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
		fmt.Printf("path\t%s\n", path)
		file, err := tg.Read(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", path, err))
			continue
		}

		fmt.Printf("    Album\t%s\n", file.Album())
		fmt.Printf("    AlbumArtist\t%s\n", file.AlbumArtist())
		fmt.Printf("    AlbumArtists\t%s\n", file.AlbumArtists())
		fmt.Printf("    Date\t%s\n", file.Date())
		fmt.Printf("    OriginalDate\t%s\n", file.OriginalDate())
		fmt.Printf("    MediaFormat\t%s\n", file.MediaFormat())
		fmt.Printf("    Label\t%s\n", file.Label())
		fmt.Printf("    CatalogueNum\t%s\n", file.CatalogueNum())

		fmt.Printf("    MBReleaseID\t%s\n", file.MBReleaseID())
		fmt.Printf("    MBReleaseGroupID\t%s\n", file.MBReleaseGroupID())
		fmt.Printf("    MBAlbumArtistID\t%s\n", file.MBAlbumArtistID())

		fmt.Printf("    Title\t%s\n", file.Title())
		fmt.Printf("    Artist\t%s\n", file.Artist())
		fmt.Printf("    Artists\t%s\n", file.Artists())
		fmt.Printf("    Genre\t%s\n", file.Genre())
		fmt.Printf("    Genres\t%s\n", file.Genres())
		fmt.Printf("    TrackNumber\t%v\n", file.TrackNumber())
		fmt.Printf("    DiscNumber\t%v\n", file.DiscNumber())

		fmt.Printf("    MBRecordingID\t%s\n", file.MBRecordingID())
		fmt.Printf("    MBArtistID\t%s\n", file.MBArtistID())

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
