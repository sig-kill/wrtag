package taglib

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/sentriz/audiotags"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

var ErrWrite = errors.New("error writing tags")

type TagLib struct{}

func (TagLib) CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

func (TagLib) Read(absPath string) (tagcommon.File, error) {
	f, err := audiotags.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	props := f.ReadAudioProperties()
	raw := f.ReadTags()
	return &File{raw: raw, props: props, file: f}, nil
}

type File struct {
	raw      map[string][]string
	props    *audiotags.AudioProperties
	file     *audiotags.File
	didWrite bool
}

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

func (f *File) Album() string          { return first(find(f.raw, "album")) }
func (f *File) AlbumArtist() string    { return first(find(f.raw, "albumartist", "album artist")) }
func (f *File) AlbumArtists() []string { return find(f.raw, "albumartists", "album_artists") }
func (f *File) AlbumArtistCredit() string {
	return first(find(f.raw, "albumartist_credit", "album artist credit"))
}
func (f *File) AlbumArtistsCredit() []string {
	return find(f.raw, "albumartists_credit", "album_artists_credit")
}
func (f *File) Date() time.Time         { return anyTime(first(find(f.raw, "date", "year"))) }
func (f *File) OriginalDate() time.Time { return anyTime(first(find(f.raw, "originaldate"))) }
func (f *File) MediaFormat() string     { return first(find(f.raw, "media")) }
func (f *File) Label() string           { return first(find(f.raw, "label")) }
func (f *File) CatalogueNum() string    { return first(find(f.raw, "catalognumber")) }

func (f *File) MBReleaseID() string       { return first(find(f.raw, "musicbrainz_albumid")) }
func (f *File) MBReleaseGroupID() string  { return first(find(f.raw, "musicbrainz_releasegroupid")) }
func (f *File) MBAlbumArtistID() []string { return find(f.raw, "musicbrainz_albumartistid") }

func (f *File) Title() string           { return first(find(f.raw, "title")) }
func (f *File) Artist() string          { return first(find(f.raw, "artist")) }
func (f *File) Artists() []string       { return find(f.raw, "artists") }
func (f *File) ArtistCredit() string    { return first(find(f.raw, "artist_credit")) }
func (f *File) ArtistsCredit() []string { return find(f.raw, "artists_credit") }
func (f *File) Genre() string           { return first(find(f.raw, "genre")) }
func (f *File) Genres() []string        { return find(f.raw, "genres") }
func (f *File) TrackNumber() int        { return intSep("/", first(find(f.raw, "tracknumber", "track"))) } // eg. 5/12
func (f *File) DiscNumber() int         { return intSep("/", first(find(f.raw, "discnumber"))) }           // eg. 1/2

func (f *File) MBRecordingID() string { return first(find(f.raw, "musicbrainz_trackid")) }
func (f *File) MBArtistID() []string  { return find(f.raw, "musicbrainz_artistid") }

func (f *File) WriteAlbum(v string)                { f.set("album", v) }
func (f *File) WriteAlbumArtist(v string)          { f.set("albumartist", v) }
func (f *File) WriteAlbumArtists(v []string)       { f.set("albumartists", v...) }
func (f *File) WriteAlbumArtistCredit(v string)    { f.set("albumartist_credit", v) }
func (f *File) WriteAlbumArtistsCredit(v []string) { f.set("albumartists_credit", v...) }
func (f *File) WriteDate(v string)                 { f.set("date", v) }
func (f *File) WriteOriginalDate(v string)         { f.set("originaldate", v) }
func (f *File) WriteMediaFormat(v string)          { f.set("media", v) }
func (f *File) WriteLabel(v string)                { f.set("label", v) }
func (f *File) WriteCatalogueNum(v string)         { f.set("catalognumber", v) }

func (f *File) WriteMBReleaseID(v string)       { f.set("musicbrainz_albumid", v) }
func (f *File) WriteMBReleaseGroupID(v string)  { f.set("musicbrainz_releasegroupid", v) }
func (f *File) WriteMBAlbumArtistID(v []string) { f.set("musicbrainz_albumartistid", v...) }

func (f *File) WriteTitle(v string)           { f.set("title", v) }
func (f *File) WriteArtist(v string)          { f.set("artist", v) }
func (f *File) WriteArtists(v []string)       { f.set("artists", v...) }
func (f *File) WriteArtistCredit(v string)    { f.set("artist_credit", v) }
func (f *File) WriteArtistsCredit(v []string) { f.set("artists_credit", v...) }
func (f *File) WriteGenre(v string)           { f.set("genre", v) }
func (f *File) WriteGenres(v []string)        { f.set("genres", v...) }
func (f *File) WriteTrackNumber(v int)        { f.set("track", intStr(v)) }
func (f *File) WriteDiscNumber(v int)         { f.set("discnumber", intStr(v)) }

func (f *File) WriteMBRecordingID(v string) { f.set("musicbrainz_trackid", v) }
func (f *File) WriteMBArtistID(v []string)  { f.set("musicbrainz_artistid", v...) }

func (f *File) set(k string, vs ...string) {
	f.didWrite = true
	f.raw[k] = vs
}

func (f *File) Length() int  { return f.props.Length }
func (f *File) Bitrate() int { return f.props.Bitrate }

func (f *File) Raw() map[string][]string { return f.raw }

func (f *File) String() string {
	var buf strings.Builder
	for k, v := range f.raw {
		fmt.Fprintf(&buf, "%s\t%v\n", k, strings.Join(v, "; "))
	}
	return buf.String()
}

func (f *File) RemoveUnknown() {
	for k := range f.raw {
		switch strings.ToLower(k) {
		// TODO: re use from above somehow
		case "title", "artist", "artists", "artist_credit", "artists_credit", "album", "albumartist", "albumartists", "albumartist_credit", "albumartists_credit", "genre", "genres", "track", "discnumber", "media", "date", "originaldate", "label", "catalognumber",
			"musicbrainz_trackid", "musicbrainz_albumid", "musicbrainz_releasegroupid", "musicbrainz_artistid", "musicbrainz_albumartistid":
		default:
			delete(f.raw, k)
		}
	}
}

func (f *File) Close() error {
	defer f.file.Close()
	if f.didWrite {
		if !f.file.WriteTags(f.raw) {
			return ErrWrite
		}
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

func intStr(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func find(m map[string][]string, keys ...string) []string {
	for _, k := range keys {
		if r := filterStr(m[k]); len(r) > 0 {
			return r
		}
	}
	return nil
}

func filterStr(ss []string) []string {
	var r []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			r = append(r, s)
		}
	}
	return r
}

func intSep(sep, in string) int {
	start, _, _ := strings.Cut(in, sep)
	out, _ := strconv.Atoi(start)
	return out
}

func anyTime(str string) time.Time {
	t, _ := dateparse.ParseAny(str)
	return t
}
