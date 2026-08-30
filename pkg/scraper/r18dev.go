package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	R18DevAPIBase = "https://r18.dev/videos/vod/movies/detail/-/combined=%s/json"
	DefaultUA     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

// R18RawResponse maps the complete modern JSON payload from R18.dev API.
type R18RawResponse struct {
	Status         int    `json:"status"`
	DVDID          string `json:"dvd_id"`
	ContentID      string `json:"content_id"`
	TitleEN        string `json:"title_en"`
	TitleJA        string `json:"title_ja"`
	ReleaseDate    string `json:"release_date"`
	RuntimeMins    int    `json:"runtime_mins"`
	MakerNameEN    string `json:"maker_name_en"`
	MakerNameJA    string `json:"maker_name_ja"`
	LabelNameEN    string `json:"label_name_en"`
	LabelNameJA    string `json:"label_name_ja"`
	JacketFullURL  string `json:"jacket_full_url"`
	JacketThumbURL string `json:"jacket_thumb_url"`
	SampleURL      string `json:"sample_url"`

	Directors []struct {
		ID         int    `json:"id"`
		NameRomaji string `json:"name_romaji"`
		NameKanji  string `json:"name_kanji"`
	} `json:"directors"`

	Actresses []struct {
		ID         int    `json:"id"`
		NameRomaji string `json:"name_romaji"`
		NameKanji  string `json:"name_kanji"`
		ImageURL   string `json:"image_url"`
	} `json:"actresses"`

	Categories []struct {
		ID     int    `json:"id"`
		NameEN string `json:"name_en"`
		NameJA string `json:"name_ja"`
	} `json:"categories"`

	Gallery []struct {
		ImageFull  string `json:"image_full"`
		ImageThumb string `json:"image_thumb"`
	} `json:"gallery"`

	URL string `json:"url"`
}

// Client interacts with the R18.dev JSON API.
type Client struct {
	httpClient *http.Client
	userAgent  string
	language   string // "en" or "ja"
	mu         sync.Mutex
	lastReq    time.Time
}

// NewClient creates a new R18.dev scraper client with language support.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		userAgent: DefaultUA,
		language:  "en", // Default to English metadata
	}
}

// SetLanguage sets the metadata language preference ("en" or "ja").
func (c *Client) SetLanguage(lang string) {
	if strings.ToLower(lang) == "ja" {
		c.language = "ja"
	} else {
		c.language = "en"
	}
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	minInterval := 250 * time.Millisecond
	elapsed := time.Since(c.lastReq)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	c.lastReq = time.Now()
}

// Scrape fetches metadata for a given JAV ID from R18.dev in preferred language (default English).
func (c *Client) Scrape(ctx context.Context, id string) (*Movie, error) {
	combinedID := NormalizeToCombinedID(id)
	if combinedID == "" {
		return nil, fmt.Errorf("invalid ID: %s", id)
	}

	apiURL := fmt.Sprintf(R18DevAPIBase, combinedID)
	var resp *http.Response
	var err error

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		c.throttle()

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}

		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Referer", "https://r18.dev/")
		if c.language == "ja" {
			req.Header.Set("Accept-Language", "ja,en-US;q=0.8,en;q=0.6")
		} else {
			req.Header.Set("Accept-Language", "en-US,en;q=0.9,ja;q=0.8")
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}

		// Handle Cloudflare / API rate limit with backoff retry
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if attempt < maxRetries-1 {
				backoff := time.Duration(600*(1<<attempt)) * time.Millisecond
				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, fmt.Errorf("R18.dev rate limit reached (HTTP 429). Retrying in a moment...")
		}

		break
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

	// Resolve title based on language preference
	title := strings.TrimSpace(raw.TitleEN)
	if c.language == "ja" || title == "" {
		if raw.TitleJA != "" {
			title = strings.TrimSpace(raw.TitleJA)
		}
	}

	// Resolve Maker / Studio
	maker := strings.TrimSpace(raw.MakerNameEN)
	if c.language == "ja" || maker == "" {
		if raw.MakerNameJA != "" {
			maker = strings.TrimSpace(raw.MakerNameJA)
		}
	}

	// Resolve Label
	label := strings.TrimSpace(raw.LabelNameEN)
	if c.language == "ja" || label == "" {
		if raw.LabelNameJA != "" {
			label = strings.TrimSpace(raw.LabelNameJA)
		}
	}

	// Resolve Director
	director := ""
	if len(raw.Directors) > 0 {
		d := raw.Directors[0]
		if c.language == "ja" && d.NameKanji != "" {
			director = d.NameKanji
		} else if d.NameRomaji != "" {
			director = d.NameRomaji
		} else {
			director = d.NameKanji
		}
	}

	movie := &Movie{
		ID:             raw.DVDID,
		CombinedID:     raw.ContentID,
		Title:          title,
		OriginalTitle:  strings.TrimSpace(raw.TitleJA),
		Maker:          maker,
		Label:          label,
		Director:       director,
		ReleaseDate:    raw.ReleaseDate,
		RuntimeMinutes: raw.RuntimeMins,
		CoverURL:       raw.JacketFullURL,
		PosterURL:      raw.JacketThumbURL,
		TrailerURL:     raw.SampleURL,
		DetailURL:      raw.URL,
		ScrapedAt:      time.Now(),
	}

	if movie.ID == "" {
		movie.ID = id
	}
	if movie.CombinedID == "" {
		movie.CombinedID = combinedID
	}

	// Populate Actresses (Romaji in English mode, Kanji in Japanese mode)
	for _, act := range raw.Actresses {
		name := act.NameRomaji
		if c.language == "ja" && act.NameKanji != "" {
			name = act.NameKanji
		}
		if name == "" {
			name = act.NameKanji
		}
		thumb := act.ImageURL
		if thumb != "" && !strings.HasPrefix(thumb, "http") {
			thumb = "https://pics.dmm.co.jp/mono/actjpgs/" + thumb
		}
		movie.Actresses = append(movie.Actresses, Actress{
			Name:     name,
			JaName:   act.NameKanji,
			ImageURL: thumb,
		})
	}

	// Populate Genres (English by default, e.g. "Female Teacher", "Big Tits", "Hi-Def")
	for _, cat := range raw.Categories {
		name := strings.TrimSpace(cat.NameEN)
		if c.language == "ja" || name == "" {
			if cat.NameJA != "" {
				name = strings.TrimSpace(cat.NameJA)
			}
		}
		if name != "" {
			movie.Genres = append(movie.Genres, name)
		}
	}

	// Populate Sample Gallery Screenshots
	for _, item := range raw.Gallery {
		if item.ImageFull != "" {
			movie.SampleScreenshots = append(movie.SampleScreenshots, item.ImageFull)
		}
	}

	return movie, nil
}
