package musicbrainz

import (
	"net/http"
	"strconv"
	"time"

	"go.senan.xyz/wrtag/clientutil"
)

const userAgent = `wrtag/v0.0.0-alpha ( https://go.senan.xyz/wrtag )`

type Client struct {
	*MBClient
	*CAAClient
}

func DefaultClient() Client {
	cache := clientutil.WithCache()
	logging := clientutil.WithLogging()

	// https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting
	mbClient := &http.Client{Transport: clientutil.Chain(cache, clientutil.WithRateLimit(1*time.Second), logging)(http.DefaultTransport)}

	// https://wiki.musicbrainz.org/Cover_Art_Archive/API#Rate_limiting_rules
	caaClient := &http.Client{Transport: clientutil.Chain(cache, logging)(http.DefaultTransport)}

	return Client{
		MBClient:  NewMBClient("https://musicbrainz.org/ws/2/", mbClient),
		CAAClient: NewCAAClient("https://coverartarchive.org/", caaClient),
	}
}

type StatusError int

func (se StatusError) Error() string {
	return strconv.Itoa(int(se))
}
