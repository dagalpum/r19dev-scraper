package torrent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TransmissionClient provides methods to interact with Transmission via JSON-RPC.
type TransmissionClient struct {
	Config     TransmissionConfig
	HTTPClient *http.Client
	sessionID  string
	mu         sync.RWMutex
}

// NewTransmissionClient creates a new Transmission RPC client.
func NewTransmissionClient(cfg TransmissionConfig) *TransmissionClient {
	url := strings.TrimRight(cfg.URL, "/")
	if url == "" {
		url = "http://127.0.0.1:9091"
	}
	if !strings.HasSuffix(url, "/transmission/rpc") && !strings.Contains(url, "/rpc") {
		url = url + "/transmission/rpc"
	}
	cfg.URL = url

	return &TransmissionClient{
		Config: cfg,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type rpcRequest struct {
	Method    string                 `json:"method"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Tag       int                    `json:"tag,omitempty"`
}

type rpcResponse struct {
	Result    string                 `json:"result"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Tag       int                    `json:"tag,omitempty"`
}

func (c *TransmissionClient) getSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

func (c *TransmissionClient) setSessionID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = id
}

func (c *TransmissionClient) doRPC(ctx context.Context, method string, args map[string]interface{}) (*rpcResponse, error) {
	reqBody, err := json.Marshal(rpcRequest{
		Method:    method,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rpc request: %w", err)
	}

	exec := func(sessID string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", c.Config.URL, bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if sessID != "" {
			req.Header.Set("X-Transmission-Session-Id", sessID)
		}
		if c.Config.Username != "" || c.Config.Password != "" {
			auth := base64.StdEncoding.EncodeToString([]byte(c.Config.Username + ":" + c.Config.Password))
			req.Header.Set("Authorization", "Basic "+auth)
		}
		return c.HTTPClient.Do(req)
	}

	resp, err := exec(c.getSessionID())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Transmission at %s: %w", c.Config.URL, err)
	}
	defer resp.Body.Close()

	// Handle CSRF 409 Conflict handshake
	if resp.StatusCode == http.StatusConflict {
		newSessID := resp.Header.Get("X-Transmission-Session-Id")
		if newSessID == "" {
			return nil, fmt.Errorf("transmission returned 409 conflict but no X-Transmission-Session-Id header")
		}
		c.setSessionID(newSessID)

		// Retry with updated session ID
		resp2, err := exec(newSessID)
		if err != nil {
			return nil, fmt.Errorf("failed to retry rpc request with new session id: %w", err)
		}
		defer resp2.Body.Close()
		resp = resp2
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("transmission authentication failed (401 Unauthorized): invalid username or password")
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transmission rpc error (status %d): %s", resp.StatusCode, string(b))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transmission response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse transmission json response: %w", err)
	}

	if rpcResp.Result != "success" {
		return nil, fmt.Errorf("transmission rpc returned error: %s", rpcResp.Result)
	}

	return &rpcResp, nil
}

// TestConnection verifies Transmission connectivity, credentials, and returns version string.
func (c *TransmissionClient) TestConnection(ctx context.Context) (version string, downloadDir string, err error) {
	resp, err := c.doRPC(ctx, "session-get", nil)
	if err != nil {
		return "", "", err
	}
	if v, ok := resp.Arguments["version"].(string); ok {
		version = v
	}
	if dir, ok := resp.Arguments["download-dir"].(string); ok {
		downloadDir = dir
	}
	return version, downloadDir, nil
}

