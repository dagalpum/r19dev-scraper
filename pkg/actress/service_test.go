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
		{"MKCK-429", "Soft-Breast Sex 50", "", []string{"Big Tits", "Compilation"}, true},
		{"VRKM-1769", "Fascinating Hairless 300 Minutes", "", []string{"Compilation", "Over 4 Hours"}, true},
		{"S209AJMEM00081", "S1 Campaign 2025 Special Photo Book", "", []string{"Collection Of Photographs"}, true},
		{"MIDE-999", "人気女優 240分 総集編", "", nil, true},
		{"CAWD-123", "オムニバス 傑作選", "", nil, true},
		// Group 3: Non-AV Variety show
		{"KCKC-210", "カチコチTV #210", "", nil, true},
		// Cheki / Limited goods bundle
		{"EBDB-1051", "Julia18 チェキ付き", "", nil, true},
		// Group 4: Director cut / Remaster re-issues
		{"SSIS-160", "未公開映像収録のプレミアムエディション！ディレクターズカット版 新人NO.1STYLE 河北彩花AVデビュー", "", nil, true},
		// Group 1: Multi-body / Multi-actress omnibus
		{"RKI-114", "THE AV WORLD SPECIAL このカ・ラ・ダ超絶品。 50体480分", "", nil, true},
		{"MKCK-417", "Body Specialized For SEX 600min", "", []string{"Over 4 Hours"}, true},
		// Group 5: Anniversary crossover works MUST NOT be filtered!
		{"SONE-566", "S1 20th Anniversary Is The Strongest Tag Team Work", "", []string{"Harem"}, false},
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

	// Save genuine solo release
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "FWAY-095",
		Title:       "Everyone Loves Boobs. Shido Rui",
		ReleaseDate: "2025-06-03",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Beautiful Tits", "Slender", "Exclusive Distribution", "Featured Actress", "4K", "Documentary", "Hi-Def"},
	})

	// Save Group 5: Official anniversary crossover film (MUST be preserved!)
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "SONE-566",
		Title:       "S1 20th Anniversary Is The Strongest Tag Team Work In The History Of AV",
		ReleaseDate: "2025-08-01",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}, {Name: "Other Actress 1"}, {Name: "Other Actress 2"}, {Name: "Other Actress 3"}, {Name: "Other Actress 4"}},
		Genres:      []string{"Harem", "Beautiful Girl"},
	})

	// Save Group 2: Duplicate SKU variants (PPP-485 vs PPPD-485) - must be deduplicated to 1!
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "PPP-485",
		Title:       "乳エステ通い妻 紫堂るい",
		ReleaseDate: "2025-04-01",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Slender"},
	})
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "PPPD-485",
		Title:       "乳エステ通い妻 紫堂るい",
		ReleaseDate: "2025-04-01",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Slender"},
	})

	// Save Group 3: Variety talk show (must be filtered!)
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "KCKC-210",
		Title:       "カチコチTV #210",
		ReleaseDate: "2025-05-01",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
	})

	// Save Group 4: Director's cut re-issue (must be filtered!)
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "SSIS-160",
		Title:       "未公開映像収録のプレミアムエディション！ディレクターズカット版 新人NO.1STYLE",
		ReleaseDate: "2025-05-02",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
	})

	// Save unowned compilation / omnibus recuts (must be filtered!)
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "MKCK-417",
		Title:       "Body Specialized For SEX 600min",
		ReleaseDate: "2025-07-01",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
		Genres:      []string{"Over 4 Hours"},
	})
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "RKI-114",
		Title:       "THE AV WORLD SPECIAL このカ・ラ・ダ超絶品。 50体480分",
		ReleaseDate: "2025-07-02",
		Actresses:   []scraper.Actress{{Name: "Rui Shido"}},
	})

	summary, err := svc.GetActressSummary(context.Background(), "Rui Shido")
	if err != nil {
		t.Fatalf("GetActressSummary failed: %v", err)
	}

	// Must contain ONLY 3 genuine releases:
	// 1. FWAY-095 (Solo)
	// 2. SONE-566 (Anniversary preserved)
	// 3. PPP-485 (Deduplicated with PPPD-485)
	if summary.Total != 3 {
		t.Errorf("Expected exactly 3 genuine releases, got %d (releases: %+v)", summary.Total, summary.Releases)
	}

	// Top genres must only contain genuine acting themes (Beautiful Tits, Slender, Harem, etc.), NOT technical specs
	for _, g := range summary.TopGenres {
		switch g.Genre {
		case "Special Offers And Set Products", "Compilation", "Exclusive Distribution", "Featured Actress", "4K", "Documentary", "Hi-Def":
			t.Errorf("Top genres should not contain non-acting/technical genre %q: %+v", g.Genre, summary.TopGenres)
		}
	}
}

