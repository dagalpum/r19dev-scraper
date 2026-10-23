package torrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQBittorrentClient_AllOperations(t *testing.T) {
	var loggedIn bool
	addedCategory := ""
	addedSavepath := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", 400)
				return
			}
			user := r.FormValue("username")
			pass := r.FormValue("password")
			if user == "admin" && pass == "adminadmin" {
				loggedIn = true
				http.SetCookie(w, &http.Cookie{
					Name:  "SID",
					Value: "mock-qb-session-cookie-12345",
					Path:  "/",
				})
				w.Write([]byte("Ok."))
				return
			}
			http.Error(w, "Fails.", http.StatusForbidden)

		case "/api/v2/app/version":
			if !loggedIn {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			w.Write([]byte("v4.6.5"))

		case "/api/v2/app/defaultSavePath":
			if !loggedIn {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			w.Write([]byte("/volume1/downloads"))

		case "/api/v2/torrents/add":
			if !loggedIn {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			_ = r.ParseForm()
			addedCategory = r.FormValue("category")
			addedSavepath = r.FormValue("savepath")
			w.Write([]byte("Ok."))

		case "/api/v2/torrents/info":
			if !loggedIn {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			jsonResp := `[
				{
					"hash": "42bf9453a53ab13267953ffbb25392c931add6d1",
					"name": "SNOS-424.mp4",
					"size": 4294967296,
					"total_size": 4294967296,
					"progress": 0.55,
					"dlspeed": 5242880,
					"upspeed": 10240,
					"eta": 120,
					"state": "downloading",
					"save_path": "/volume1/downloads",
					"completed": 2362232012
				},
				{
					"hash": "aabbccddeeff00112233445566778899aabbccdd",
					"name": "MIDA-633.mp4",
					"size": 2147483648,
					"total_size": 2147483648,
					"progress": 1.0,
					"dlspeed": 0,
					"upspeed": 20480,
					"eta": 0,
					"state": "uploading",
					"save_path": "/volume1/downloads",
					"completed": 2147483648
				}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(jsonResp))

		case "/api/v2/torrents/delete":
			if !loggedIn {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			_ = r.ParseForm()
			hashes := r.FormValue("hashes")
			if hashes == "" {
				http.Error(w, "missing hashes", 400)
				return
			}
			w.Write([]byte("Ok."))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewQBittorrentClient(QBittorrentConfig{
		URL:         server.URL,
		Username:    "admin",
		Password:    "adminadmin",
		DownloadDir: "/volume1/downloads",
		Category:    "r19dev",
	})

	if client.ClientType() != "qbittorrent" {
		t.Fatalf("expected ClientType 'qbittorrent', got '%s'", client.ClientType())
	}

	// 1. Test Connection
	version, savePath, err := client.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	if version != "v4.6.5" {
		t.Errorf("expected version 'v4.6.5', got '%s'", version)
	}
	if savePath != "/volume1/downloads" {
		t.Errorf("expected savePath '/volume1/downloads', got '%s'", savePath)
	}

	// 2. Add Torrent
	magnet := "magnet:?xt=urn:btih:42bf9453a53ab13267953ffbb25392c931add6d1&dn=SNOS-424"
	id, hash, _, err := client.AddTorrent(context.Background(), magnet, "/volume1/custom")
	if err != nil {
		t.Fatalf("AddTorrent failed: %v", err)
	}
	if id != 0 {
		t.Errorf("expected id 0 for qBittorrent, got %d", id)
	}
	if hash != "42bf9453a53ab13267953ffbb25392c931add6d1" {
		t.Errorf("expected parsed hash '42bf9453a53ab13267953ffbb25392c931add6d1', got '%s'", hash)
	}
	if addedSavepath != "/volume1/custom" {
		t.Errorf("expected savepath '/volume1/custom', got '%s'", addedSavepath)
	}
	if addedCategory != "r19dev" {
		t.Errorf("expected category 'r19dev', got '%s'", addedCategory)
	}

	// 3. Get Torrents
	torrents, err := client.GetTorrents(context.Background())
	if err != nil {
		t.Fatalf("GetTorrents failed: %v", err)
	}
	if len(torrents) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(torrents))
	}

	// First torrent: downloading
	if torrents[0].HashString != "42bf9453a53ab13267953ffbb25392c931add6d1" {
		t.Errorf("unexpected hash for first torrent: %s", torrents[0].HashString)
	}
	if torrents[0].Status != 4 || torrents[0].StatusText != "Downloading" {
		t.Errorf("expected status 4 / Downloading, got %d / %s", torrents[0].Status, torrents[0].StatusText)
	}
	if torrents[0].PercentDone != 0.55 {
		t.Errorf("expected PercentDone 0.55, got %f", torrents[0].PercentDone)
	}

	// Second torrent: uploading / seeding (finished)
	if !torrents[1].IsFinished || torrents[1].Status != 6 {
		t.Errorf("expected second torrent to be finished/seeding, got IsFinished=%t, Status=%d", torrents[1].IsFinished, torrents[1].Status)
	}

	// 4. Remove Torrent
	err = client.RemoveTorrent(context.Background(), 0, "42bf9453a53ab13267953ffbb25392c931add6d1", false)
	if err != nil {
		t.Fatalf("RemoveTorrent failed: %v", err)
	}
}

func TestQBittorrentClient_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Fails.", http.StatusForbidden)
	}))
	defer server.Close()

	client := NewQBittorrentClient(QBittorrentConfig{
		URL:      server.URL,
		Username: "wrong",
		Password: "password",
	})

	_, _, err := client.TestConnection(context.Background())
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected authentication failed error, got: %v", err)
	}
}
