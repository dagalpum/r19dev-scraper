package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestDBOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_db_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	// 1. Test Follow / Unfollow Actress
	if err := d.FollowActress("Kanna Seto", "瀬戸環奈", "https://example.com/kanna.jpg"); err != nil {
		t.Fatalf("FollowActress failed: %v", err)
	}

	followed, err := d.IsActressFollowed("kanna seto") // Case-insensitive
	if err != nil || !followed {
		t.Errorf("IsActressFollowed expected true, got %v (err: %v)", followed, err)
	}

	actresses, err := d.ListFollowedActresses()
	if err != nil || len(actresses) != 1 || actresses[0].Name != "Kanna Seto" {
		t.Errorf("ListFollowedActresses mismatch: %+v", actresses)
	}

	// 2. Test Save & Get Movie
	movie := &scraper.Movie{
		ID:             "SNOS-038",
		CombinedID:     "snos00038",
		Title:          "Sample Movie",
		Maker:          "S1",
		ReleaseDate:    "2026-01-09",
		RuntimeMinutes: 120,
		Actresses: []scraper.Actress{
			{Name: "Kanna Seto", JaName: "瀬戸環奈"},
		},
		Genres:            []string{"Beautiful Girl", "Hi-Def"},
		SampleScreenshots: []string{"https://example.com/1.jpg"},
		ScrapedAt:         time.Now(),
	}

	if err := d.SaveMovie(movie); err != nil {
		t.Fatalf("SaveMovie failed: %v", err)
	}

	saved, err := d.GetMovie("snos-038")
	if err != nil || saved == nil || saved.Title != movie.Title {
		t.Errorf("GetMovie failed: %+v (err: %v)", saved, err)
	}

	// 3. Test User State: Watched, Rating, Favorite
	if _, err := d.ToggleWatched("SNOS-038"); err != nil {
		t.Fatalf("ToggleWatched failed: %v", err)
	}
	if err := d.SetRating("SNOS-038", 5); err != nil {
		t.Fatalf("SetRating failed: %v", err)
	}
	if _, err := d.ToggleFavorite("SNOS-038"); err != nil {
		t.Fatalf("ToggleFavorite failed: %v", err)
	}

	st, err := d.GetUserState("SNOS-038")
	if err != nil || st == nil || !st.IsWatched || st.UserRating != 5 || !st.IsFavorite {
		t.Errorf("UserState mismatch: %+v", st)
	}

	// 4. Test Library Files
	rec := LibraryFileRecord{
		FilePath:  "/nas/SNOS-038.mp4",
		MovieID:   "SNOS-038",
		SizeBytes: 1024 * 1024 * 1024,
	}
	if err := d.UpsertLibraryFile(rec); err != nil {
		t.Fatalf("UpsertLibraryFile failed: %v", err)
	}

	inLib, err := d.HasMovieInLibrary("SNOS-038")
	if err != nil || !inLib {
		t.Errorf("HasMovieInLibrary expected true, got %v", inLib)
	}
}
