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

		file.WriteAlbum("")
		file.WriteAlbumArtist("")
		file.WriteAlbumArtists(nil)
		file.WriteAlbumArtistCredit("")
		file.WriteAlbumArtistsCredit(nil)
		file.WriteDate("")
		file.WriteOriginalDate("")
		file.WriteMediaFormat("")
		file.WriteLabel("")
		file.WriteCatalogueNum("")

		file.WriteMBReleaseID("")
		file.WriteMBReleaseGroupID("")
		file.WriteMBAlbumArtistID(nil)

		file.WriteTitle("")
		file.WriteArtist("")
		file.WriteArtists(nil)
		file.WriteArtistCredit("")
		file.WriteArtistsCredit(nil)
		file.WriteGenre("")
		file.WriteGenres(nil)
		file.WriteTrackNumber(0)
		file.WriteDiscNumber(0)

		file.WriteMBRecordingID("")
		file.WriteMBArtistID(nil)

		file.WriteLyrics("")

		if ru, ok := file.(interface{ RemoveUnknown() }); ok {
			ru.RemoveUnknown()
		}

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
