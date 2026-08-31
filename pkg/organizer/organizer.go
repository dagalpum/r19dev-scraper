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
		if act.JaName != "" && act.Name != "" && act.JaName != act.Name {
			actressFolder = fmt.Sprintf("%s (%s)", act.JaName, act.Name)
		} else if act.Name != "" {
			actressFolder = act.Name
		} else if act.JaName != "" {
			actressFolder = act.JaName
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

// OrganizeMatch moves the video file, creates the NAS folder structure, and generates all Jellyfin assets.
func OrganizeMatch(ctx context.Context, match *matcher.MatchResult, movie *scraper.Movie, userState *db.UserState, targetRoot string, dryRun bool) (*OrganizeResult, error) {
	plan, err := PlanOrganize(match, movie, targetRoot)
	if err != nil {
		return nil, err
	}

	if dryRun {
		return plan, nil
	}

	// 1. Create target folder structure
	if err := os.MkdirAll(plan.TargetFolder, 0o755); err != nil {
		plan.Success = false
		plan.Error = fmt.Sprintf("failed to create directory %s: %v", plan.TargetFolder, err)
		return plan, err
	}

	// 2. Move video file safely (with cross-device fallback)
	if err := moveFile(match.File.Path, plan.TargetVideo); err != nil {
		plan.Success = false
		plan.Error = fmt.Sprintf("failed to move video file: %v", err)
		return plan, err
	}

	// 3. Generate Jellyfin NFO
	if err := jellyfin.WriteNFO(movie, userState, plan.NFOPath); err != nil {
		// Log warning but don't abort
		plan.Error = fmt.Sprintf("warning: failed to write NFO: %v", err)
	}

	// 4. Generate Standalone HTML Metadata Page
	if err := jellyfin.WriteHTML(movie, userState, plan.HTMLPath); err != nil {
		plan.Error = fmt.Sprintf("warning: failed to write HTML: %v", err)
	}

	// 5. Download and write all Jellyfin image assets (poster.jpg, fanart.jpg, extrafanart/...)
	_ = jellyfin.DownloadAllAssets(ctx, movie, plan.TargetFolder)

	// 6. Record in SQLite Database
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
