package release

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

type Release struct {
	MBID             string
	ReleaseGroupMBID string
	ArtistCredit     string
	Artists          []Artist
	Title            string
	Date             time.Time
	OriginalDate     time.Time
	Label            string
	LabelMBID        string
	CatalogueNum     string
	MediaFormat      string
	Tracks           []Track
}

type Track struct {
	Number        int
	Title         string
	RecordingMBID string
	ArtistCredit  string
	Artists       []Artist
}

type Artist struct {
	MBID  string
	Title string
}

func FromMusicBrainz(mb *musicbrainz.ReleaseResponse) *Release {
	var r Release
	r.MBID = mb.ID
	r.ReleaseGroupMBID = mb.ID
	r.ArtistCredit = mbArtistCredit(mb.ArtistCredit)
	r.Artists = mapp(mb.ArtistCredit, func(i int, v musicbrainz.ArtistCredit) Artist {
		return Artist{
			MBID:  v.Artist.ID,
			Title: v.Artist.Name,
		}
	})
	r.Title = mb.Title

	r.Date, _ = dateparse.ParseAny(mb.Date)
	r.OriginalDate, _ = dateparse.ParseAny(mb.Date) // TODO: original

	if len(mb.LabelInfo) > 0 {
		r.CatalogueNum = mb.LabelInfo[0].CatalogNumber
		r.Label = mb.LabelInfo[0].Label.Name
		r.LabelMBID = mb.LabelInfo[0].Label.ID
	}

	if len(mb.Media) > 0 {
		r.MediaFormat = mb.Media[0].Format
	}

	var mbTracks []musicbrainz.Track
	for _, m := range mb.Media {
		mbTracks = append(mbTracks, m.Tracks...)
	}

	r.Tracks = mapp(mbTracks, func(i int, v musicbrainz.Track) Track {
		return Track{
			Number:        i + 1,
			Title:         v.Title,
			RecordingMBID: v.Recording.ID,
			ArtistCredit:  mbArtistCredit(v.ArtistCredit),
			Artists: mapp(v.ArtistCredit, func(i int, v musicbrainz.ArtistCredit) Artist {
				return Artist{
					MBID:  v.Artist.ID,
					Title: v.Artist.Name,
				}
			}),
		}
	})

	return &r
}

func FromTags(ti []tagcommon.File) *Release {
	if len(ti) == 0 {
		return nil
	}
	sort.SliceStable(ti, func(i, j int) bool {
		return ti[i].TrackNumber() < ti[j].TrackNumber()
	})

	t := ti[0]

	var r Release
	r.MBID = t.MBReleaseID()
	r.ReleaseGroupMBID = t.MBReleaseGroupID()
	r.ArtistCredit = t.AlbumArtist()

	r.Artists = mapp(t.AlbumArtists(), func(_ int, v string) Artist {
		return Artist{
			MBID:  "", // TODO
			Title: v,
		}
	})
	r.Title = t.Album()
	r.Date, _ = dateparse.ParseAny(t.Date())
	r.OriginalDate, _ = dateparse.ParseAny(t.OriginalDate())
	r.CatalogueNum = t.CatalogueNum()
	r.Label = t.Label()
	r.LabelMBID = "" // TODO
	r.MediaFormat = t.MediaFormat()

	r.Tracks = mapp(ti, func(i int, v tagcommon.File) Track {
		return Track{
			Number:        i + 1,
			Title:         v.Title(),
			RecordingMBID: v.MBRecordingID(),
			ArtistCredit:  v.Artist(),
			Artists: mapp(v.Artists(), func(_ int, v string) Artist {
				return Artist{
					MBID:  "",
					Title: v,
				}
			}),
		}
	})

	return &r
}

func ToTags(r *Release, ti []tagcommon.File) {
	for i, t := range r.Tracks {
		ti[i].WriteAlbum(r.Title)
		ti[i].WriteAlbumArtist(r.ArtistCredit)
		ti[i].WriteAlbumArtists(mapp(r.Artists, func(i int, v Artist) string {
			return v.Title
		}))
		ti[i].WriteDate(r.Date.Format(time.DateOnly))
		ti[i].WriteOriginalDate(r.OriginalDate.Format(time.DateOnly))
		ti[i].WriteMediaFormat(r.MediaFormat)
		ti[i].WriteLabel(r.Label)
		ti[i].WriteCatalogueNum(r.CatalogueNum)

		ti[i].WriteMBReleaseID(r.MBID)
		ti[i].WriteMBReleaseGroupID(r.ReleaseGroupMBID)
		ti[i].WriteMBAlbumArtistID(mapp(r.Artists, func(i int, v Artist) string {
			return v.MBID
		}))

		ti[i].WriteTitle(t.Title)
		ti[i].WriteArtist(t.ArtistCredit)
		ti[i].WriteArtists(mapp(t.Artists, func(i int, v Artist) string {
			return v.Title
		}))
		ti[i].WriteGenre("")
		ti[i].WriteGenres(nil)
		ti[i].WriteTrackNumber(i + 1)
		ti[i].WriteDiscNumber(1)

		ti[i].WriteMBRecordingID(t.RecordingMBID)
		ti[i].WriteMBArtistID(mapp(t.Artists, func(i int, v Artist) string {
			return v.MBID
		}))
	}
}

func mbArtistCredit(artists []musicbrainz.ArtistCredit) string {
	var sb strings.Builder
	for _, mba := range artists {
		fmt.Fprintf(&sb, "%s%s", mba.Name, mba.JoinPhrase)
	}
	return sb.String()
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}

func DiffScore(a, b *Release) int {
	return 0
}

func Diff(a, b *Release) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "score: %d\n", DiffScore(a, b))
	fmt.Fprintf(&buf, "release:\n")
	fmt.Fprintf(&buf, "  name      : %q -> %q\n", a.Title, b.Title)
	fmt.Fprintf(&buf, "  artist    : %q -> %q\n", a.ArtistCredit, b.ArtistCredit)
	fmt.Fprintf(&buf, "  label     : %q -> %q\n", a.Label, b.Label)
	fmt.Fprintf(&buf, "  catalogue : %q -> %q\n", a.CatalogueNum, b.CatalogueNum)
	fmt.Fprintf(&buf, "  media     : %q -> %q\n", a.MediaFormat, b.MediaFormat)
	fmt.Fprintf(&buf, "tracks:\n")
	for i := range a.Tracks {
		fmt.Fprintf(&buf, "  %02d  : %q %q\n     -> %q %q\n",
			i,
			a.Tracks[i].ArtistCredit, a.Tracks[i].Title,
			b.Tracks[i].ArtistCredit, b.Tracks[i].Title)
	}
	return buf.String()
}
