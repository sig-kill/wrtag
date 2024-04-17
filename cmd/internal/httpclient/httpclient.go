package httpclient

import (
	"net/http"
	"time"

	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/musicbrainz"
)

type MusicBrainz struct {
	*musicbrainz.MBClient
	*musicbrainz.CAAClient
}

func DefaultMusicBrainz() MusicBrainz {
	cache := clientutil.WithCache()
	logging := clientutil.WithLogging()
	userAgent := clientutil.WithUserAgent(`wrtag/v0.0.0-alpha ( https://go.senan.xyz/wrtag )`)

	// https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting
	mbClient := &http.Client{Transport: clientutil.Chain(cache, userAgent, clientutil.WithRateLimit(1*time.Second), logging)(http.DefaultTransport)}

	// https://wiki.musicbrainz.org/Cover_Art_Archive/API#Rate_limiting_rules
	caaClient := &http.Client{Transport: clientutil.Chain(cache, userAgent, logging)(http.DefaultTransport)}

	return MusicBrainz{
		MBClient:  musicbrainz.NewMBClient("https://musicbrainz.org/ws/2/", mbClient),
		CAAClient: musicbrainz.NewCAAClient("https://coverartarchive.org/", caaClient),
	}
}
