package torrent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// QBittorrentClient interacts with the qBittorrent Web API v2.
type QBittorrentClient struct {
	Config     QBittorrentConfig
	HTTPClient *http.Client
	mu         sync.RWMutex
	isLoggedIn bool
}

// NewQBittorrentClient creates a new qBittorrent Web API v2 client with automatic cookie management.
func NewQBittorrentClient(cfg QBittorrentConfig) *QBittorrentClient {
	rawURL := strings.TrimRight(cfg.URL, "/")
	if rawURL == "" {
		rawURL = "http://127.0.0.1:8080"
	}
	cfg.URL = rawURL

	if cfg.Category == "" {
		cfg.Category = "r19dev"
	}

	jar, _ := cookiejar.New(nil)
	return &QBittorrentClient{
		Config: cfg,
		HTTPClient: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		},
	}
}

// ClientType returns "qbittorrent".
func (c *QBittorrentClient) ClientType() string {
	return "qbittorrent"
}

// ensureAuth authenticates against qBittorrent Web API if needed.
func (c *QBittorrentClient) ensureAuth(ctx context.Context) error {
	c.mu.RLock()
	logged := c.isLoggedIn
	c.mu.RUnlock()

	if logged {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	loginURL := c.Config.URL + "/api/v2/auth/login"
	form := url.Values{}
	form.Set("username", c.Config.Username)
	form.Set("password", c.Config.Password)

	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.Config.URL)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to qBittorrent at %s: %w", c.Config.URL, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(bodyBytes))

	if resp.StatusCode == http.StatusForbidden || bodyStr == "Fails." {
		return fmt.Errorf("qbittorrent authentication failed (403 Forbidden): invalid username or password")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return fmt.Errorf("qbittorrent login error (status %d): %s", resp.StatusCode, bodyStr)
	}

	c.isLoggedIn = true
	return nil
}

// doRequest sends an HTTP request with automatic re-authentication if 403 Forbidden is received.
func (c *QBittorrentClient) doRequest(ctx context.Context, method string, apiPath string, body io.Reader, contentType string) (*http.Response, error) {
	if err := c.ensureAuth(ctx); err != nil && (c.Config.Username != "" || c.Config.Password != "") {
		// Try proceeding anyway in case WebUI is configured without auth on LAN
	}

	fullURL := c.Config.URL + apiPath
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("Referer", c.Config.URL)
		return req, nil
	}

	req, err := makeReq()
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent request to %s failed: %w", apiPath, err)
	}

	// If session expired or unauthorized, attempt re-login once
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.mu.Lock()
		c.isLoggedIn = false
		c.mu.Unlock()

		if err := c.ensureAuth(ctx); err != nil {
			return nil, err
		}

		req2, err := makeReq()
		if err != nil {
			return nil, err
		}
		return c.HTTPClient.Do(req2)
	}

	return resp, nil
}

// TestConnection verifies qBittorrent connectivity, credentials, and retrieves version & default save path.
func (c *QBittorrentClient) TestConnection(ctx context.Context) (version string, downloadDir string, err error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v2/app/version", nil, "")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("qbittorrent authentication failed (403 Forbidden): invalid username or password")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("qbittorrent returned status %d: %s", resp.StatusCode, string(b))
	}

	vBytes, _ := io.ReadAll(resp.Body)
	version = strings.TrimSpace(string(vBytes))

	// Fetch default save path
	respDir, err := c.doRequest(ctx, "GET", "/api/v2/app/defaultSavePath", nil, "")
	if err == nil && respDir.StatusCode == http.StatusOK {
		defer respDir.Body.Close()
		dBytes, _ := io.ReadAll(respDir.Body)
		downloadDir = strings.TrimSpace(string(dBytes))
	}

	return version, downloadDir, nil
}

// AddTorrent sends a magnet link or .torrent URL to qBittorrent.
func (c *QBittorrentClient) AddTorrent(ctx context.Context, torrentURLOrMagnet string, customDownloadDir string) (id int, hash string, name string, err error) {
	// Extract infoHash from magnet URL if available
	hash = extractInfoHashFromMagnet(torrentURLOrMagnet)

	form := url.Values{}
	form.Set("urls", torrentURLOrMagnet)
	form.Set("paused", "false")
	if c.Config.Category != "" {
		form.Set("category", c.Config.Category)
	}

	targetDir := customDownloadDir
	if targetDir == "" {
		targetDir = c.Config.DownloadDir
	}
	if targetDir != "" {
		form.Set("savepath", targetDir)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v2/torrents/add", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, "", "", fmt.Errorf("qbittorrent add torrent failed (status %d): %s", resp.StatusCode, string(b))
	}

	return 0, hash, "", nil
}

