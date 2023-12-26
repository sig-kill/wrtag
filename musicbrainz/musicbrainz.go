package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const base = "https://musicbrainz.org/ws/2/"

var ErrNoResults = fmt.Errorf("no results")

type Client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
		limiter:    rate.NewLimiter(rate.Every(time.Second), 1),
	}
}

func (c *Client) request(ctx context.Context, r *http.Request, dest any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	resp, err := c.httpClient.Do(r.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("search returned non 2xx: %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode response")
	}
	return nil
}

func (c *Client) GetRelease(ctx context.Context, mbid string) (*ReleaseResponse, error) {
	urlV := url.Values{}
	urlV.Set("fmt", "json")
	urlV.Set("inc", "recordings+artist-credits+labels")

	url, _ := url.Parse(joinPath(base, "release", mbid))
	url.RawQuery = urlV.Encode()
	req, _ := http.NewRequest(http.MethodGet, url.String(), nil)

	var sr ReleaseResponse
	if err := c.request(ctx, req, &sr); err != nil {
		return nil, fmt.Errorf("request release: %w", err)
	}

	return &sr, nil
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

func (c *Client) SearchRelease(ctx context.Context, q Query) (int, *ReleaseResponse, error) {
	if q.MBReleaseID != "" {
		release, err := c.GetRelease(ctx, q.MBReleaseID)
		if err != nil {
			return 0, nil, fmt.Errorf("get direct release: %w", err)
		}
		return 100, release, nil
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
		return 0, nil, ErrNoResults
	}

	queryStr := strings.Join(params, " ")
	log.Printf("sending query %s", queryStr)

	urlV := url.Values{}
	urlV.Set("fmt", "json")
	urlV.Set("limit", "1")
	urlV.Set("query", queryStr)

	url, _ := url.Parse(joinPath(base, "release"))
	url.RawQuery = urlV.Encode()
	req, _ := http.NewRequest(http.MethodGet, url.String(), nil)

	var sr SearchResponse
	if err := c.request(ctx, req, &sr); err != nil {
		return 0, nil, fmt.Errorf("request release: %w", err)
	}
	if len(sr.Releases) == 0 || sr.Releases[0].ID == "" {
		return 0, nil, ErrNoResults
	}
	releaseKey := sr.Releases[0]

	release, err := c.GetRelease(ctx, releaseKey.ID)
	if err != nil {
		return 0, nil, fmt.Errorf("get release by mbid %s: %w", releaseKey.ID, err)
	}

	return releaseKey.Score, release, nil
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

type SearchResponse struct {
	Releases []struct {
		ID    string `json:"id"`
		Score int    `json:"score"`
	} `json:"releases"`
}

type ArtistCredit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		TypeID         string `json:"type-id"`
		SortName       string `json:"sort-name"`
		Type           string `json:"type"`
		Disambiguation string `json:"disambiguation"`
	} `json:"artist"`
}

type Track struct {
	ID        string `json:"id"`
	Length    int    `json:"length"`
	Recording struct {
		FirstReleaseDate string `json:"first-release-date"`
		Video            bool   `json:"video"`
		Disambiguation   string `json:"disambiguation"`
		ID               string `json:"id"`
		Length           int    `json:"length"`
		Title            string `json:"title"`
		ArtistCredit     []struct {
			Name   string `json:"name"`
			Artist struct {
				TypeID         string `json:"type-id"`
				Name           string `json:"name"`
				ID             string `json:"id"`
				Type           string `json:"type"`
				Disambiguation string `json:"disambiguation"`
				SortName       string `json:"sort-name"`
			} `json:"artist"`
			Joinphrase string `json:"joinphrase"`
		} `json:"artist-credit"`
	} `json:"recording"`
	Number       string         `json:"number"`
	Position     int            `json:"position"`
	Title        string         `json:"title"`
	ArtistCredit []ArtistCredit `json:"artist-credit"`
}

type ReleaseResponse struct {
	Title              string `json:"title"`
	ID                 string `json:"id"`
	TextRepresentation struct {
		Language string `json:"language"`
		Script   string `json:"script"`
	} `json:"text-representation"`
	StatusID        string `json:"status-id"`
	Asin            string `json:"asin"`
	Country         string `json:"country"`
	Barcode         string `json:"barcode"`
	Disambiguation  string `json:"disambiguation"`
	Packaging       string `json:"packaging"`
	CoverArtArchive struct {
		Artwork  bool `json:"artwork"`
		Front    bool `json:"front"`
		Darkened bool `json:"darkened"`
		Back     bool `json:"back"`
		Count    int  `json:"count"`
	} `json:"cover-art-archive"`
	ArtistCredit []ArtistCredit `json:"artist-credit"`
	Date         string         `json:"date"`
	Quality      string         `json:"quality"`
	Media        []struct {
		TrackOffset int     `json:"track-offset"`
		TrackCount  int     `json:"track-count"`
		Tracks      []Track `json:"tracks"`
		Format      string  `json:"format"`
		FormatID    string  `json:"format-id"`
		Title       string  `json:"title"`
		Position    int     `json:"position"`
	} `json:"media"`
	Status        string `json:"status"`
	ReleaseEvents []struct {
		Area struct {
			ID             string   `json:"id"`
			Name           string   `json:"name"`
			Iso31661Codes  []string `json:"iso-3166-1-codes"`
			TypeID         any      `json:"type-id"`
			SortName       string   `json:"sort-name"`
			Disambiguation string   `json:"disambiguation"`
			Type           any      `json:"type"`
		} `json:"area"`
		Date string `json:"date"`
	} `json:"release-events"`
	PackagingID string `json:"packaging-id"`
	LabelInfo   []struct {
		Label struct {
			LabelCode      any    `json:"label-code"`
			Type           string `json:"type"`
			Disambiguation string `json:"disambiguation"`
			SortName       string `json:"sort-name"`
			TypeID         string `json:"type-id"`
			ID             string `json:"id"`
			Name           string `json:"name"`
		} `json:"label"`
		CatalogNumber string `json:"catalog-number"`
	} `json:"label-info"`
}
