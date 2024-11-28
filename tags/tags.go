package tags

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/araddon/dateparse"
	"go.senan.xyz/taglib-wasm"
)

var ErrWrite = errors.New("error writing tags")

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

//go:generate go run gen_taglist.go -- $GOFILE taglist.gen.go
const (
	Album              = "album"
	AlbumArtist        = "albumartist"         //tag: alts "album_artist"
	AlbumArtists       = "albumartists"        //tag: alts "album_artists"
	AlbumArtistCredit  = "albumartist_credit"  //tag: alts "album_artist_credit"
	AlbumArtistsCredit = "albumartists_credit" //tag: alts "album_artists_credit"
	Date               = "date"                //tag: alts "year"
	OriginalDate       = "originaldate"        //tag: alts "original_year"
	MediaFormat        = "media"
	Label              = "label"
	CatalogueNum       = "catalognumber" //tag: alts "catalognum"
	UPC                = "upc"           //tag: alts "mcn"
	Compilation        = "compilation"

	MBReleaseID      = "musicbrainz_albumid"
	MBReleaseGroupID = "musicbrainz_releasegroupid"
	MBAlbumArtistID  = "musicbrainz_albumartistid"
	MBAlbumComment   = "musicbrainz_albumcomment"

	Title         = "title"
	Artist        = "artist"
	Artists       = "artists"
	ArtistCredit  = "artist_credit"  //tag: alts "artistcredit"
	ArtistsCredit = "artists_credit" //tag: alts "artistscredit"
	Genre         = "genre"
	Genres        = "genres"
	TrackNumber   = "tracknumber" //tag: alts "track" "trackc"
	DiscNumber    = "discnumber"

	MBRecordingID = "musicbrainz_trackid"
	MBArtistID    = "musicbrainz_artistid"

	ReplayGainTrackGain = "replaygain_track_gain"
	ReplayGainTrackPeak = "replaygain_track_peak"
	ReplayGainAlbumGain = "replaygain_album_gain"
	ReplayGainAlbumPeak = "replaygain_album_peak"

	Lyrics = "lyrics" //tag: alts "lyrics:description" "uslt:description" "©lyr"
)

func CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

type File struct {
	tags       map[string][]string
	properties func() taglib.Properties
	path       string
}

func Read(path string) (*File, error) {
	tags, err := taglib.ReadTags(path)
	if err != nil {
		return nil, err
	}

	normalise(tags, alternatives)

	properties := sync.OnceValue(func() taglib.Properties {
		p, _ := taglib.ReadProperties(path)
		return p
	})

	return &File{
		tags:       tags,
		properties: properties,
		path:       path,
	}, nil
}

func (f *File) Read(t string) string        { return first(f.tags[t]) }
func (f *File) ReadMulti(t string) []string { return f.tags[t] }

func (f *File) ReadNum(t string) int       { return anyNum(first(f.tags[t])) }
func (f *File) ReadFloat(t string) float64 { return anyFloat(first(f.tags[t])) }

func (f *File) ReadTime(t string) time.Time { return anyTime(first(f.tags[t])) }

func (f *File) Iter() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		for _, k := range slices.Sorted(maps.Keys(f.tags)) {
			if !yield(k, f.tags[k]) {
				break
			}
		}
	}
}

func (f *File) Write(t string, v ...string) {
	v = slices.DeleteFunc(v, func(t string) bool {
		return t == ""
	})
	if len(v) == 0 {
		delete(f.tags, t)
		return
	}
	f.tags[t] = v
}
func (f *File) WriteNum(t string, v int)       { f.Write(t, fmtInt(v)) }
func (f *File) WriteFloat(t string, v float64) { f.Write(t, fmtFloat(v, 6)) }

func (f *File) Clear(t string) { delete(f.tags, t) }
func (f *File) ClearAll()      { clear(f.tags) }
func (f *File) ClearUnknown() {
	for k := range f.tags {
		if _, ok := knownTags[k]; !ok {
			delete(f.tags, k)
		}
	}
}

func (f *File) Length() time.Duration { return f.properties().Length }
func (f *File) Bitrate() uint         { return f.properties().Bitrate }
func (f *File) SampleRate() uint      { return f.properties().SampleRate }
func (f *File) NumChannels() uint     { return f.properties().Channels }

func (f *File) Save() error {
	return taglib.WriteTags(f.path, f.tags)
}

func (f *File) Path() string { return f.path }

func Write(path string, fn func(f *File) error) error {
	f, err := Read(path)
	if err != nil {
		return fmt.Errorf("read tag file: %w", err)
	}

	before := maps.Clone(f.tags)
	if err := fn(f); err != nil {
		return err
	}

	// try avoid filesystem writes if we can
	if maps.EqualFunc(before, f.tags, slices.Equal) {
		return nil
	}

	if lvl, l := slog.LevelDebug, slog.Default(); l.Enabled(context.Background(), lvl) {
		pathBase := filepath.Base(path)
		for k := range f.tags {
			if before, after := before[k], f.tags[k]; !slices.Equal(before, after) {
				l.Log(context.Background(), lvl, "tag change", "file", pathBase, "key", k, "from", before, "to", after)
			}
		}
	}

	if err := f.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func first(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func fmtInt(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func fmtFloat(v float64, p int) string {
	return strconv.FormatFloat(v, 'f', p, 64)
}

var numExpr = regexp.MustCompile(`\d+`)

func anyNum(in string) int {
	match := numExpr.FindString(in)
	i, _ := strconv.Atoi(match)
	return i
}

var floatExpr = regexp.MustCompile(`[+-]?([0-9]*[.])?[0-9]+`)

func anyFloat(in string) float64 {
	match := floatExpr.FindString(in)
	i, _ := strconv.ParseFloat(match, 64)
	return i
}

func anyTime(str string) time.Time {
	t, _ := dateparse.ParseAny(str)
	return t
}

func normalise(raw map[string][]string, alternatives map[string]string) {
	for k, v := range raw {
		if lk := strings.ToLower(k); lk != k {
			delete(raw, k)
			raw[lk] = v
		}
	}

	for kbad, kgood := range alternatives {
		if _, ok := raw[kgood]; ok {
			continue
		}
		if v, ok := raw[kbad]; ok {
			raw[kgood] = v
			delete(raw, kbad)
			continue
		}
	}
}
