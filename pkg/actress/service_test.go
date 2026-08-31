package actress

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestActressService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_actress_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	d, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	defer d.Close()

	svc := New(d, nil)

	// 1. Follow Actress
	if err := svc.Follow("Kanna Seto", "瀬戸環奈", "https://example.com/kanna.jpg"); err != nil {
		t.Fatalf("Follow failed: %v", err)
	}

	followed, err := svc.IsFollowed("Kanna Seto")
	if err != nil || !followed {
		t.Errorf("IsFollowed expected true, got %v", followed)
	}

	// 2. Add sample movies to DB
	movie1 := &scraper.Movie{
		ID:             "SNOS-038",
		Title:          "Movie 1",
		ReleaseDate:    "2026-01-09",
		Actresses:      []scraper.Actress{{Name: "Kanna Seto"}},
		ScrapedAt:      time.Now(),
	}
	movie2 := &scraper.Movie{
		ID:             "SNOS-099",
		Title:          "Movie 2 (New / Missing)",
		ReleaseDate:    "2026-02-15",
		Actresses:      []scraper.Actress{{Name: "Kanna Seto"}},
		ScrapedAt:      time.Now(),
	}
	_ = d.SaveMovie(movie1)
	_ = d.SaveMovie(movie2)

	// Mark movie1 as in library and watched
	_ = d.UpsertLibraryFile(db.LibraryFileRecord{
		FilePath: "/nas/SNOS-038.mp4",
		MovieID:  "SNOS-038",
	})
	_ = d.SetUserState(db.UserState{
		MovieID:    "SNOS-038",
		IsWatched:  true,
		UserRating: 5,
		IsFavorite: true,
	})

	// 3. Get Summary
	summary, err := svc.GetActressSummary(context.Background(), "Kanna Seto")
	if err != nil {
		t.Fatalf("GetActressSummary failed: %v", err)
	}

	if summary.Total != 2 {
		t.Errorf("Expected 2 releases, got %d", summary.Total)
	}
	if summary.Downloaded != 1 {
		t.Errorf("Expected 1 downloaded, got %d", summary.Downloaded)
	}
	if summary.Missing != 1 {
		t.Errorf("Expected 1 missing, got %d", summary.Missing)
	}
	if summary.Watched != 1 {
		t.Errorf("Expected 1 watched, got %d", summary.Watched)
	}
	if summary.Favorites != 1 {
		t.Errorf("Expected 1 favorite, got %d", summary.Favorites)
	}
}
