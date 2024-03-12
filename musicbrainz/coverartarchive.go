package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

var caaClient = http.Client{}

func caaRequest(ctx context.Context, r *http.Request, dest any) error {
	r = r.WithContext(ctx)
	resp, err := caaClient.Do(r)
	if err != nil {
		return fmt.Errorf("make caa request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("non 2xx from caa")
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode caa response: %w", err)
	}
	return nil
}

func GetCoverURL(ctx context.Context, release *Release) (string, error) {
	// try release first
	if release.CoverArtArchive.Front {
		req, _ := http.NewRequest(http.MethodGet, joinPath(caaBase, "release", release.ID), nil)
		var caa caaResponse
		if err := caaRequest(ctx, req, &caa); err != nil {
			return "", fmt.Errorf("make caa release request: %w", err)
		}
		for _, img := range caa.Images {
			if img.Front {
				return img.Image, nil
			}
		}
	}

	// otherwise fallback to release group
	req, _ := http.NewRequest(http.MethodGet, joinPath(caaBase, "release-group", release.ReleaseGroup.ID), nil)
	var caa caaResponse
	if err := caaRequest(ctx, req, &caa); err != nil {
		return "", fmt.Errorf("make caa release group request: %w", err)
	}
	for _, img := range caa.Images {
		if img.Front {
			return img.Image, nil
		}
	}
	return "", nil
}

type caaResponse struct {
	Release string `json:"release"`
	Images  []struct {
		Approved   bool     `json:"approved"`
		Back       bool     `json:"back"`
		Comment    string   `json:"comment"`
		Edit       int      `json:"edit"`
		Front      bool     `json:"front"`
		ID         any      `json:"id"`
		Image      string   `json:"image"`
		Types      []string `json:"types"`
		Thumbnails struct {
			Num250  string `json:"250"`
			Num500  string `json:"500"`
			Num1200 string `json:"1200"`
			Large   string `json:"large"`
			Small   string `json:"small"`
		} `json:"thumbnails"`
	} `json:"images"`
}
