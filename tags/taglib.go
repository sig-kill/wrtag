package tags

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/sentriz/audiotags"
)

var ErrWrite = errors.New("error writing tags")

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html
const (
	Album              = "album"
	AlbumArtist        = "albumartist"
	AlbumArtists       = "albumartists"
	AlbumArtistCredit  = "albumartist_credit"
	AlbumArtistsCredit = "albumartists_credit"
	Date               = "date"
	OriginalDate       = "originaldate"
	MediaFormat        = "media"
	Label              = "label"
	CatalogueNum       = "catalognumber"

	MBReleaseID      = "musicbrainz_albumid"
	MBReleaseGroupID = "musicbrainz_releasegroupid"
	MBAlbumArtistID  = "musicbrainz_albumartistid"

	Title         = "title"
	Artist        = "artist"
	Artists       = "artists"
	ArtistCredit  = "artist_credit"
	ArtistsCredit = "artists_credit"
	Genre         = "genre"
	Genres        = "genres"
	TrackNumber   = "tracknumber"
	DiscNumber    = "discnumber"

	MBRecordingID = "musicbrainz_trackid"
	MBArtistID    = "musicbrainz_artistid"

	Lyrics = "lyrics"
)

// tags we can use instead if we dont have the ones we're expecting.
// it maps [bad] -> [good]
var replacements = map[string]string{
	"year":                Date,
	"original_year":       OriginalDate,
	"track":               TrackNumber,
	"trackc":              TrackNumber,
	"catalognum":          CatalogueNum,
	"album_artists":       AlbumArtists,
	"album artist credit": AlbumArtistCredit,
	"artist credit":       ArtistCredit,
}

func CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

type File struct {
	raw  map[string][]string
	file *audiotags.File
}

func Read(absPath string) (*File, error) {
	f, err := audiotags.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	raw := f.ReadTags()
	normalise(raw, replacements)
	return &File{raw: raw, file: f}, nil
}

func (f *File) Read(t string) string        { return first(f.raw[t]) }
func (f *File) ReadMulti(t string) []string { return f.raw[t] }
func (f *File) ReadNum(t string) int        { return anyNum(first(f.raw[t])) }
func (f *File) ReadTime(t string) time.Time { return anyTime(first(f.raw[t])) }

func (f *File) ReadAll(fn func(k string, vs []string)) {
	for k, vs := range f.raw {
		fn(k, vs)
	}
}

func (f *File) Write(t string, v ...string) { f.raw[t] = v }
func (f *File) WriteNum(t string, v int)    { f.raw[t] = []string{intStr(v)} }

func (f *File) ClearAll() { clear(f.raw) }

func (f *File) Save() error {
	if !f.file.WriteTags(f.raw) {
		return ErrWrite
	}
	return nil
}

func (f *File) Close() {
	f.file.Close()
}

func first(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func intStr(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

var numExpr = regexp.MustCompile(`\d+`)

func anyNum(in string) int {
	match := numExpr.FindString(in)
	i, _ := strconv.Atoi(match)
	return i
}

func anyTime(str string) time.Time {
	t, _ := dateparse.ParseAny(str)
	return t
}

func normalise(raw map[string][]string, fallbacks map[string]string) {
	for kbad, kgood := range fallbacks {
		if _, ok := raw[kgood]; ok {
			continue
		}
		if v, ok := raw[kbad]; ok {
			raw[kgood] = v
			delete(raw, kbad)
			continue
		}
	}

	for k, vs := range raw {
		kNew := k
		kNew = strings.ToLower(kNew)
		kNew = strings.ReplaceAll(kNew, " ", "_")
		if k == kNew {
			continue
		}
		delete(raw, k)
		raw[kNew] = vs
	}
}
