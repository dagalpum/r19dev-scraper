package cache

import (
	"os"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestCacheMetadataAndImages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_cache_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	c, err := New(tempDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	movie := &scraper.Movie{
		ID:             "SNOS-038",
		CombinedID:     "snos00038",
		Title:          "Sample Movie Title",
		OriginalTitle:  "サンプル",
		Maker:          "S1 NO.1 STYLE",
		ReleaseDate:    "2026-01-09",
		RuntimeMinutes: 120,
		ScrapedAt:      time.Now(),
	}

	// Test Set and Get Movie
	if err := c.SetMovie(movie); err != nil {
		t.Fatalf("SetMovie failed: %v", err)
	}

	cached, found := c.GetMovie("SNOS-038")
	if !found || cached == nil {
		t.Fatalf("GetMovie('SNOS-038') failed to find cached record")
	}
	if cached.Title != movie.Title || cached.Maker != movie.Maker {
		t.Errorf("Cached movie data mismatch. Got %s, expected %s", cached.Title, movie.Title)
	}

	// Test Set and Get Image
	testImageBytes := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00")
	if err := c.SetImage("SNOS-038", testImageBytes); err != nil {
		t.Fatalf("SetImage failed: %v", err)
	}

	imgData, foundImg := c.GetImage("SNOS-038")
	if !foundImg || len(imgData) == 0 {
		t.Fatalf("GetImage('SNOS-038') failed to find cached image")
	}
	if len(imgData) != len(testImageBytes) {
		t.Errorf("Cached image size mismatch: got %d, expected %d", len(imgData), len(testImageBytes))
	}
}