func TestListDiscoveredActresses(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_discovered_test_*")
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

	// Follow only Actress A
	_ = svc.Follow("Actress A", "女優A", "https://example.com/a.jpg")

	// Save movie with Actress A and Actress B (unfollowed)
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "TEST-001",
		Title:       "Test Movie 1",
		ReleaseDate: "2026-03-01",
		Actresses: []scraper.Actress{
			{Name: "Actress A", JaName: "女優A"},
			{Name: "Actress B", JaName: "女優B", ImageURL: "https://example.com/b.jpg"},
		},
	})
	// Save another movie with Actress B and Actress C (unfollowed)
	_ = d.SaveMovie(&scraper.Movie{
		ID:          "TEST-002",
		Title:       "Test Movie 2",
		ReleaseDate: "2026-04-01",
		Actresses: []scraper.Actress{
			{Name: "Actress B", JaName: "女優B"},
			{Name: "Actress C", JaName: "女優C"},
		},
	})

	// Before movies are in library, discovered should be empty
	discovered, err := svc.ListDiscoveredActresses()
	if err != nil {
		t.Fatalf("ListDiscoveredActresses failed: %v", err)
	}
	if len(discovered) != 0 {
		t.Errorf("Expected 0 discovered before library addition, got %d", len(discovered))
	}

	// Add TEST-001 to organized_movies and TEST-002 to library_files
	_ = d.SetOrganized("TEST-001", "/nas/organized/TEST-001", "/nas/organized/TEST-001/TEST-001.mp4")
	_ = d.UpsertLibraryFile(db.LibraryFileRecord{
		FilePath: "/nas/staging/TEST-002.mp4",
		MovieID:  "TEST-002",
	})

	// Now Actress B (2 movies) and Actress C (1 movie) should be discovered, while Actress A is excluded because she is followed
	discovered, err = svc.ListDiscoveredActresses()
	if err != nil {
		t.Fatalf("ListDiscoveredActresses failed: %v", err)
	}

	if len(discovered) != 2 {
		t.Fatalf("Expected 2 discovered actresses, got %d: %+v", len(discovered), discovered)
	}

	// Actress B should be first (2 movies > 1 movie)
	if discovered[0].Name != "Actress B" || discovered[0].MovieCount != 2 {
		t.Errorf("Expected Actress B with 2 movies, got %+v", discovered[0])
	}
	if discovered[0].LatestRelease != "2026-04-01" {
		t.Errorf("Expected latest release 2026-04-01, got %q", discovered[0].LatestRelease)
	}

	// Actress C should be second (1 movie)
	if discovered[1].Name != "Actress C" || discovered[1].MovieCount != 1 {
		t.Errorf("Expected Actress C with 1 movie, got %+v", discovered[1])
	}
}

func TestCleanMovieTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "【数量限定】潮吹きクイーン誕生 170cm幼顔Icupでお漏らしバグボディ 雛形みくる 生写真3枚セット",
			expected: "潮吹きクイーン誕生 170cm幼顔Icupでお漏らしバグボディ 雛形みくる",
		},
		{
			input:    "【FANZA限定】娘の友達は幼顔なのに…身長170cm！おっぱいIカップ！ 雛形みくる （ブルーレイディスク） 生写真3枚セット",
			expected: "娘の友達は幼顔なのに…身長170cm！おっぱいIカップ！ 雛形みくる",
		},
		{
			input:    "【数量限定】男を虜にする無意識のたわわな誘惑 雛形みくる (Blu-ray Disc) チェキ付き",
			expected: "男を虜にする無意識のたわわな誘惑 雛形みくる",
		},
		{
			input:    "【FANZA限定】White Mirage 生写真3枚セット",
			expected: "White Mirage",
		},
		{
			input:    "新人NO.1STYLE 雛形みくる AVデビュー",
			expected: "新人NO.1STYLE 雛形みくる AVデビュー",
		},
	}

	for _, tc := range tests {
		got := CleanMovieTitle(tc.input)
		if got != tc.expected {
			t.Errorf("CleanMovieTitle(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestPromoBonusNotSkippedAndSkippedReleases(t *testing.T) {
	// 1. Promo raw photo bonus set on a standard movie ID should NOT be skipped
	shouldSkip, reason := CheckFilmographyInclusion("SNOS-175", "潮吹きクイーン誕生 生写真3枚セット", "", "", nil)
	if shouldSkip {
		t.Errorf("Expected SNOS-175 with photo bonus not to be skipped, got skip=true, reason=%s", reason)
	}

	// 2. Photobook should be skipped with Photobook reason
	shouldSkip, reason = CheckFilmographyInclusion("B600ZSGK41601", "雛形みくる 純欲があふれてる", "", "", nil)
	if !shouldSkip || reason != "Photobook / Digital Book" {
		t.Errorf("Expected B600ZSGK41601 to be skipped as Photobook, got %v (%s)", shouldSkip, reason)
	}

	// 3. Omnibus compilation should be skipped
	shouldSkip, reason = CheckFilmographyInclusion("MKCK-417", "総集編 600min", "", "", nil)
	if !shouldSkip || reason != "Omnibus Compilation" {
		t.Errorf("Expected MKCK-417 to be skipped as Omnibus Compilation, got %v (%s)", shouldSkip, reason)
	}

	// 4. TK Promotional SKU Variants should be skipped
	shouldSkip, reason = CheckFilmographyInclusion("TKCJOD-510", "My Female Boss...", "【FANZA限定】... JULIA チェキセット", "", nil)
	if !shouldSkip || reason != "Promotional SKU Variant" {
		t.Errorf("Expected TKCJOD-510 to be skipped as Promotional SKU Variant, got %v (%s)", shouldSkip, reason)
	}

	shouldSkip, reason = CheckFilmographyInclusion("TKMFYD-123", "Limited quantity...", "【数量限定】... JULIA チェキセット", "", nil)
	if !shouldSkip || reason != "Promotional SKU Variant" {
		t.Errorf("Expected TKMFYD-123 to be skipped as Promotional SKU Variant, got %v (%s)", shouldSkip, reason)
	}

	// 5. RBB Omnibus Compilation should be skipped
	shouldSkip, reason = CheckFilmographyInclusion("RBB-334", "My face is covered with cum! ... 80 rounds of massive facial cumshot!", "顔面ザーメンまみれであら大変！...大量顔射ぶっかけ80連発！", "", nil)
	if !shouldSkip || reason != "Omnibus Compilation" {
		t.Errorf("Expected RBB-334 to be skipped as Omnibus Compilation, got %v (%s)", shouldSkip, reason)
	}
}


