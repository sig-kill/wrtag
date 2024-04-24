package lyrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"go.senan.xyz/wrtag/clientutil"
	"golang.org/x/net/html"
)

var ErrLyricsNotFound = errors.New("lyrics not found")

var musixmatchBaseURL = `https://www.musixmatch.com/lyrics`
var musixmatchEsc = strings.NewReplacer(" ", "-")
var musixmatchSelectContent = cascadia.MustCompile(`h2[role="heading"][style="color:var(--mxm-contentSecondary);padding-bottom:24px"] + div`)
var musixmatchIgnore = []string{"Still no lyrics here"}

type Source interface {
	Search(ctx context.Context, artist, song string) (string, error)
}

type Musixmatch struct {
	RateLimit time.Duration

	initOnce   sync.Once
	HTTPClient *http.Client
}

func (mm *Musixmatch) Search(ctx context.Context, artist, song string) (string, error) {
	mm.initOnce.Do(func() {
		mm.HTTPClient = clientutil.WrapClient(mm.HTTPClient, clientutil.Chain(
			clientutil.WithRateLimit(mm.RateLimit),
		))
	})

	url, _ := url.Parse(musixmatchBaseURL)
	url = url.JoinPath(musixmatchEsc.Replace(artist))
	url = url.JoinPath(musixmatchEsc.Replace(song))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	resp, err := mm.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("req page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", ErrLyricsNotFound
	}

	node, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse page: %w", err)
	}

	var out strings.Builder
	iterText(cascadia.Query(node, musixmatchSelectContent), func(s string) {
		out.WriteString(s + "\n")
	})
	for _, ig := range musixmatchIgnore {
		if strings.Contains(out.String(), ig) {
			return "", nil
		}
	}
	return out.String(), nil
}

func iterText(n *html.Node, f func(string)) {
	if n == nil {
		return
	}
	if n.Type == html.TextNode {
		f(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		iterText(c, f)
	}
}
