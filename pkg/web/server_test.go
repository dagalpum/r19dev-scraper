package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestWebServerEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_web_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	defer d.Close()

	srv, err := NewServer(Config{
		TargetDir: tempDir,
		Port:      8080,
		DB:        d,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	handler, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// 1. Test Static Files
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	wRoot := httptest.NewRecorder()
	handler.ServeHTTP(wRoot, reqRoot)
	if wRoot.Code != http.StatusOK || !strings.Contains(wRoot.Body.String(), "R19DEV") {
		t.Errorf("GET / failed: code %d, body: %s", wRoot.Code, wRoot.Body.String())
	}

	reqCSS := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	wCSS := httptest.NewRecorder()
	handler.ServeHTTP(wCSS, reqCSS)
	if wCSS.Code != http.StatusOK {
		t.Errorf("GET /style.css failed: code %d", wCSS.Code)
	}

	// 2. Test /api/scan
	reqScan := httptest.NewRequest(http.MethodGet, "/api/scan", nil)
	wScan := httptest.NewRecorder()
	handler.ServeHTTP(wScan, reqScan)
	if wScan.Code != http.StatusOK {
		t.Errorf("GET /api/scan failed: code %d", wScan.Code)
	}

	// 3. Test /api/actresses/follow & list
	followBody := bytes.NewBufferString(`{"name":"Kanna Seto","ja_name":"瀬戸環奈","image_url":"https://example.com/kanna.jpg"}`)
	reqFollow := httptest.NewRequest(http.MethodPost, "/api/actresses/follow", followBody)
	wFollow := httptest.NewRecorder()
	handler.ServeHTTP(wFollow, reqFollow)
	if wFollow.Code != http.StatusOK {
		t.Errorf("POST /api/actresses/follow failed: code %d", wFollow.Code)
	}

	reqActresses := httptest.NewRequest(http.MethodGet, "/api/actresses", nil)
	wActresses := httptest.NewRecorder()
	handler.ServeHTTP(wActresses, reqActresses)
	if wActresses.Code != http.StatusOK || !strings.Contains(wActresses.Body.String(), "Kanna Seto") {
		t.Errorf("GET /api/actresses failed: code %d, body: %s", wActresses.Code, wActresses.Body.String())
	}

	// 4. Test /api/movie/{id}/state
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "SNOS-038",
		Title:       "Test Movie",
		ReleaseDate: "2026-01-09",
		ScrapedAt:   time.Now(),
	})

	stateBody := bytes.NewBufferString(`{"is_watched":true,"user_rating":5,"is_favorite":true}`)
	reqState := httptest.NewRequest(http.MethodPost, "/api/movie/SNOS-038/state", stateBody)
	wState := httptest.NewRecorder()
	handler.ServeHTTP(wState, reqState)
	if wState.Code != http.StatusOK {
		t.Errorf("POST /api/movie/SNOS-038/state failed: code %d", wState.Code)
	}

	reqGetMovie := httptest.NewRequest(http.MethodGet, "/api/movie/SNOS-038", nil)
	wGetMovie := httptest.NewRecorder()
	handler.ServeHTTP(wGetMovie, reqGetMovie)
	if wGetMovie.Code != http.StatusOK || !strings.Contains(wGetMovie.Body.String(), "Test Movie") {
		t.Errorf("GET /api/movie/SNOS-038 failed: code %d", wGetMovie.Code)
	}

	// 5. Test /api/actresses/discovered
	reqDiscovered := httptest.NewRequest(http.MethodGet, "/api/actresses/discovered", nil)
	wDiscovered := httptest.NewRecorder()
	handler.ServeHTTP(wDiscovered, reqDiscovered)
	if wDiscovered.Code != http.StatusOK || !strings.Contains(wDiscovered.Body.String(), "actresses") {
		t.Errorf("GET /api/actresses/discovered failed: code %d, body: %s", wDiscovered.Code, wDiscovered.Body.String())
	}

	// 6. Test /api/filters
	reqFilters := httptest.NewRequest(http.MethodGet, "/api/filters", nil)
	wFilters := httptest.NewRecorder()
	handler.ServeHTTP(wFilters, reqFilters)
	if wFilters.Code != http.StatusOK || !strings.Contains(wFilters.Body.String(), "blocked_prefixes") {
		t.Errorf("GET /api/filters failed: code %d, body: %s", wFilters.Code, wFilters.Body.String())
	}

	// Test POST /api/filters
	updateBody := bytes.NewBufferString(`{"version":1,"enabled":true,"blocked_prefixes":["IPOK","MYTEST"]}`)
	reqUpdateFilter := httptest.NewRequest(http.MethodPost, "/api/filters", updateBody)
	wUpdateFilter := httptest.NewRecorder()
	handler.ServeHTTP(wUpdateFilter, reqUpdateFilter)
	if wUpdateFilter.Code != http.StatusOK || !strings.Contains(wUpdateFilter.Body.String(), "MYTEST") {
		t.Errorf("POST /api/filters failed: code %d, body: %s", wUpdateFilter.Code, wUpdateFilter.Body.String())
	}

	// Test POST /api/filters/reset
	reqResetFilter := httptest.NewRequest(http.MethodPost, "/api/filters/reset", nil)
	wResetFilter := httptest.NewRecorder()
	handler.ServeHTTP(wResetFilter, reqResetFilter)
	if wResetFilter.Code != http.StatusOK || !strings.Contains(wResetFilter.Body.String(), "success") {
		t.Errorf("POST /api/filters/reset failed: code %d, body: %s", wResetFilter.Code, wResetFilter.Body.String())
	}

	// Test POST /api/filters/purge
	reqPurge := httptest.NewRequest(http.MethodPost, "/api/filters/purge", nil)
	wPurge := httptest.NewRecorder()
	handler.ServeHTTP(wPurge, reqPurge)
	if wPurge.Code != http.StatusOK || !strings.Contains(wPurge.Body.String(), "purged_count") {
		t.Errorf("POST /api/filters/purge failed: code %d, body: %s", wPurge.Code, wPurge.Body.String())
	}

	// 7. Test /api/settings (GET and POST)
	reqSettings := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	wSettings := httptest.NewRecorder()
	handler.ServeHTTP(wSettings, reqSettings)
	if wSettings.Code != http.StatusOK || !strings.Contains(wSettings.Body.String(), "auto_organize_completed") {
		t.Errorf("GET /api/settings failed: code %d, body: %s", wSettings.Code, wSettings.Body.String())
	}

	postSettingsBody := bytes.NewBufferString(`{"auto_organize_completed":"true","transmission_url":"http://192.168.1.50:9091"}`)
	reqPostSettings := httptest.NewRequest(http.MethodPost, "/api/settings", postSettingsBody)
	wPostSettings := httptest.NewRecorder()
	handler.ServeHTTP(wPostSettings, reqPostSettings)
	if wPostSettings.Code != http.StatusOK || !strings.Contains(wPostSettings.Body.String(), "success") {
		t.Errorf("POST /api/settings failed: code %d, body: %s", wPostSettings.Code, wPostSettings.Body.String())
	}

	// 8. Test /api/torrents/queue
	reqQueue := httptest.NewRequest(http.MethodGet, "/api/torrents/queue", nil)
	wQueue := httptest.NewRecorder()
	handler.ServeHTTP(wQueue, reqQueue)
	if wQueue.Code != http.StatusOK {
		t.Errorf("GET /api/torrents/queue failed: code %d", wQueue.Code)
	}

	// 9. Test /api/webhook/download-complete
	webhookBody := bytes.NewBufferString(`{"torrent_id":99,"name":"SNOS-038.mp4","dir":"/tmp","auto_organize":false}`)
	reqWebhook := httptest.NewRequest(http.MethodPost, "/api/webhook/download-complete", webhookBody)
	reqWebhook.Header.Set("Content-Type", "application/json")
	wWebhook := httptest.NewRecorder()
	handler.ServeHTTP(wWebhook, reqWebhook)
	if wWebhook.Code != http.StatusOK || !strings.Contains(wWebhook.Body.String(), "Webhook processed") {
		t.Errorf("POST /api/webhook/download-complete failed: code %d, body: %s", wWebhook.Code, wWebhook.Body.String())
	}

	// 10. Test /api/torrents/queue/organize with invalid ID
	orgBody := bytes.NewBufferString(`{"id":99999}`)
	reqOrg := httptest.NewRequest(http.MethodPost, "/api/torrents/queue/organize", orgBody)
	wOrg := httptest.NewRecorder()
	handler.ServeHTTP(wOrg, reqOrg)
	if wOrg.Code != http.StatusNotFound {
		t.Errorf("POST /api/torrents/queue/organize expected 404 for nonexistent id, got %d", wOrg.Code)
	}
}
