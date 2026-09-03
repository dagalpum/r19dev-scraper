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
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	srv.db = d

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
}
