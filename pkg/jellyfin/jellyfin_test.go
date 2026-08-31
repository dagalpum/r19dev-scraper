package jellyfin

import (
	"strings"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestUpgradeDMMImageURL(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://pics.dmm.co.jp/digital/video/cawb00006/cawb00006-15.jpg",
			expected: "https://pics.dmm.co.jp/digital/video/cawb00006/cawb00006jp-15.jpg",
		},
		{
			input:    "https://pics.dmm.co.jp/digital/video/snos00038/snos00038-1.jpg",
			expected: "https://pics.dmm.co.jp/digital/video/snos00038/snos00038jp-1.jpg",
		},
		{
			input:    "https://pics.dmm.co.jp/digital/video/snos00038/snos00038ps.jpg",
			expected: "https://pics.dmm.co.jp/digital/video/snos00038/snos00038pl.jpg",
		},
		{
			input:    "https://pics.dmm.co.jp/digital/video/snos00038/snos00038jp-5.jpg",
			expected: "https://pics.dmm.co.jp/digital/video/snos00038/snos00038jp-5.jpg",
		},
	}

	for _, c := range cases {
		actual := UpgradeDMMImageURL(c.input)
		if actual != c.expected {
			t.Errorf("UpgradeDMMImageURL(%q) = %q, expected %q", c.input, actual, c.expected)
		}
	}
}

func TestGenerateNFOAndHTML(t *testing.T) {
	movie := &scraper.Movie{
		ID:             "SNOS-038",
		CombinedID:     "snos00038",
		Title:          "AV Debut 1st Anniversary Work",
		OriginalTitle:  "AVデビュー1周年記念作品",
		Maker:          "S1 NO.1 STYLE",
		Director:       "Tiger Kosakai",
		ReleaseDate:    "2026-01-09",
		RuntimeMinutes: 127,
		Actresses: []scraper.Actress{
			{Name: "Kanna Seto", JaName: "瀬戸環奈", ImageURL: "https://pics.dmm.co.jp/mono/actjpgs/seto_kanna.jpg"},
		},
		Genres:            []string{"Beautiful Girl", "Big Tits", "Hi-Def"},
		SampleScreenshots: []string{"https://pics.dmm.co.jp/digital/video/snos00038/snos00038-1.jpg"},
		ScrapedAt:         time.Now(),
	}

	userState := &db.UserState{
		MovieID:    "SNOS-038",
		IsWatched:  true,
		UserRating: 5,
		IsFavorite: true,
	}

	// 1. Test NFO Generation
	nfoBytes, err := GenerateNFO(movie, userState)
	if err != nil {
		t.Fatalf("GenerateNFO failed: %v", err)
	}

	nfoStr := string(nfoBytes)
	if !strings.Contains(nfoStr, "<title>[SNOS-038] AV Debut 1st Anniversary Work</title>") {
		t.Errorf("NFO missing title tag: %s", nfoStr)
	}
	if !strings.Contains(nfoStr, "<watched>true</watched>") {
		t.Errorf("NFO missing watched tag: %s", nfoStr)
	}
	if !strings.Contains(nfoStr, "<userrating>10</userrating>") {
		t.Errorf("NFO missing userrating: %s", nfoStr)
	}

	// 2. Test HTML Generation
	htmlStr := GenerateHTML(movie, userState)
	if !strings.Contains(htmlStr, "Kanna Seto") || !strings.Contains(htmlStr, "SNOS-038") {
		t.Errorf("HTML missing actress or ID")
	}
	if !strings.Contains(htmlStr, "Watched") {
		t.Errorf("HTML missing watched badge")
	}
}
