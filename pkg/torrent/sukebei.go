package torrent

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultSukebeiBaseURL is the default Sukebei Nyaa domain.
const DefaultSukebeiBaseURL = "https://sukebei.nyaa.si"

// SukebeiClient provides methods to search torrents and parse RSS feeds.
type SukebeiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewSukebeiClient creates a new Sukebei RSS client.
func NewSukebeiClient(baseURL string) *SukebeiClient {
	if baseURL == "" {
		baseURL = DefaultSukebeiBaseURL
	}
	return &SukebeiClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// XML Structs for Sukebei RSS Feed
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Guid        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Seeders     string `xml:"seeders"`
	Leechers    string `xml:"leechers"`
	Downloads   string `xml:"downloads"`
	InfoHash    string `xml:"infoHash"`
	Category    string `xml:"category"`
	CategoryId  string `xml:"categoryId"`
	Size        string `xml:"size"`
	Comments    string `xml:"comments"`
	Trusted     string `xml:"trusted"`
	Description string `xml:"description"`
}

// Search queries Sukebei Nyaa RSS and returns scored torrent items.
func (c *SukebeiClient) Search(ctx context.Context, query string) (*TorrentSearchResult, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return &TorrentSearchResult{Query: query, Items: []TorrentItem{}}, nil
	}

	// Build RSS URL: category 2_2 is Real Life - Videos, sorted by seeders descending
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}

	q := u.Query()
	q.Set("page", "rss")
	q.Set("q", cleanQuery)
	q.Set("c", "2_2") // Real Life - Videos
	q.Set("s", "seeders")
	q.Set("o", "desc")
	q.Set("f", "0")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request to %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sukebei returned status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(bodyBytes, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rss feed: %w", err)
	}

	var items []TorrentItem
	for _, raw := range feed.Channel.Items {
		item := c.parseAndScoreItem(raw, cleanQuery)
		items = append(items, item)
	}

	// Sort items by score descending, then by seeders descending
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].Seeders != items[j].Seeders {
			return items[i].Seeders > items[j].Seeders
		}
		return items[i].PubDate.After(items[j].PubDate)
	})

	var recommended *TorrentItem
	if len(items) > 0 && items[0].Score > 0 {
		items[0].IsRecommended = true
		rec := items[0]
		recommended = &rec
	}

	return &TorrentSearchResult{
		Query:       cleanQuery,
		Total:       len(items),
		Recommended: recommended,
		Items:       items,
	}, nil
}

var (
	re4K          = regexp.MustCompile(`(?i)(4k|2160p|uhd)`)
	re1080p       = regexp.MustCompile(`(?i)(1080p|fhd|fhdc)`)
	re720p        = regexp.MustCompile(`(?i)(720p|hd)`)
	reUncensored  = regexp.MustCompile(`(?i)(\[un\]|\bun\b|uncensored|無碼|无码|流出|leak|無修正)`)
	reSubtitles   = regexp.MustCompile(`(?i)(\[c\]|\[ch\]|\[uc\]|chinese|中文字幕|中字|字幕)`)
	reCompilation = regexp.MustCompile(`(?i)(collection|best\s*\d+|\d+in1|vol\.\s*\d+|pack|disc\s*\d+)`)
)

func (c *SukebeiClient) parseAndScoreItem(raw rssItem, targetQuery string) TorrentItem {
	seeders, _ := strconv.Atoi(raw.Seeders)
	leechers, _ := strconv.Atoi(raw.Leechers)
	downloads, _ := strconv.Atoi(raw.Downloads)
	sizeBytes := parseSize(raw.Size)

	var pubDate time.Time
	if raw.PubDate != "" {
		if t, err := time.Parse(time.RFC1123Z, raw.PubDate); err == nil {
			pubDate = t
		} else if t, err := time.Parse(time.RFC1123, raw.PubDate); err == nil {
			pubDate = t
		}
	}

	magnetURL := ""
	if raw.InfoHash != "" {
		magnetURL = fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", strings.ToLower(raw.InfoHash), url.QueryEscape(raw.Title))
	}

	title := raw.Title
	isTrusted := strings.EqualFold(raw.Trusted, "yes") || strings.EqualFold(raw.Trusted, "true")

	// Detect Resolution
	resolution := "SD"
	resScore := 10
	if re4K.MatchString(title) {
		resolution = "4K"
		resScore = 100
	} else if re1080p.MatchString(title) {
		resolution = "1080p"
		resScore = 80
	} else if re720p.MatchString(title) {
		resolution = "720p"
		resScore = 40
	}

	// Detect Uncensored
	isUncensored := reUncensored.MatchString(title)
	hasSubtitles := reSubtitles.MatchString(title)

	// Base score calculation
	score := resScore

	// Uncensored bonus
	if isUncensored {
		score += 50
	}

	// Trusted uploader bonus
	if isTrusted {
		score += 10
	}

	// ID Match verification
	normalizedTarget := strings.ToUpper(strings.ReplaceAll(targetQuery, "-", ""))
	normalizedTitle := strings.ToUpper(strings.ReplaceAll(title, "-", ""))
	if strings.Contains(normalizedTitle, normalizedTarget) {
		score += 40
	}

	// Penalize multi-movie compilation packs
	if reCompilation.MatchString(title) {
		score -= 40
	}

	// Size sanity check
	if sizeBytes > 0 {
		gb := float64(sizeBytes) / (1024 * 1024 * 1024)
		if resolution == "1080p" {
			if gb >= 4.0 && gb <= 11.0 {
				score += 20
			} else if gb < 2.0 {
				score -= 20 // Oversqueezed bitrate
			} else if gb > 30.0 {
				score -= 30 // Likely oversized archive
			}
		} else if resolution == "4K" {
			if gb >= 10.0 && gb <= 30.0 {
				score += 20
			}
		}
	}

	// Health / Seeders scoring
	if seeders >= 10 {
		score += 30
	} else if seeders >= 3 {
		score += 20
	} else if seeders >= 1 {
		score += 10
	} else {
		score -= 60 // No seeders
	}

	return TorrentItem{
		ID:            raw.Guid,
		Title:         title,
		Link:          raw.Link,
		ViewURL:       raw.Guid,
		InfoHash:      strings.ToLower(raw.InfoHash),
		MagnetURL:     magnetURL,
		SizeFormatted: raw.Size,
		SizeBytes:     sizeBytes,
		PubDate:       pubDate,
		Seeders:       seeders,
		Leechers:      leechers,
		Downloads:     downloads,
		Category:      raw.Category,
		IsTrusted:     isTrusted,
		Resolution:    resolution,
		IsUncensored:  isUncensored,
		HasSubtitles:  hasSubtitles,
		Score:         score,
		IsRecommended: false,
	}
}

func parseSize(sz string) int64 {
	sz = strings.TrimSpace(sz)
	if sz == "" {
		return 0
	}
	parts := strings.Fields(sz)
	if len(parts) < 2 {
		return 0
	}
	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	unit := strings.ToUpper(parts[1])
	switch {
	case strings.HasPrefix(unit, "T"):
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "G"):
		return int64(val * 1024 * 1024 * 1024)
	case strings.HasPrefix(unit, "M"):
		return int64(val * 1024 * 1024)
	case strings.HasPrefix(unit, "K"):
		return int64(val * 1024)
	case strings.HasPrefix(unit, "B"):
		return int64(val)
	}
	return 0
}
