package tags

import (
	"iter"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"go.senan.xyz/taglib"
)

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
	case ".mp3", ".flac", ".opus", ".aac", ".aiff", ".ape", ".m4a", ".m4b", ".mp2", ".mpc", ".oga", ".ogg", ".spx", ".tak", ".wav", ".wma", ".wv":
		return true
	}
	return false
}

type Tags struct{ tags map[string][]string }

func ReadTags(path string) (Tags, error) {
	t, err := taglib.ReadTags(path)
	if err != nil {
		return Tags{}, err
	}

	normalise(t, alternatives)

	return Tags{t}, nil
}

func (t Tags) Read(k string) string        { return first(t.tags[k]) }
func (t Tags) ReadMulti(k string) []string { return t.tags[k] }

func (t Tags) ReadNum(k string) int       { return anyNum(first(t.tags[k])) }
func (t Tags) ReadFloat(k string) float64 { return anyFloat(first(t.tags[k])) }

func (t Tags) ReadTime(k string) time.Time { return anyTime(first(t.tags[k])) }

func (t Tags) Iter() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		for _, k := range slices.Sorted(maps.Keys(t.tags)) {
			if !yield(k, t.tags[k]) {
				break
			}
		}
	}
}

func (t Tags) Write(k string, v ...string) {
	v = slices.DeleteFunc(v, func(t string) bool {
		return t == ""
	})
	if len(v) == 0 {
		delete(t.tags, k)
		return
	}
	t.tags[k] = v
}
func (t Tags) WriteNum(k string, v int)       { t.Write(k, fmtInt(v)) }
func (t Tags) WriteFloat(k string, v float64) { t.Write(k, fmtFloat(v, 6)) }

func (t Tags) Clear(k string) { delete(t.tags, k) }
func (t Tags) ClearAll()      { clear(t.tags) }
func (t Tags) ClearUnknown() {
	for k := range t.tags {
		if _, ok := knownTags[k]; !ok {
			delete(t.tags, k)
		}
	}
}

func ReadProperties(path string) (taglib.Properties, error) {
	return taglib.ReadProperties(path)
}

func WriteTags(path string, tags Tags) error {
	return taglib.WriteTags(path, tags.tags)
}

func Clone(t Tags) Tags    { return Tags{tags: maps.Clone(t.tags)} }
func Equal(a, b Tags) bool { return maps.EqualFunc(a.tags, b.tags, slices.Equal) }

func UpdateTags(path string, f func(t Tags)) error {
	t, err := ReadTags(path)
	if err != nil {
		return err
	}

	before := Clone(t)
	f(t)

	if Equal(t, before) {
		return nil
	}

	if err := WriteTags(path, t); err != nil {
		return err
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