type qbTorrentItem struct {
	Hash         string  `json:"hash"`
	Name         string  `json:"name"`
	Size         int64   `json:"size"`
	TotalSize    int64   `json:"total_size"`
	Progress     float64 `json:"progress"` // 0.0 - 1.0
	DlSpeed      int64   `json:"dlspeed"`  // bytes/s
	UpSpeed      int64   `json:"upspeed"`  // bytes/s
	ETA          int64   `json:"eta"`      // seconds
	State        string  `json:"state"`
	SavePath     string  `json:"save_path"`
	ContentPath  string  `json:"content_path"`
	Downloaded   int64   `json:"completed"`
	AmountLeft   int64   `json:"amount_left"`
	AddedOn      int64   `json:"added_on"`
	CompletionOn int64   `json:"completion_on"`
}

// GetTorrents returns active and completed torrents formatted as TransmissionTorrent models.
func (c *QBittorrentClient) GetTorrents(ctx context.Context) ([]TransmissionTorrent, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v2/torrents/info?filter=all", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qbittorrent get torrents failed (status %d): %s", resp.StatusCode, string(b))
	}

	var qbList []qbTorrentItem
	if err := json.NewDecoder(resp.Body).Decode(&qbList); err != nil {
		return nil, fmt.Errorf("failed to parse qbittorrent torrents json: %w", err)
	}

	results := make([]TransmissionTorrent, 0, len(qbList))
	for _, item := range qbList {
		t := TransmissionTorrent{
			HashString:   item.Hash,
			Name:         item.Name,
			PercentDone:  item.Progress,
			RateDownload: item.DlSpeed,
			RateUpload:   item.UpSpeed,
			ETA:          item.ETA,
			DownloadDir:  item.SavePath,
			Downloaded:   item.Downloaded,
		}

		if item.TotalSize > 0 {
			t.TotalSize = item.TotalSize
		} else {
			t.TotalSize = item.Size
		}

		if t.ETA >= 8640000 || t.ETA < 0 {
			t.ETA = -1
		}

		// Map qBittorrent states to status codes & text
		switch item.State {
		case "uploading", "stalledUP", "queuedUP", "forcedUP":
			t.Status = 6
			t.StatusText = "Seeding"
			t.IsFinished = true
			if t.PercentDone < 1.0 {
				t.PercentDone = 1.0
			}
		case "pausedUP":
			t.Status = 0
			t.StatusText = "Paused (Finished)"
			t.IsFinished = true
			if t.PercentDone < 1.0 {
				t.PercentDone = 1.0
			}
		case "downloading", "forcedDL":
			t.Status = 4
			t.StatusText = "Downloading"
		case "stalledDL":
			t.Status = 4
			t.StatusText = "Stalled DL"
		case "metaDL":
			t.Status = 4
			t.StatusText = "Fetching Metadata"
		case "allocating":
			t.Status = 4
			t.StatusText = "Allocating Space"
		case "queuedDL":
			t.Status = 3
			t.StatusText = "Download Wait"
		case "pausedDL":
			t.Status = 0
			t.StatusText = "Paused"
		case "checkingDL", "checkingUP", "checkingResumeData":
			t.Status = 2
			t.StatusText = "Checking"
		case "error", "missingFiles":
			t.Status = -1
			t.StatusText = "Error"
			t.Error = 1
			t.ErrorString = fmt.Sprintf("qBittorrent error state: %s", item.State)
		default:
			t.Status = 4
			t.StatusText = item.State
		}

		if t.PercentDone >= 1.0 {
			t.IsFinished = true
		}

		results = append(results, t)
	}

	return results, nil
}

// RemoveTorrent deletes a torrent from qBittorrent by hash.
func (c *QBittorrentClient) RemoveTorrent(ctx context.Context, id int, hash string, deleteLocalData bool) error {
	if hash == "" {
		return fmt.Errorf("cannot remove torrent: hash is required for qbittorrent")
	}

	form := url.Values{}
	form.Set("hashes", hash)
	form.Set("deleteFiles", fmt.Sprintf("%t", deleteLocalData))

	resp, err := c.doRequest(ctx, "POST", "/api/v2/torrents/delete", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qbittorrent delete torrent failed (status %d): %s", resp.StatusCode, string(b))
	}

	return nil
}

// extractInfoHashFromMagnet parses the BTIH from a magnet URI string.
func extractInfoHashFromMagnet(magnetURI string) string {
	if !strings.HasPrefix(magnetURI, "magnet:?") {
		return ""
	}
	parts := strings.Split(magnetURI, "&")
	for _, p := range parts {
		if strings.HasPrefix(p, "xt=urn:btih:") || strings.HasPrefix(p, "magnet:?xt=urn:btih:") {
			hash := strings.TrimPrefix(p, "magnet:?xt=urn:btih:")
			hash = strings.TrimPrefix(hash, "xt=urn:btih:")
			return strings.ToLower(hash)
		}
	}
	return ""
}
