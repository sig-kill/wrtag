package musicbrainz

import (
	"net/http"
	"strconv"

	"go.senan.xyz/wrtag/clientutil"
)

type StatusError int

func (se StatusError) Error() string {
	return strconv.Itoa(int(se))
}

func wrapClient(c *http.Client, mw clientutil.Middleware) *http.Client {
	if c == nil {
		c = &http.Client{}
	}
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	c.Transport = mw(c.Transport)
	return c
}
