package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	R18DevAPIBase = "https://r18.dev/videos/vod/movies/detail/-/combined=%s/json"
	DefaultUA     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

// R18RawResponse maps the JSON payload from R18.dev.
type R18RawResponse struct {
	Status      int    `json:"status"`
	DVDID       string `json:"dvd_id"`
	ContentID   string `json:"content_id"`
	Title       string `json:"title"`
	TitleJA     string `json:"title_ja"`
	ReleaseDate string `json:"release_date"`
	RuntimeMins int    `json:"runtime_mins"`
	Maker       struct {
		Name string `json:"name"`
	} `json:"maker"`
	Label struct {
		Name string `json:"name"`
	} `json:"label"`
	Series struct {
		Name string `json:"name"`
	} `json:"series"`
	Directors []struct {
		Name string `json:"name"`
	} `json:"directors"`
	Actresses []struct {
		NameRomaji string `json:"name_romaji"`
		NameJA     string `json:"name_ja"`
		ImageURL   string `json:"image_url"`
	} `json:"actresses"`
	Categories []struct {
		Name   string `json:"name"`
		NameJA string `json:"name_ja"`
	} `json:"categories"`
	Images struct {
		JacketFull  string   `json:"jacket_full"`
		JacketFront string   `json:"jacket_front"`
		Gallery     []string `json:"gallery"`
	} `json:"images"`
	URL string `json:"url"`
}

// Client interacts with the R18.dev JSON API.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new R18.dev scraper client.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		userAgent: DefaultUA,
	}
}

// Scrape fetches metadata for a given JAV ID from R18.dev.
func (c *Client) Scrape(ctx context.Context, id string) (*Movie, error) {
	combinedID := NormalizeToCombinedID(id)
	if combinedID == "" {
		return nil, fmt.Errorf("invalid ID: %s", id)
	}

	apiURL := fmt.Sprintf(R18DevAPIBase, combinedID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://r18.dev/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("movie not found on R18.dev (404) for ID: %s (combined: %s)", id, combinedID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("R18.dev API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var raw R18RawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	movie := &Movie{
		ID:                raw.DVDID,
		CombinedID:        raw.ContentID,
		Title:             strings.TrimSpace(raw.Title),
		OriginalTitle:     strings.TrimSpace(raw.TitleJA),
		Maker:             strings.TrimSpace(raw.Maker.Name),
		Label:             strings.TrimSpace(raw.Label.Name),
		Series:            strings.TrimSpace(raw.Series.Name),
		ReleaseDate:       raw.ReleaseDate,
		RuntimeMinutes:    raw.RuntimeMins,
		CoverURL:          raw.Images.JacketFull,
		PosterURL:         raw.Images.JacketFront,
		SampleScreenshots: raw.Images.Gallery,
		DetailURL:         raw.URL,
		ScrapedAt:         time.Now(),
	}

	if movie.ID == "" {
		movie.ID = id
	}
	if movie.CombinedID == "" {
		movie.CombinedID = combinedID
	}
	if len(raw.Directors) > 0 {
		movie.Director = raw.Directors[0].Name
	}

	for _, act := range raw.Actresses {
		name := act.NameRomaji
		if name == "" {
			name = act.NameJA
		}
		movie.Actresses = append(movie.Actresses, Actress{
			Name:     name,
			JaName:   act.NameJA,
			ImageURL: act.ImageURL,
		})
	}

	for _, cat := range raw.Categories {
		name := cat.Name
		if name == "" {
			name = cat.NameJA
		}
		if name != "" {
			movie.Genres = append(movie.Genres, name)
		}
	}

	return movie, nil
}
