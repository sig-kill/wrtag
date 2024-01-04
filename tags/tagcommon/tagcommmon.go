package tagcommon

import (
	"errors"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	CanRead(absPath string) bool
	Read(absPath string) (File, error)
}

type File interface {
	Album() string
	AlbumArtist() string
	AlbumArtists() []string
	Date() string
	OriginalDate() string
	MediaFormat() string
	Label() string
	CatalogueNum() string

	MBReleaseID() string
	MBReleaseGroupID() string
	MBAlbumArtistID() []string

	Title() string
	Artist() string
	Artists() []string
	Genre() string
	Genres() []string
	TrackNumber() int
	DiscNumber() int

	MBRecordingID() string
	MBArtistID() []string

	WriteAlbum(v string)
	WriteAlbumArtist(v string)
	WriteAlbumArtists(v []string)
	WriteDate(v string)
	WriteOriginalDate(v string)
	WriteMediaFormat(v string)
	WriteLabel(v string)
	WriteCatalogueNum(v string)

	WriteMBReleaseID(v string)
	WriteMBReleaseGroupID(v string)
	WriteMBAlbumArtistID(v []string)

	WriteTitle(v string)
	WriteArtist(v string)
	WriteArtists(v []string)
	WriteGenre(v string)
	WriteGenres(v []string)
	WriteTrackNumber(v int)
	WriteDiscNumber(v int)

	WriteMBRecordingID(v string)
	WriteMBArtistID(v []string)

	Length() int
	Bitrate() int

	String() string

	Close() error
}
