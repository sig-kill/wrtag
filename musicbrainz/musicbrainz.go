package musicbrainz

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const base = "https://musicbrainz.org/ws/2/"

var ErrNoResults = fmt.Errorf("no results")

type Release struct{}

type Client struct{}

func (c *Client) GetRelease(mbid string) (Release, error) {
	urlV := url.Values{}
	urlV.Set("fmt", "json")
	urlV.Set("inc", "recordings")

	url, _ := url.Parse(joinPath(base, "release", mbid))
	url.RawQuery = urlV.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return Release{}, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return Release{}, fmt.Errorf("search returned non 2xx: %d", resp.StatusCode)
	}

	a, _ := httputil.DumpResponse(resp, true)
	fmt.Println(string(a))

	return Release{}, nil
}

type Query struct {
	MBReleaseID      string
	MBArtistID       string
	MBReleaseGroupID string

	Release      string
	Artist       string
	Date         string
	Format       string
	Label        string
	CatalogueNum string
	NumTracks    int
}

func (c *Client) SearchRelease(q Query) (Release, error) {
	if q.MBReleaseID != "" {
		return c.GetRelease(q.MBReleaseID)
	}

	// https://beta.musicbrainz.org/doc/MusicBrainz_API/Search#Release

	var params []string
	if q.MBArtistID != "" {
		params = append(params, field("arid", q.MBArtistID))
	}
	if q.MBReleaseGroupID != "" {
		params = append(params, field("rgid", q.MBReleaseGroupID))
	}
	if q.Release != "" {
		params = append(params, field("release", q.Release))
	}
	if q.Artist != "" {
		params = append(params, field("artist", q.Artist))
	}
	if q.Date != "" {
		params = append(params, field("date", q.Date))
	}
	if q.Format != "" {
		params = append(params, field("format", q.Format))
	}
	if q.Label != "" {
		params = append(params, field("label", q.Label))
	}
	if q.CatalogueNum != "" {
		params = append(params, field("catno", q.CatalogueNum))
	}
	if q.NumTracks > 0 {
		params = append(params, field("tracks", q.NumTracks))
	}
	if len(params) == 0 {
		return Release{}, ErrNoResults
	}

	queryStr := strings.Join(params, " ")
	log.Printf("sending query %s", queryStr)

	urlV := url.Values{}
	urlV.Set("fmt", "json")
	urlV.Set("limit", "1")
	urlV.Set("query", queryStr)

	url, _ := url.Parse(joinPath(base, "release"))
	url.RawQuery = urlV.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return Release{}, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return Release{}, fmt.Errorf("search returned non 2xx: %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return Release{}, fmt.Errorf("decode response")
	}
	if len(sr.Releases) == 0 || sr.Releases[0].ID == "" {
		return Release{}, ErrNoResults
	}

	// TODO: check score
	release, err := c.GetRelease(sr.Releases[0].ID)
	if err != nil {
		return Release{}, fmt.Errorf("get release by mbid %s: %w", sr.Releases[0].ID, err)
	}

	return release, nil
}

type searchResponse struct {
	Releases []struct {
		ID    string `json:"id"`
		Score int    `json:"score"`
	} `json:"releases"`
}

func field(k string, v any) string {
	switch v.(type) {
	case int:
		return fmt.Sprintf("%s:%d", k, v)
	default:
		return fmt.Sprintf("%s:%q", k, v)
	}
}

func joinPath(base string, p ...string) string {
	r, _ := url.JoinPath(base, p...)
	return r
}
