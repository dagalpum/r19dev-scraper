package organizer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestOrganizeMatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_organize_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "src")
	destRoot := filepath.Join(tempDir, "dest")
	_ = os.MkdirAll(srcDir, 0o755)

	// Create dummy video files
	video1 := filepath.Join(srcDir, "hhd800.com@SNOS-038.mp4")
	_ = os.WriteFile(video1, []byte("dummy video content"), 0o644)

	match := &matcher.MatchResult{
		File: scanner.FileInfo{
			Path: video1,
			Name: "hhd800.com@SNOS-038.mp4",
			Size: 1024,
		},
		ID:        "SNOS-038",
		MatchedBy: "regex_standard",
	}

	movie := &scraper.Movie{
		ID:             "SNOS-038",
		CombinedID:     "snos00038",
		Title:          "AV Debut 1st Anniversary Work",
		OriginalTitle:  "AVデビュー1周年記念作品",
		Maker:          "S1 NO.1 STYLE",
		ReleaseDate:    "2026-01-09",
		RuntimeMinutes: 127,
		Actresses: []scraper.Actress{
			{Name: "Kanna Seto", JaName: "瀬戸環奈"},
		},
		Genres:    []string{"Beautiful Girl", "Hi-Def"},
		ScrapedAt: time.Now(),
	}

	userState := &db.UserState{
		MovieID:    "SNOS-038",
		IsWatched:  true,
		UserRating: 5,
	}

	// 1. Test Dry-Run (English Actress name priority)
	dryPlan, err := OrganizeMatch(context.Background(), match, movie, userState, destRoot, true)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}
	if !strings.Contains(dryPlan.TargetFolder, "Kanna Seto") {
		t.Errorf("Actress folder mismatch (expected English Kanna Seto): %s", dryPlan.TargetFolder)
	}
	if strings.Contains(dryPlan.TargetFolder, "瀬戸環奈") {
		t.Errorf("Actress folder should not contain Japanese when English is present: %s", dryPlan.TargetFolder)
	}
	if !strings.HasSuffix(dryPlan.TargetVideo, "SNOS-038.mp4") {
		t.Errorf("Target video filename mismatch: %s", dryPlan.TargetVideo)
	}

	// 1b. Test Japanese Fallback when English Name is empty
	movieJaOnly := *movie
	movieJaOnly.Actresses = []scraper.Actress{{Name: "", JaName: "葵つかさ"}}
	planJa, err := PlanOrganize(match, &movieJaOnly, destRoot)
	if err != nil {
		t.Fatalf("Plan failed for JaOnly: %v", err)
	}
	if !strings.Contains(planJa.TargetFolder, "葵つかさ") {
		t.Errorf("Expected Japanese fallback 葵つかさ: %s", planJa.TargetFolder)
	}

	// 1c. Test Unknown Actress fallback
	movieNoActress := *movie
	movieNoActress.Actresses = nil
	planUnknown, err := PlanOrganize(match, &movieNoActress, destRoot)
	if err != nil {
		t.Fatalf("Plan failed for Unknown Actress: %v", err)
	}
	if !strings.Contains(planUnknown.TargetFolder, "Unknown Actress") {
		t.Errorf("Expected Unknown Actress: %s", planUnknown.TargetFolder)
	}

	// 1d. Test English Movie Title priority and Japanese Title fallback
	if !strings.Contains(dryPlan.TargetFolder, "SNOS-038 AV Debut 1st Anniversary Work") {
		t.Errorf("Expected English movie title in folder: %s", dryPlan.TargetFolder)
	}

	movieJaTitleOnly := *movie
	movieJaTitleOnly.Title = ""
	movieJaTitleOnly.OriginalTitle = "夫の目の前で犯されて"
	planJaTitle, err := PlanOrganize(match, &movieJaTitleOnly, destRoot)
	if err != nil {
		t.Fatalf("Plan failed for JaTitleOnly: %v", err)
	}
	if !strings.Contains(planJaTitle.TargetFolder, "SNOS-038 夫の目の前で犯されて") {
		t.Errorf("Expected Japanese title fallback: %s", planJaTitle.TargetFolder)
	}

	// Verify source video still exists after dry run
	if _, err := os.Stat(video1); err != nil {
		t.Errorf("Dry-run moved file unexpectedly: %v", err)
	}

	// 2. Test Real Run
	res, err := OrganizeMatch(context.Background(), match, movie, userState, destRoot, false)
	if err != nil {
		t.Fatalf("Real organize failed: %v", err)
	}

	// Verify moved video
	if _, err := os.Stat(res.TargetVideo); err != nil {
		t.Errorf("Target video not found: %s", res.TargetVideo)
	}
	// Verify NFO file
	if _, err := os.Stat(res.NFOPath); err != nil {
		t.Errorf("NFO file not found: %s", res.NFOPath)
	}
	// Verify HTML file
	if _, err := os.Stat(res.HTMLPath); err != nil {
		t.Errorf("HTML file not found: %s", res.HTMLPath)
	}
}
