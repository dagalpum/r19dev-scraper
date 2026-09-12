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

func TestPromotionalVariantFiltering(t *testing.T) {
	// 1. Direct unit checks for IsPromotionalOrDuplicateVariant
	testCases := []struct {
		id       string
		title    string
		coverURL string
		genres   []string
		expected bool
	}{
		{"FWAY-095", "Everyone Loves Boobs. Shido Rui", "https://example.com/cover.jpg", []string{"Beautiful Tits", "Slender"}, false},
		{"SNOS-038", "AV Debut Kanna Seto", "https://example.com/cover.jpg", []string{"Hi-Def"}, false},
		{"C9FWAY095", "みんな、おっぱいが好き 紫堂るい 3本購入特典付き", "", []string{"Special Offers And Set Products"}, true},
		{"E9FWAY095", "みんな、おっぱいが好き 紫堂るい 2本購入特典付き", "", []string{"Special Offers And Set Products", "Includes Event Participation Rights"}, true},
		{"S9FWAY095", "【FANZA限定】【6月3日20時開催オンラインサイン会参加権付き】紫堂るい 1本購入特典付き", "", nil, true},
		{"N9FWAY104", "好きだぜコノヤロー 瀬戸環奈 5本購入特典付き", "", nil, true},
		{"L9MIDA-438", "福田ゆあ キーホルダーセット", "", nil, true},
		{"9SNOS001", "Compilation title", "", nil, true},
		{"S727AWQGD00043", "君の裸が見たい 福田ゆあ", "https://ebook-assets.dmm.co.jp/digital/e-book/s727awqgd00043/s727awqgd00043pl.jpg", nil, true},
	}

	for _, tc := range testCases {
		res := IsPromotionalOrDuplicateVariant(tc.id, tc.title, tc.coverURL, tc.genres)
		if res != tc.expected {
			t.Errorf("IsPromotionalOrDuplicateVariant(%q, %q) expected %v, got %v", tc.id, tc.title, tc.expected, res)
		}
	}

	// 2. Integration test in GetActressSummary: variants must not appear in summary
	tempDir, err := os.MkdirTemp("", "r19dev_promo_test_*")
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
	_ = svc.Follow("Rui Shido", "紫堂るい", "")

	// Save genuine release
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "FWAY-095",
		Title:       "Everyone Loves Boobs. Shido Rui",
		ReleaseDate: "2025-06-03",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Beautiful Tits", "Slender"},
	})

	// Save promotional duplicates
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "C9FWAY095",
		Title:       "【FANZA限定】紫堂るい 3本購入特典付き",
		ReleaseDate: "2025-06-03",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Special Offers And Set Products"},
	})
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "E9FWAY095",
		Title:       "【FANZA限定】紫堂るい 2本購入特典付き",
		ReleaseDate: "2025-06-03",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Special Offers And Set Products"},
	})
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "S9FWAY095",
		Title:       "【FANZA限定】【オンラインサイン会】紫堂るい 1本購入特典付き",
		ReleaseDate: "2025-06-03",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
	})

	summary, err := svc.GetActressSummary(context.Background(), "Rui Shido")
	if err != nil {
		t.Fatalf("GetActressSummary failed: %v", err)
	}

	// Must contain ONLY 1 genuine release (FWAY-095), C9/E9/S9 filtered out!
	if summary.Total != 1 {
		t.Errorf("Expected exactly 1 genuine release, got %d (releases: %+v)", summary.Total, summary.Releases)
	}
	if len(summary.Releases) != 1 || summary.Releases[0].MovieID != "FWAY-095" {
		t.Errorf("Expected release FWAY-095, got %+v", summary.Releases)
	}

	// Top genres must not contain "Special Offers And Set Products"
	for _, g := range summary.TopGenres {
		if g.Genre == "Special Offers And Set Products" {
			t.Errorf("Top genres should not contain promotional category: %+v", summary.TopGenres)
		}
	}
}

