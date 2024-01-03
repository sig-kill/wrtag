package tagcommon

import (
	"errors"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	CanRead(absPath string) bool
	Read(absPath string) (Info, error)
}

type Info interface {
	Title() string
	Artist() string
	Artists() []string
	Album() string
	AlbumArtist() string
	AlbumArtists() []string
	Genre() string
	Genres() []string
	TrackNumber() int
	DiscNumber() int
	MediaFormat() string
	Date() string
	OriginalDate() string
	Label() string
	CatalogueNum() string

	MBRecordingID() string
	MBReleaseID() string
	MBReleaseGroupID() string
	MBArtistID() []string
	MBAlbumArtistID() []string

	Length() int
	Bitrate() int

	String() string
}
