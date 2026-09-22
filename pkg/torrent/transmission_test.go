package torrent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransmissionClient(t *testing.T) {
	sessionHandshakeDone := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessID := r.Header.Get("X-Transmission-Session-Id")
		if sessID != "test-session-12345" {
			w.Header().Set("X-Transmission-Session-Id", "test-session-12345")
			w.WriteHeader(http.StatusConflict)
			return
		}

		sessionHandshakeDone = true

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "session-get":
			_ = json.NewEncoder(w).Encode(rpcResponse{
				Result: "success",
				Arguments: map[string]interface{}{
					"version":      "4.0.5 (d1e2f3)",
					"download-dir": "/downloads",
				},
			})
		case "torrent-add":
			_ = json.NewEncoder(w).Encode(rpcResponse{
				Result: "success",
				Arguments: map[string]interface{}{
					"torrent-added": map[string]interface{}{
						"id":         10,
						"name":       "SNOS-250.mp4",
						"hashString": "42bf9453a53ab13267953ffbb25392c931add6d1",
					},
				},
			})
		case "torrent-get":
			_ = json.NewEncoder(w).Encode(rpcResponse{
				Result: "success",
				Arguments: map[string]interface{}{
					"torrents": []interface{}{
						map[string]interface{}{
							"id":           10,
							"name":         "SNOS-250.mp4",
							"hashString":   "42bf9453a53ab13267953ffbb25392c931add6d1",
							"status":       4,
							"percentDone":  0.45,
							"rateDownload": 10485760,
							"eta":          300,
							"totalSize":    6979321856,
						},
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(rpcResponse{Result: "success"})
		}
	}))
	defer ts.Close()

	client := NewTransmissionClient(TransmissionConfig{
		URL: ts.URL,
	})

	// Test connection with handshake
	ver, dir, err := client.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	if !sessionHandshakeDone {
		t.Errorf("Expected 409 session handshake to complete")
	}
	if ver != "4.0.5 (d1e2f3)" || dir != "/downloads" {
		t.Errorf("Unexpected version/dir: %s, %s", ver, dir)
	}

	// Test torrent-add
	id, hash, name, err := client.AddTorrent(context.Background(), "magnet:?xt=urn:btih:42bf9453a53ab13267953ffbb25392c931add6d1", "")
	if err != nil {
		t.Fatalf("AddTorrent failed: %v", err)
	}
	if id != 10 || hash != "42bf9453a53ab13267953ffbb25392c931add6d1" || name != "SNOS-250.mp4" {
		t.Errorf("Unexpected add result: id=%d, hash=%s, name=%s", id, hash, name)
	}

	// Test torrent-get
	torrents, err := client.GetTorrents(context.Background())
	if err != nil {
		t.Fatalf("GetTorrents failed: %v", err)
	}
	if len(torrents) != 1 {
		t.Fatalf("Expected 1 torrent, got %d", len(torrents))
	}
	if torrents[0].PercentDone != 0.45 || torrents[0].StatusText != "Downloading" {
		t.Errorf("Unexpected torrent data: %+v", torrents[0])
	}
}