// AddTorrent sends a magnet link or .torrent URL to Transmission.
func (c *TransmissionClient) AddTorrent(ctx context.Context, torrentURLOrMagnet string, customDownloadDir string) (id int, hash string, name string, err error) {
	args := map[string]interface{}{
		"filename": torrentURLOrMagnet,
		"paused":   false,
	}
	if customDownloadDir != "" {
		args["download-dir"] = customDownloadDir
	} else if c.Config.DownloadDir != "" {
		args["download-dir"] = c.Config.DownloadDir
	}

	resp, err := c.doRPC(ctx, "torrent-add", args)
	if err != nil {
		return 0, "", "", err
	}

	// Response structure: arguments: { "torrent-added": { "id": 1, "name": "...", "hashString": "..." } }
	// Or "torrent-duplicate": { ... }
	var torMap map[string]interface{}
	if added, ok := resp.Arguments["torrent-added"].(map[string]interface{}); ok {
		torMap = added
	} else if dup, ok := resp.Arguments["torrent-duplicate"].(map[string]interface{}); ok {
		torMap = dup
	}

	if torMap != nil {
		if tid, ok := torMap["id"].(float64); ok {
			id = int(tid)
		}
		if h, ok := torMap["hashString"].(string); ok {
			hash = h
		}
		if n, ok := torMap["name"].(string); ok {
			name = n
		}
	}

	return id, hash, name, nil
}

// GetTorrents returns all active and completed torrents in Transmission.
func (c *TransmissionClient) GetTorrents(ctx context.Context) ([]TransmissionTorrent, error) {
	fields := []string{
		"id", "name", "hashString", "status", "percentDone",
		"rateDownload", "rateUpload", "eta", "totalSize",
		"downloadedEver", "downloadDir", "isFinished", "error", "errorString",
	}

	resp, err := c.doRPC(ctx, "torrent-get", map[string]interface{}{
		"fields": fields,
	})
	if err != nil {
		return nil, err
	}

	torList, ok := resp.Arguments["torrents"].([]interface{})
	if !ok {
		return []TransmissionTorrent{}, nil
	}

	var results []TransmissionTorrent
	for _, raw := range torList {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		var t TransmissionTorrent
		if id, ok := m["id"].(float64); ok {
			t.ID = int(id)
		}
		if name, ok := m["name"].(string); ok {
			t.Name = name
		}
		if hash, ok := m["hashString"].(string); ok {
			t.HashString = hash
		}
		if status, ok := m["status"].(float64); ok {
			t.Status = int(status)
			switch t.Status {
			case 0:
				t.StatusText = "Paused"
			case 1:
				t.StatusText = "Check Wait"
			case 2:
				t.StatusText = "Checking"
			case 3:
				t.StatusText = "Download Wait"
			case 4:
				t.StatusText = "Downloading"
			case 5:
				t.StatusText = "Seed Wait"
			case 6:
				t.StatusText = "Seeding"
			default:
				t.StatusText = "Unknown"
			}
		}
		if pct, ok := m["percentDone"].(float64); ok {
			t.PercentDone = pct
		}
		if rd, ok := m["rateDownload"].(float64); ok {
			t.RateDownload = int64(rd)
		}
		if ru, ok := m["rateUpload"].(float64); ok {
			t.RateUpload = int64(ru)
		}
		if eta, ok := m["eta"].(float64); ok {
			t.ETA = int64(eta)
		}
		if ts, ok := m["totalSize"].(float64); ok {
			t.TotalSize = int64(ts)
		}
		if d, ok := m["downloadedEver"].(float64); ok {
			t.Downloaded = int64(d)
		}
		if dir, ok := m["downloadDir"].(string); ok {
			t.DownloadDir = dir
		}
		if fin, ok := m["isFinished"].(bool); ok {
			t.IsFinished = fin
		}
		if errCode, ok := m["error"].(float64); ok {
			t.Error = int(errCode)
		}
		if errStr, ok := m["errorString"].(string); ok {
			t.ErrorString = errStr
		}

		results = append(results, t)
	}

	return results, nil
}

// RemoveTorrent deletes a torrent from Transmission by ID.
func (c *TransmissionClient) RemoveTorrent(ctx context.Context, id int, deleteLocalData bool) error {
	_, err := c.doRPC(ctx, "torrent-remove", map[string]interface{}{
		"ids":               []int{id},
		"delete-local-data": deleteLocalData,
	})
	return err
}
