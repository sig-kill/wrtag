package taglib

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sentriz/audiotags"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

type TagLib struct{}

func (TagLib) CanRead(absPath string) bool {
	switch ext := filepath.Ext(absPath); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

func (TagLib) Read(absPath string) (tagcommon.Info, error) {
	raw, props, err := audiotags.Read(absPath)
	return &info{raw, props}, err
}

type info struct {
	raw   map[string][]string
	props *audiotags.AudioProperties
}

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

func (i *info) Title() string          { return first(find(i.raw, "title")) }
func (i *info) Artist() string         { return first(find(i.raw, "artist")) }
func (i *info) Artists() []string      { return find(i.raw, "artists") }
func (i *info) Album() string          { return first(find(i.raw, "album")) }
func (i *info) AlbumArtist() string    { return first(find(i.raw, "albumartist", "album artist")) }
func (i *info) AlbumArtists() []string { return find(i.raw, "albumartists", "album_artists") }
func (i *info) Genre() string          { return first(find(i.raw, "genre")) }
func (i *info) Genres() []string       { return find(i.raw, "genres") }
func (i *info) TrackNumber() int       { return intSep("/", first(find(i.raw, "tracknumber"))) } // eg. 5/12
func (i *info) DiscNumber() int        { return intSep("/", first(find(i.raw, "discnumber"))) }  // eg. 1/2
func (i *info) Media() string          { return first(find(i.raw, "media")) }
func (i *info) Date() string           { return first(find(i.raw, "date", "year")) }
func (i *info) OriginalDate() string   { return first(find(i.raw, "originaldate", "date", "year")) }
func (i *info) Label() string          { return first(find(i.raw, "label")) }
func (i *info) CatalogueNum() string   { return first(find(i.raw, "catalognumber")) }

func (i *info) MBRecordingID() string     { return first(find(i.raw, "musicbrainz_trackid")) }
func (i *info) MBReleaseID() string       { return first(find(i.raw, "musicbrainz_albumid")) }
func (i *info) MBReleaseGroupID() string  { return first(find(i.raw, "musicbrainz_releasegroupid")) }
func (i *info) MBArtistID() []string      { return find(i.raw, "musicbrainz_artistid") }
func (i *info) MBAlbumArtistID() []string { return find(i.raw, "musicbrainz_albumartistid") }

func (i *info) Length() int  { return i.props.Length }
func (i *info) Bitrate() int { return i.props.Bitrate }

func (i *info) String() string {
	var buf strings.Builder
	for k, v := range i.raw {
		fmt.Fprintf(&buf, "%s\t%v\n", k, strings.Join(v, "; "))
	}
	return buf.String()
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
