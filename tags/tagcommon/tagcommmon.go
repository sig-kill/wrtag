package tagcommon

import (
	"errors"
	"time"
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
	AlbumArtistCredit() string
	AlbumArtistsCredit() []string
	Date() time.Time
	OriginalDate() time.Time
	MediaFormat() string
	Label() string
	CatalogueNum() string

	MBReleaseID() string
	MBReleaseGroupID() string
	MBAlbumArtistID() []string

	Title() string
	Artist() string
	Artists() []string
	ArtistCredit() string
	ArtistsCredit() []string
	Genre() string
	Genres() []string
	TrackNumber() int
	DiscNumber() int

	MBRecordingID() string
	MBArtistID() []string

	WriteAlbum(v string)
	WriteAlbumArtist(v string)
	WriteAlbumArtists(v []string)
	WriteAlbumArtistCredit(v string)
	WriteAlbumArtistsCredit(v []string)
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
	WriteArtistCredit(v string)
	WriteArtistsCredit(v []string)
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
