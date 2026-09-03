package organizer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/jellyfin"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

// OrganizeResult tracks the outcome of an organize operation.
type OrganizeResult struct {
	MovieID        string `json:"movie_id"`
	SourceFile     string `json:"source_file"`
	TargetFolder   string `json:"target_folder"`
	TargetVideo    string `json:"target_video"`
	NFOPath        string `json:"nfo_path"`
	HTMLPath       string `json:"html_path"`
	PosterPath     string `json:"poster_path"`
	FanartPath     string `json:"fanart_path"`
	ScreenshotsNum int    `json:"screenshots_num"`
	IsMultiPart    bool   `json:"is_multi_part"`
	PartNumber     int    `json:"part_number"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// PlanOrganize computes the target destination paths for a movie without performing disk I/O.
func PlanOrganize(match *matcher.MatchResult, movie *scraper.Movie, targetRoot string) (*OrganizeResult, error) {
	if match == nil || movie == nil || targetRoot == "" {
		return nil, fmt.Errorf("invalid match, movie or targetRoot")
	}

	actressFolder := "Unknown Actress"
	if len(movie.Actresses) > 0 {
		act := movie.Actresses[0]
		if strings.TrimSpace(act.Name) != "" {
			actressFolder = strings.TrimSpace(act.Name)
		} else if strings.TrimSpace(act.JaName) != "" {
			actressFolder = strings.TrimSpace(act.JaName)
		}
	}
	actressFolder = jellyfin.SanitizeFilename(actressFolder)

	// Clean movie folder: JAV-ID SanitizedTitle
	cleanTitle := jellyfin.SanitizeFilename(movie.Title)
	if cleanTitle == "" {
		cleanTitle = "Movie"
	}
	movieFolder := jellyfin.SanitizeFilename(fmt.Sprintf("%s %s", movie.ID, cleanTitle))
	targetMovieDir := filepath.Join(targetRoot, actressFolder, movieFolder)

	ext := filepath.Ext(match.File.Path)
	var videoFilename string
	if match.IsMultiPart && match.PartNumber > 0 {
		videoFilename = fmt.Sprintf("%s-cd%d%s", movie.ID, match.PartNumber, ext)
	} else {
		videoFilename = fmt.Sprintf("%s%s", movie.ID, ext)
	}

	targetVideoPath := filepath.Join(targetMovieDir, videoFilename)
	nfoPath := filepath.Join(targetMovieDir, movie.ID+".nfo")
	htmlPath := filepath.Join(targetMovieDir, "movie.html")
	posterPath := filepath.Join(targetMovieDir, "poster.jpg")
	fanartPath := filepath.Join(targetMovieDir, "fanart.jpg")

	return &OrganizeResult{
		MovieID:        movie.ID,
		SourceFile:     match.File.Path,
		TargetFolder:   targetMovieDir,
		TargetVideo:    targetVideoPath,
		NFOPath:        nfoPath,
		HTMLPath:       htmlPath,
		PosterPath:     posterPath,
		FanartPath:     fanartPath,
		ScreenshotsNum: len(movie.SampleScreenshots),
		IsMultiPart:    match.IsMultiPart,
		PartNumber:     match.PartNumber,
		Success:        true,
	}, nil
}

// ProgressReporter defines a callback function to report organize step progress.
type ProgressReporter func(step string, current, total int, message string)

// OrganizeMatchWithProgress moves the video file, creates folder structure, generates NFO/HTML, downloads assets, with real-time progress callbacks.
func OrganizeMatchWithProgress(ctx context.Context, match *matcher.MatchResult, movie *scraper.Movie, userState *db.UserState, targetRoot string, dryRun bool, reporter ProgressReporter) (*OrganizeResult, error) {
	plan, err := PlanOrganize(match, movie, targetRoot)
	if err != nil {
		return nil, err
	}

	if dryRun {
		return plan, nil
	}

	// 1. Create target folder structure
	if reporter != nil {
		reporter("create_folder", 1, 6, fmt.Sprintf("กำลังสร้างโฟลเดอร์ %s...", movie.ID))
	}
	if err := os.MkdirAll(plan.TargetFolder, 0o755); err != nil {
		plan.Success = false
		plan.Error = fmt.Sprintf("failed to create directory %s: %v", plan.TargetFolder, err)
		return plan, err
	}

	// 2. Move video file safely (with cross-device fallback)
	if reporter != nil {
		reporter("move_video", 2, 6, fmt.Sprintf("กำลังย้ายไฟล์วิดีโอ %s...", filepath.Base(match.File.Path)))
	}
	if err := moveFile(match.File.Path, plan.TargetVideo); err != nil {
		plan.Success = false
		plan.Error = fmt.Sprintf("failed to move video file: %v", err)
		return plan, err
	}

	// 3. Generate Jellyfin NFO
	if reporter != nil {
		reporter("write_nfo", 3, 6, fmt.Sprintf("กำลังสร้างไฟล์ Jellyfin Metadata (%s.nfo)...", movie.ID))
	}
	if err := jellyfin.WriteNFO(movie, userState, plan.NFOPath); err != nil {
		// Log warning but don't abort
		plan.Error = fmt.Sprintf("warning: failed to write NFO: %v", err)
	}

	// 4. Generate Standalone HTML Metadata Page
	if reporter != nil {
		reporter("write_html", 4, 6, "กำลังสร้างไฟล์ HTML Viewer (movie.html)...")
	}
	if err := jellyfin.WriteHTML(movie, userState, plan.HTMLPath); err != nil {
		plan.Error = fmt.Sprintf("warning: failed to write HTML: %v", err)
	}

	// 5. Download and write all Jellyfin image assets (poster.jpg, fanart.jpg, extrafanart/...)
	if reporter != nil {
		reporter("download_assets", 5, 6, "กำลังดาวน์โหลดรูปภาพและภาพตัวอย่าง...")
	}
	var assetReporter jellyfin.ProgressReporter
	if reporter != nil {
		assetReporter = func(step string, current, total int, message string) {
			reporter(step, current, total, message)
		}
	}
	_ = jellyfin.DownloadAllAssetsWithProgress(ctx, movie, plan.TargetFolder, assetReporter)

	// 6. Record in SQLite Database
	if reporter != nil {
		reporter("save_db", 6, 6, "กำลังบันทึกสถานะลงฐานข้อมูล SQLite...")
	}
	if defaultDB, dErr := db.Default(); dErr == nil && defaultDB != nil {
		_ = defaultDB.SaveMovie(movie)
		_ = defaultDB.UpsertLibraryFile(db.LibraryFileRecord{
			FilePath:      plan.TargetVideo,
			MovieID:       movie.ID,
			SizeBytes:     match.File.Size,
			IsMultiPart:   match.IsMultiPart,
			PartNumber:    match.PartNumber,
			OrganizedPath: plan.TargetFolder,
		})
	}

	return plan, nil
}

// OrganizeMatch moves the video file, creates the NAS folder structure, and generates all Jellyfin assets.
func OrganizeMatch(ctx context.Context, match *matcher.MatchResult, movie *scraper.Movie, userState *db.UserState, targetRoot string, dryRun bool) (*OrganizeResult, error) {
	return OrganizeMatchWithProgress(ctx, match, movie, userState, targetRoot, dryRun, nil)
}

// moveFile moves a file using os.Rename, falling back to copy+delete across different filesystems/mounts.
func moveFile(src, dst string) error {
	if strings.EqualFold(src, dst) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	// Fast atomic rename (instant on same NAS share/filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Fallback to streaming copy + delete if cross-device link error (EXDEV)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return err
	}

	_ = in.Close()
	_ = out.Close()
	return os.Remove(src)
}
