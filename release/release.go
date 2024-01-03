package release

import (
	"fmt"
	"strings"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/tagcommon"
)

type Release struct {
	MBID         string
	ArtistCredit string
	Artists      []Artist
	Title        string
	Label        string
	LabelMBID    string
	Catalogue    string
	MediaFormat  string
	Tracks       []Track
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
	r.ArtistCredit = mbArtistCredit(mb.ArtistCredit)
	r.Artists = mbArtists(mb.ArtistCredit)
	r.Title = mb.Title

	if len(mb.LabelInfo) > 0 {
		r.Catalogue = mb.LabelInfo[0].CatalogNumber
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

	var tracks []Track
	for i, mbt := range mbTracks {
		tracks = append(tracks, Track{
			Number:        i,
			Title:         mbt.Title,
			RecordingMBID: mbt.Recording.ID,
			ArtistCredit:  mbArtistCredit(mbt.ArtistCredit),
			Artists:       mbArtists(mbt.ArtistCredit),
		})
	}
	r.Tracks = tracks

	return &r
}

func FromTagInfo(ti []tagcommon.Info) *Release {
	if len(ti) == 0 {
		return nil
	}
	t := ti[0]

	var r Release
	r.MBID = t.MBReleaseID()
	r.ArtistCredit = t.AlbumArtist()

	var artists []Artist
	for _, ta := range t.AlbumArtists() {
		artists = append(artists, Artist{
			MBID:  "", // TODO
			Title: ta,
		})
	}

	r.Artists = artists
	r.Title = t.Album()
	r.Catalogue = t.CatalogueNum()
	r.Label = t.Label()
	r.LabelMBID = "" // TODO
	r.MediaFormat = t.MediaFormat()

	var tracks []Track
	for i, t := range ti {
		tracks = append(tracks, Track{
			Number:        i,
			Title:         t.Title(),
			RecordingMBID: t.MBRecordingID(),
			ArtistCredit:  t.Artist(),
			Artists:       tagTrackArtists(t),
		})
	}
	r.Tracks = tracks

	return &r
}

func mbArtistCredit(artists []musicbrainz.ArtistCredit) string {
	var sb strings.Builder
	for _, mba := range artists {
		fmt.Fprintf(&sb, "%s%s", mba.Name, mba.JoinPhrase)
	}
	return sb.String()
}

func mbArtists(mbArtists []musicbrainz.ArtistCredit) []Artist {
	var artists []Artist
	for _, mba := range mbArtists {
		artists = append(artists, Artist{
			MBID:  mba.Artist.ID,
			Title: mba.Artist.Name,
		})
	}
	return artists
}

func tagTrackArtists(t tagcommon.Info) []Artist {
	var artists []Artist
	for _, ta := range t.Artists() {
		artists = append(artists, Artist{
			MBID:  "", // TODO
			Title: ta,
		})
	}
	return artists
}

func Diff(a, b *Release) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "score: %d\n", 0)
	fmt.Fprintf(&buf, "release:\n")
	fmt.Fprintf(&buf, "  name      : %q -> %q\n", a.Title, b.Title)
	fmt.Fprintf(&buf, "  artist    : %q -> %q\n", a.ArtistCredit, b.ArtistCredit)
	fmt.Fprintf(&buf, "  label     : %q -> %q\n", a.Label, b.Label)
	fmt.Fprintf(&buf, "  catalogue : %q -> %q\n", a.Catalogue, b.Catalogue)
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
