package lyrics

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

var ErrLyricsNotFound = errors.New("lyrics not found")

var musixmatchBaseURL = `https://www.musixmatch.com/lyrics`
var musixmatchEsc = strings.NewReplacer(" ", "-")
var musixmatchSelectContent = cascadia.MustCompile(`h2[role="heading"][style="color:var(--mxm-contentSecondary);padding-bottom:24px"] + div`)

type Musixmatch struct {
	HTTPClient *http.Client
}

func (mm Musixmatch) Search(artist, song string) (string, error) {
	url, _ := url.Parse(musixmatchBaseURL)
	url = url.JoinPath(musixmatchEsc.Replace(artist))
	url = url.JoinPath(musixmatchEsc.Replace(song))

	req, _ := http.NewRequest(http.MethodGet, url.String(), nil)
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
