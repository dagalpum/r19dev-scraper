package migrator

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCleanEmptyTree(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_clean_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Create an empty directory
	emptyDir := filepath.Join(tempDir, "empty_folder")
	_ = os.MkdirAll(emptyDir, 0o755)

	// 2. Create a non-empty directory with a file
	nonEmptyDir := filepath.Join(tempDir, "keep_folder")
	_ = os.MkdirAll(nonEmptyDir, 0o755)
	_ = os.WriteFile(filepath.Join(nonEmptyDir, "sample.mp4"), []byte("sample"), 0o644)

	// Run cleanEmptyTree
	count := cleanEmptyTree(tempDir)
	if count != 1 {
		t.Errorf("expected 1 cleaned directory, got %d", count)
	}

	// Verify empty directory is gone
	if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
		t.Errorf("expected empty directory to be removed")
	}

	// Verify non-empty directory is preserved
	if _, err := os.Stat(filepath.Join(nonEmptyDir, "sample.mp4")); err != nil {
		t.Errorf("expected non-empty directory and file to be preserved")
	}

	// Verify root directory is preserved
	if _, err := os.Stat(tempDir); err != nil {
		t.Errorf("expected root directory to be preserved")
	}
}

func TestUpgradeHTMLFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_upgrade_html_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup mock SQLite database
	dbPath := filepath.Join(tempDir, "app.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE movies (
			id TEXT PRIMARY KEY,
			title TEXT,
			original_title TEXT,
			maker TEXT,
			label TEXT,
			director TEXT,
			release_date TEXT,
			cover_url TEXT,
			actresses_json TEXT,
			genres_json TEXT
		);
		CREATE TABLE organized_movies (
			movie_id TEXT PRIMARY KEY,
			target_folder TEXT,
			target_video TEXT,
			organized_at DATETIME
		);
	`)
	if err != nil {
		t.Fatalf("failed to init db schema: %v", err)
	}

	// Create 3 mock movie folders
	movieIDs := []string{"TEST-001", "TEST-002", "TEST-003"}
	for _, id := range movieIDs {
		folder := filepath.Join(tempDir, "ActressName", id+" Title")
		_ = os.MkdirAll(folder, 0o755)
		videoPath := filepath.Join(folder, id+".mp4")
		_ = os.WriteFile(videoPath, []byte("fake video"), 0o644)
		// old HTML
		_ = os.WriteFile(filepath.Join(folder, "movie.html"), []byte("<html>old</html>"), 0o644)

		_, _ = db.Exec(`INSERT INTO movies (id, title, original_title, maker, release_date) VALUES (?, ?, ?, ?, ?)`,
			id, "Title "+id, "JaTitle "+id, "S1", "2026-01-01")
		_, _ = db.Exec(`INSERT INTO organized_movies (movie_id, target_folder, target_video) VALUES (?, ?, ?)`,
			id, folder, videoPath)
	}

	var events []ProgressEvent
	count := UpgradeHTMLFiles(context.Background(), tempDir, db, nil, func(e ProgressEvent) {
		events = append(events, e)
	})

	if count != 3 {
		t.Errorf("expected 3 upgraded files, got %d", count)
	}

	// Verify all 3 movie.html files were updated
	for _, id := range movieIDs {
		folder := filepath.Join(tempDir, "ActressName", id+" Title")
		content, err := os.ReadFile(filepath.Join(folder, "movie.html"))
		if err != nil {
			t.Errorf("failed to read upgraded movie.html: %v", err)
		}
		if !strings.Contains(string(content), "btn-play") && !strings.Contains(string(content), "R19dev") {
			t.Errorf("expected cinematic template content in movie.html")
		}
	}

	// Verify progress events were emitted
	if len(events) < 3 {
		t.Errorf("expected at least 3 progress events, got %d", len(events))
	}
}
