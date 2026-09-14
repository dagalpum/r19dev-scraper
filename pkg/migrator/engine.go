package migrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/jellyfin"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
	_ "modernc.org/sqlite"
)

var (
	marketingPrefixRegex = regexp.MustCompile(`(?i)^\[(?:limited quantity|fanza exclusive|dmm exclusive|pre-release|exclusive|special edition)\]\s*`)
	bonusSuffixRegex     = regexp.MustCompile(`(?i)\s*(?:\((?:blu-ray|dvd).*?\))?\s*(?:set of \d+.*|with polaroid.*|with photo.*|with bonus.*)?$`)
	reExtractID          = regexp.MustCompile(`(?i)\b([a-z]{2,6})[-_]?(\d{2,6})\b`)
	reZeroPadNum         = regexp.MustCompile(`^([a-z]+)(\d+)$`)
)

func cleanTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	t = marketingPrefixRegex.ReplaceAllString(t, "")
	if strings.Contains(strings.ToLower(t), "polaroid") || strings.Contains(strings.ToLower(t), "blu-ray disc") {
		t = bonusSuffixRegex.ReplaceAllString(t, "")
	}
	return jellyfin.SanitizeFilename(strings.TrimSpace(t))
}

func movePath(src, dst string) error {
	if src == dst {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)

	// Smart collision protection: if destination already exists, inspect sizes
	if fiDst, err := os.Stat(dst); err == nil && fiDst.Size() > 0 {
		fiSrc, sErr := os.Stat(src)
		if sErr == nil {
			if fiSrc.Size() == fiDst.Size() {
				return nil
			}
			ext := filepath.Ext(dst)
			base := strings.TrimSuffix(dst, ext)
			if fiSrc.Size() > fiDst.Size() {
				if !strings.HasSuffix(strings.ToLower(base), "-4k") {
					dst = base + "-4k" + ext
				} else {
					dst = base + "-v2" + ext
				}
			} else {
				dst = base + "-v2" + ext
			}
		}
	}

	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	return copyAndRemove(src, dst)
}

func copyAndRemove(src, dst string) error {
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
		return err
	}
	_ = in.Close()
	_ = out.Close()
	return os.Remove(src)
}

func moveDirContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dstDir, 0o755)
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if entry.IsDir() {
			_ = moveDirContents(srcPath, dstPath)
			_ = os.Remove(srcPath)
		} else {
			_ = movePath(srcPath, dstPath)
		}
	}
	return os.Remove(srcDir)
}

func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == io.EOF
}

func mergeFolderAssets(sourceDir, targetDir string) {
	if sourceDir == targetDir {
		return
	}
	targetPoster := filepath.Join(targetDir, "poster.jpg")
	if _, err := os.Stat(targetPoster); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(sourceDir, "folder.jpg")); err == nil {
			_ = movePath(filepath.Join(sourceDir, "folder.jpg"), targetPoster)
		} else if _, err := os.Stat(filepath.Join(sourceDir, "poster.jpg")); err == nil {
			_ = movePath(filepath.Join(sourceDir, "poster.jpg"), targetPoster)
		}
	} else {
		_ = os.Remove(filepath.Join(sourceDir, "folder.jpg"))
		_ = os.Remove(filepath.Join(sourceDir, "poster.jpg"))
	}

	targetFanart := filepath.Join(targetDir, "fanart.jpg")
	if _, err := os.Stat(targetFanart); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(sourceDir, "fanart.jpg")); err == nil {
			_ = movePath(filepath.Join(sourceDir, "fanart.jpg"), targetFanart)
		}
	} else {
		_ = os.Remove(filepath.Join(sourceDir, "fanart.jpg"))
	}

	targetExtra := filepath.Join(targetDir, "extrafanart")
	sourceExtra := filepath.Join(sourceDir, "extrafanart")
	if _, err := os.Stat(targetExtra); os.IsNotExist(err) {
		if _, err := os.Stat(sourceExtra); err == nil {
			_ = movePath(sourceExtra, targetExtra)
		}
	} else {
		if _, err := os.Stat(sourceExtra); err == nil {
			_ = moveDirContents(sourceExtra, targetExtra)
		}
	}

	entries, _ := os.ReadDir(sourceDir)
	for _, e := range entries {
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".nfo") || lower == "movie.html" {
			_ = os.Remove(filepath.Join(sourceDir, e.Name()))
		}
	}
}

func cleanDirIfEmpty(dir, stopRoot string) {
	if isDirEmpty(dir) {
		_ = os.Remove(dir)
		parent := filepath.Dir(dir)
		if isDirEmpty(parent) && parent != stopRoot && parent != filepath.Dir(stopRoot) {
			_ = os.Remove(parent)
		}
	}
}

func cleanEmptyTree(root string) int {
	var count int
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == root {
			return nil
		}
		if isDirEmpty(path) {
			if err := os.Remove(path); err == nil {
				count++
			}
		}
		return nil
	})
	return count
}

// Run executes the migration workflow.
func Run(ctx context.Context, cfg Config, eventCh chan<- ProgressEvent, confirmCh <-chan bool) (*Summary, error) {
	startTime := time.Now()
	summary := &Summary{}

	emit := func(e ProgressEvent) {
		if eventCh != nil {
			select {
			case eventCh <- e:
			case <-ctx.Done():
			}
		}
	}

	homeDir, _ := os.UserHomeDir()
	dumpDBPath := filepath.Join(homeDir, "Library", "Application Support", "r19dev", "r18_dump.db")
	dumpDB, err := sql.Open("sqlite", dumpDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open dump database: %w", err)
	}
	defer dumpDB.Close()

	appDBPath := filepath.Join(homeDir, "Library", "Application Support", "r19dev", "r19dev.db")
	appDB, err := sql.Open("sqlite", appDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open application database: %w", err)
	}
	defer appDB.Close()

	// 1. Load followed actresses
	followedMap := make(map[string]string)
	fRows, err := appDB.Query("SELECT name, ja_name FROM actresses WHERE followed_at IS NOT NULL")
	if err == nil {
		for fRows.Next() {
			var name, jaName sql.NullString
			_ = fRows.Scan(&name, &jaName)
			if name.Valid && strings.TrimSpace(name.String) != "" {
				followedMap[strings.ToLower(strings.TrimSpace(name.String))] = strings.TrimSpace(name.String)
			}
			if jaName.Valid && strings.TrimSpace(jaName.String) != "" {
				followedMap[strings.ToLower(strings.TrimSpace(jaName.String))] = strings.TrimSpace(name.String)
			}
		}
		fRows.Close()
	}

	// 2. Scan source directory with live streaming progress
	emit(ProgressEvent{
		Type:    EventScanStart,
		Message: fmt.Sprintf("Scanning %s...", cfg.SourceDir),
	})

	sc := scanner.New(nil)
	streamCh := make(chan []scanner.FileInfo, 50)
	var scanRes *scanner.ScanResult
	var scanErr error

	scanDone := make(chan struct{})
	go func() {
		scanRes, scanErr = sc.ScanStream(ctx, cfg.SourceDir, 2, streamCh)
		close(streamCh)
		close(scanDone)
	}()

	foundCount := 0
	for chunk := range streamCh {
		foundCount += len(chunk)
		latestFile := ""
		if len(chunk) > 0 {
			latestFile = chunk[len(chunk)-1].Name
		}
		emit(ProgressEvent{
			Type:    EventScanProgress,
			Current: foundCount,
			Message: fmt.Sprintf("Discovered %d video files...", foundCount),
			Detail:  latestFile,
		})
	}
	<-scanDone
	if scanErr != nil {
		return nil, fmt.Errorf("scan error: %w", scanErr)
	}

	summary.TotalDiscovered = len(scanRes.Files)
	emit(ProgressEvent{
		Type:    EventScanDone,
		Total:   len(scanRes.Files),
		Message: fmt.Sprintf("Scanning complete: Discovered %d video files", len(scanRes.Files)),
	})

	m, _ := matcher.New(nil)

	// =========================================================================
	// PASS 1: Pre-flight Check & Mapping (ตรวจสอบและจัดทำแผนผังทั้งหมดล่วงหน้า)
	// =========================================================================
	emit(ProgressEvent{
		Type:    EventPlanItem,
		Message: "Validating and mapping destination paths...",
	})

	type PlannedItem struct {
		File            scanner.FileInfo
		MovieID         string
		NormID          string
		TitleEn         string
		TitleJa         string
		CleanTitle      string
		Maker           string
		ReleaseDate     string
		CoverURL        string
		Actresses       []scraper.Actress
		ActressDir      string
		TargetDir       string
		TargetVideo     string
		TargetVideoName string
		SourceDir       string
		IsDuplicate     bool
		IsMultiPart     bool
		PartNumber      int
	}

	var plannedItems []*PlannedItem

	for idx, fi := range scanRes.Files {
		select {
		case <-ctx.Done():
			return summary, ctx.Err()
		default:
		}

		currentNum := idx + 1

		// 1. Check rogue orphan file
		if fi.Name == ".mp4" && filepath.Dir(fi.Path) == filepath.Join(cfg.SourceDir, "@Unknown") {
			summary.SkippedCount++
			skipMsg := fmt.Sprintf("Skipping unidentifiable orphan file: %s", fi.Path)
			summary.SkippedDetails = append(summary.SkippedDetails, skipMsg)
			emit(ProgressEvent{
				Type:       EventMoveSkip,
				Current:    currentNum,
				Total:      len(scanRes.Files),
				SourcePath: fi.Path,
				Message:    skipMsg,
			})
			continue
		}

		// 2. Match JAV ID
		mr := m.MatchFile(fi)
		if mr == nil || mr.ID == "" {
			summary.SkippedCount++
			skipMsg := fmt.Sprintf("Could not extract JAV ID from %s", fi.Path)
			summary.SkippedDetails = append(summary.SkippedDetails, skipMsg)
			emit(ProgressEvent{
				Type:       EventMoveSkip,
				Current:    currentNum,
				Total:      len(scanRes.Files),
				SourcePath: fi.Path,
				Message:    skipMsg,
			})
			continue
		}

		normID := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mr.ID, "-", ""), "_", ""))
		if matches := reZeroPadNum.FindStringSubmatch(normID); len(matches) == 3 {
			numPart := matches[2]
			if len(numPart) < 5 {
				normID = fmt.Sprintf("%s%05s", matches[1], numPart)
			}
		}

		// 3. Look up metadata in dump DB
		var contentID, dvdID, titleEn, titleJa, maker, relDate, jacketURL string
		err := dumpDB.QueryRow(`
			SELECT content_id, dvd_id, title_en, title_ja, maker_name_en, release_date, jacket_full_url 
			FROM r18_movies 
			WHERE clean_id = ? OR dvd_id = ? OR content_id = ?
			LIMIT 1`, normID, mr.ID, normID).Scan(&contentID, &dvdID, &titleEn, &titleJa, &maker, &relDate, &jacketURL)

		if err != nil {
			unpaddedID := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mr.ID, "-", ""), "_", ""))
			_ = dumpDB.QueryRow(`
				SELECT content_id, dvd_id, title_en, title_ja, maker_name_en, release_date, jacket_full_url 
				FROM r18_movies 
				WHERE clean_id = ? OR dvd_id = ?
				LIMIT 1`, unpaddedID, mr.ID).Scan(&contentID, &dvdID, &titleEn, &titleJa, &maker, &relDate, &jacketURL)
		}

		// Fallback to appDB if not found in dumpDB
		if dvdID == "" {
			_ = appDB.QueryRow(`
				SELECT id, title, original_title, maker, release_date, cover_url 
				FROM movies 
				WHERE UPPER(id) = UPPER(?)
				LIMIT 1`, mr.ID).Scan(&dvdID, &titleEn, &titleJa, &maker, &relDate, &jacketURL)
		}

		// Fetch actresses
		var actList []scraper.Actress
		if contentID != "" {
			actRows, aErr := dumpDB.Query(`
				SELECT a.name_romaji, a.name_kanji, a.image_url 
				FROM video_actresses va
				JOIN actresses a ON va.actress_id = a.id
				WHERE va.content_id = ?`, contentID)
			if aErr == nil {
				for actRows.Next() {
					var romaji, kanji, img sql.NullString
					_ = actRows.Scan(&romaji, &kanji, &img)
					name := strings.TrimSpace(romaji.String)
					kanjiStr := strings.TrimSpace(kanji.String)
					if name == "" {
						if known, ok := knownActressRomajiMap[kanjiStr]; ok {
							name = known
						} else {
							name = kanjiStr
						}
					}
					if name != "" {
						actList = append(actList, scraper.Actress{
							Name:     name,
							JaName:   kanjiStr,
							ImageURL: strings.TrimSpace(img.String),
						})
					}
				}
				actRows.Close()
			}
		}

		if len(actList) == 0 {
			var actJSON sql.NullString
			if err := appDB.QueryRow("SELECT actresses_json FROM movies WHERE UPPER(id) = UPPER(?)", mr.ID).Scan(&actJSON); err == nil && actJSON.Valid {
				_ = json.Unmarshal([]byte(actJSON.String), &actList)
			}
		}

		// Determine target actress directory
		actressDir := "Unknown Actress"
		foundFollowed := false
		for _, act := range actList {
			if canonical, ok := followedMap[strings.ToLower(act.Name)]; ok {
				actressDir = canonical
				foundFollowed = true
				break
			}
			if act.JaName != "" {
				if canonical, ok := followedMap[strings.ToLower(act.JaName)]; ok {
					actressDir = canonical
					foundFollowed = true
					break
				}
			}
		}

		if !foundFollowed {
			rel, _ := filepath.Rel(cfg.SourceDir, fi.Path)
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 2 {
				parentAct := parts[0]
				if parentAct != "@Group" && parentAct != "@Unknown" && strings.TrimSpace(parentAct) != "" {
					if canonical, ok := followedMap[strings.ToLower(parentAct)]; ok {
						actressDir = canonical
						foundFollowed = true
					} else if len(actList) > 0 && isPureASCII(actList[0].Name) {
						actressDir = strings.TrimSpace(actList[0].Name)
					} else {
						actressDir = parentAct
					}
				}
			}
			if !foundFollowed && len(actList) > 0 && strings.TrimSpace(actList[0].Name) != "" {
				actressDir = strings.TrimSpace(actList[0].Name)
			}
		}

		if known, ok := knownActressRomajiMap[actressDir]; ok {
			actressDir = known
		}

		actressDir = jellyfin.SanitizeFilename(actressDir)

		cTitle := cleanTitle(titleEn)
		if cTitle == "" {
			cTitle = cleanTitle(titleJa)
		}

		movieFolder := mr.ID
		if cTitle != "" {
			movieFolder = jellyfin.SanitizeFilename(fmt.Sprintf("%s %s", mr.ID, cTitle))
		}

		targetDir := filepath.Join(cfg.DestRoot, actressDir, movieFolder)
		ext := filepath.Ext(fi.Path)
		lowerName := strings.ToLower(fi.Name)
		is4K := strings.Contains(lowerName, "-4k") || strings.Contains(lowerName, "_4k")
		isUncen := strings.Contains(lowerName, "uncensored") || strings.Contains(lowerName, " u.") || strings.Contains(lowerName, "_uncen")

		var targetVideoName string
		if mr.IsMultiPart && mr.PartNumber > 0 {
			targetVideoName = fmt.Sprintf("%s-cd%d%s", mr.ID, mr.PartNumber, ext)
		} else if is4K {
			targetVideoName = fmt.Sprintf("%s-4k%s", mr.ID, ext)
		} else if isUncen {
			targetVideoName = fmt.Sprintf("%s-uncensored%s", mr.ID, ext)
		} else {
			targetVideoName = fmt.Sprintf("%s%s", mr.ID, ext)
		}
		targetVideo := filepath.Join(targetDir, targetVideoName)
		sourceDir := filepath.Dir(fi.Path)

		// Check duplicate at destination
		isDup := false
		if fiTarget, err := os.Stat(targetVideo); err == nil && fiTarget.Size() > 0 && targetVideo != fi.Path {
			isDup = true
			summary.DuplicateCount++
			dupMsg := fmt.Sprintf("Destination has existing copy: %s (size %d bytes)", mr.ID, fiTarget.Size())
			emit(ProgressEvent{
				Type:       EventMoveDuplicate,
				Current:    currentNum,
				Total:      len(scanRes.Files),
				MovieID:    mr.ID,
				Actress:    actressDir,
				Title:      cTitle,
				SourcePath: fi.Path,
				TargetPath: targetVideo,
				Message:    dupMsg,
			})
		}

		item := &PlannedItem{
			File:            fi,
			MovieID:         mr.ID,
			NormID:          normID,
			TitleEn:         titleEn,
			TitleJa:         titleJa,
			CleanTitle:      cTitle,
			Maker:           maker,
			ReleaseDate:     relDate,
			CoverURL:        jacketURL,
			Actresses:       actList,
			ActressDir:      actressDir,
			TargetDir:       targetDir,
			TargetVideo:     targetVideo,
			TargetVideoName: targetVideoName,
			SourceDir:       sourceDir,
			IsDuplicate:     isDup,
			IsMultiPart:     mr.IsMultiPart,
			PartNumber:      mr.PartNumber,
		}
		plannedItems = append(plannedItems, item)

		if cfg.DryRun {
			summary.OrganizedCount++
			emit(ProgressEvent{
				Type:       EventPlanItem,
				Current:    currentNum,
				Total:      len(scanRes.Files),
				MovieID:    mr.ID,
				Actress:    actressDir,
				Title:      cTitle,
				SourcePath: fi.Path,
				TargetPath: targetVideo,
				Message:    fmt.Sprintf("[DRY-RUN] %s -> %s", mr.ID, targetDir),
			})
		}
	}

	// Save pre-flight plan report to local file for reference
	planReportPath := filepath.Join(cfg.DestRoot, ".migration_plan.json")
	if planData, err := json.MarshalIndent(plannedItems, "", "  "); err == nil {
		_ = os.WriteFile(planReportPath, planData, 0o644)
	}

	// In Dry-Run mode, planning is complete, no disk changes performed
	if cfg.DryRun {
		summary.Duration = time.Since(startTime)
		emit(ProgressEvent{
			Type:        EventDone,
			SummaryData: summary,
			Message:     "Dry-run planning completed successfully! No files modified.",
		})
		return summary, nil
	}

	// If interactive confirmation is enabled (not AutoConfirm)
	if !cfg.AutoConfirm {
		emit(ProgressEvent{
			Type:        EventConfirmReady,
			Current:     len(plannedItems),
			Total:       len(plannedItems),
			SummaryData: summary,
			Message:     fmt.Sprintf("Plan verified: %d movies ready to organize. Awaiting your confirmation...", len(plannedItems)),
		})

		if confirmCh != nil {
			select {
			case ok := <-confirmCh:
				if !ok {
					summary.Duration = time.Since(startTime)
					emit(ProgressEvent{
						Type:        EventDone,
						SummaryData: summary,
						Message:     "Migration cancelled by user. No files modified.",
					})
					return summary, nil
				}
			case <-ctx.Done():
				return summary, ctx.Err()
			}
		}
	}

	// =========================================================================
	// PASS 2: Execution (ลงมือย้ายไฟล์จริงและสร้าง Metadata ตามแผนที่ตรวจสอบแล้ว)
	// =========================================================================
	totalPlans := len(plannedItems)
	for i, item := range plannedItems {
		select {
		case <-ctx.Done():
			return summary, ctx.Err()
		default:
		}

		currentNum := i + 1

		if item.IsDuplicate {
			// Check if incoming duplicate file is a higher quality/larger version (e.g. 4K vs 1080p)
			if fiDst, err := os.Stat(item.TargetVideo); err == nil && fiDst.Size() > 0 {
				fiSrc, sErr := os.Stat(item.File.Path)
				if sErr == nil && fiSrc.Size() > fiDst.Size() {
					ext := filepath.Ext(item.TargetVideo)
					base := strings.TrimSuffix(item.TargetVideo, ext)
					newTarget := base + "-4k" + ext
					_ = os.Rename(item.File.Path, newTarget)
					item.TargetVideo = newTarget
					mergeFolderAssets(item.SourceDir, item.TargetDir)
					cleanDirIfEmpty(item.SourceDir, cfg.SourceDir)
					continue
				}
			}
			mergeFolderAssets(item.SourceDir, item.TargetDir)
			_ = os.Remove(item.File.Path)
			cleanDirIfEmpty(item.SourceDir, cfg.SourceDir)
			continue
		}

		emit(ProgressEvent{
			Type:       EventMoveStart,
			Current:    currentNum,
			Total:      totalPlans,
			MovieID:    item.MovieID,
			Actress:    item.ActressDir,
			Title:      item.CleanTitle,
			SourcePath: item.File.Path,
			TargetPath: item.TargetVideo,
			Message:    fmt.Sprintf("Moving %s -> %s", item.MovieID, item.ActressDir),
		})

		if err := os.MkdirAll(item.TargetDir, 0o755); err != nil {
			summary.ErrorCount++
			errMsg := fmt.Sprintf("mkdir %s error: %v", item.TargetDir, err)
			summary.Errors = append(summary.Errors, errMsg)
			emit(ProgressEvent{
				Type:    EventMoveError,
				Current: currentNum,
				Total:   totalPlans,
				MovieID: item.MovieID,
				Message: errMsg,
				Err:     err,
			})
			continue
		}

		// 1. Move video file
		if err := movePath(item.File.Path, item.TargetVideo); err != nil {
			summary.ErrorCount++
			errMsg := fmt.Sprintf("move %s error: %v", item.MovieID, err)
			summary.Errors = append(summary.Errors, errMsg)
			emit(ProgressEvent{
				Type:    EventMoveError,
				Current: currentNum,
				Total:   totalPlans,
				MovieID: item.MovieID,
				Message: errMsg,
				Err:     err,
			})
			continue
		}

		// 2. Move existing image assets
		mergeFolderAssets(item.SourceDir, item.TargetDir)

		// 3. Build Movie metadata object
		movie := &scraper.Movie{
			ID:            item.MovieID,
			CombinedID:    item.NormID,
			Title:         item.TitleEn,
			OriginalTitle: item.TitleJa,
			Maker:         item.Maker,
			ReleaseDate:   item.ReleaseDate,
			CoverURL:      item.CoverURL,
			Actresses:     item.Actresses,
			ScrapedAt:     time.Now(),
		}
		if movie.Title == "" {
			movie.Title = movie.OriginalTitle
		}

		// 4. Write Jellyfin NFO
		nfoPath := filepath.Join(item.TargetDir, fmt.Sprintf("%s.nfo", item.MovieID))
		_ = jellyfin.WriteNFO(movie, nil, nfoPath)

		// 5. Write Movie.html with Cinematic template
		htmlPath := filepath.Join(item.TargetDir, "movie.html")
		_ = jellyfin.WriteHTML(movie, nil, htmlPath, item.TargetVideoName)

		// 6. Register into r19dev.db
		actBytes, _ := json.Marshal(item.Actresses)
		var genList []string
		genBytes, _ := json.Marshal(genList)
		var shotList []string
		shotBytes, _ := json.Marshal(shotList)

		nowStr := time.Now().Format("2006-01-02 15:04:05")
		_, _ = appDB.Exec(`
			INSERT INTO movies (id, combined_id, title, original_title, maker, release_date, cover_url, actresses_json, genres_json, screenshots_json, scraped_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET 
				title=coalesce(nullif(excluded.title, ''), movies.title),
				original_title=coalesce(nullif(excluded.original_title, ''), movies.original_title),
				maker=coalesce(nullif(excluded.maker, ''), movies.maker),
				release_date=coalesce(nullif(excluded.release_date, ''), movies.release_date),
				cover_url=coalesce(nullif(excluded.cover_url, ''), movies.cover_url),
				actresses_json=coalesce(nullif(excluded.actresses_json, ''), movies.actresses_json)`,
			item.MovieID, item.NormID, movie.Title, movie.OriginalTitle, movie.Maker, movie.ReleaseDate, movie.CoverURL,
			string(actBytes), string(genBytes), string(shotBytes), nowStr)

		_, _ = appDB.Exec(`
			INSERT INTO organized_movies (movie_id, target_folder, target_video, organized_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(movie_id) DO UPDATE SET target_folder=excluded.target_folder, target_video=excluded.target_video, organized_at=excluded.organized_at`,
			item.MovieID, item.TargetDir, item.TargetVideo, nowStr)

		_, _ = appDB.Exec(`
			INSERT INTO library_files (file_path, movie_id, size_bytes, is_multi_part, part_number, organized_path, scanned_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(file_path) DO UPDATE SET movie_id=excluded.movie_id, size_bytes=excluded.size_bytes, organized_path=excluded.organized_path, scanned_at=excluded.scanned_at`,
			item.TargetVideo, item.MovieID, item.File.Size, item.IsMultiPart, item.PartNumber, item.TargetDir, nowStr)

		cleanDirIfEmpty(item.SourceDir, cfg.SourceDir)

		summary.OrganizedCount++
		emit(ProgressEvent{
			Type:       EventMoveDone,
			Current:    currentNum,
			Total:      totalPlans,
			MovieID:    item.MovieID,
			Actress:    item.ActressDir,
			Title:      item.CleanTitle,
			SourcePath: item.File.Path,
			TargetPath: item.TargetVideo,
			Message:    fmt.Sprintf("Organized %s -> %s", item.MovieID, item.TargetDir),
		})
	}

	// 4. Update existing movie.html if requested
	if !cfg.DryRun && cfg.UpdateExisting {
		emit(ProgressEvent{
			Type:    EventUpdateHTML,
			Message: "Scanning destination for existing movie.html files...",
		})
		summary.UpdatedHTMLNum = UpgradeHTMLFiles(ctx, cfg.DestRoot, appDB, dumpDB, emit)
	}

	// 5. Clean empty source tree
	if !cfg.DryRun {
		emit(ProgressEvent{
			Type:    EventCleanArchive,
			Message: "Cleaning empty directories in source...",
		})
		cleaned := cleanEmptyTree(cfg.SourceDir)
		if cleaned > 0 {
			emit(ProgressEvent{
				Type:    EventCleanArchive,
				Message: fmt.Sprintf("Cleaned %d empty directories in source", cleaned),
			})
		}
	}

	summary.Duration = time.Since(startTime)
	emit(ProgressEvent{
		Type:        EventDone,
		SummaryData: summary,
		Message:     "Migration completed successfully!",
	})

	return summary, nil
}

type htmlUpgradeTarget struct {
	folder      string
	movieID     string
	targetVideo string
}

// UpgradeHTMLFiles upgrades all movie.html files in destRoot using a concurrent worker pool.
func UpgradeHTMLFiles(ctx context.Context, destRoot string, appDB, dumpDB *sql.DB, emit func(ProgressEvent)) int {
	var targets []htmlUpgradeTarget

	// 1. Fast discovery from organized_movies database (instant, no recursive network SMB walk)
	if appDB != nil {
		rows, err := appDB.QueryContext(ctx, `
			SELECT target_folder, movie_id, target_video 
			FROM organized_movies 
			WHERE target_folder LIKE ?`, destRoot+"%")
		if err == nil {
			for rows.Next() {
				var folder, movieID, video string
				if err := rows.Scan(&folder, &movieID, &video); err == nil && folder != "" {
					targets = append(targets, htmlUpgradeTarget{
						folder:      folder,
						movieID:     movieID,
						targetVideo: video,
					})
				}
			}
			rows.Close()
		}
	}

	// 2. Fallback to filesystem walk if database has no records for this destination
	if len(targets) == 0 {
		_ = filepath.Walk(destRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			htmlPath := filepath.Join(path, "movie.html")
			if _, statErr := os.Stat(htmlPath); statErr != nil {
				return nil
			}

			dirName := filepath.Base(path)
			m := reExtractID.FindStringSubmatch(dirName)
			var movieID string
			if len(m) >= 3 {
				movieID = fmt.Sprintf("%s-%s", strings.ToUpper(m[1]), m[2])
			}

			if movieID == "" {
				entries, _ := os.ReadDir(path)
				for _, e := range entries {
					if strings.HasSuffix(strings.ToLower(e.Name()), ".nfo") {
						sub := reExtractID.FindStringSubmatch(e.Name())
						if len(sub) >= 3 {
							movieID = fmt.Sprintf("%s-%s", strings.ToUpper(sub[1]), sub[2])
							break
						}
					}
				}
			}

			if movieID != "" {
				targets = append(targets, htmlUpgradeTarget{
					folder:  path,
					movieID: movieID,
				})
			}
			return nil
		})
	}

	total := len(targets)
	if total == 0 {
		return 0
	}

	if emit != nil {
		emit(ProgressEvent{
			Type:    EventUpdateHTML,
			Current: 0,
			Total:   total,
			Message: fmt.Sprintf("Found %d movies. Upgrading movie.html to Cinematic template (16 concurrent workers)...", total),
		})
	}

	workers := 16
	if workers > total {
		workers = total
	}

	jobs := make(chan htmlUpgradeTarget, total)
	for _, t := range targets {
		jobs <- t
	}
	close(jobs)

	var processed int32
	var successCount int32
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				movieID := target.movieID
				var title, origTitle, maker, label, director, relDate, coverURL string
				var actJSON, genJSON sql.NullString

				if appDB != nil {
					_ = appDB.QueryRowContext(ctx, `
						SELECT title, original_title, maker, label, director, release_date, cover_url, actresses_json, genres_json
						FROM movies 
						WHERE UPPER(id) = UPPER(?)
						LIMIT 1`, movieID).Scan(&title, &origTitle, &maker, &label, &director, &relDate, &coverURL, &actJSON, &genJSON)
				}

				movie := &scraper.Movie{
					ID:            movieID,
					Title:         title,
					OriginalTitle: origTitle,
					Maker:         maker,
					Label:         label,
					Director:      director,
					ReleaseDate:   relDate,
					CoverURL:      coverURL,
				}

				if actJSON.Valid {
					_ = json.Unmarshal([]byte(actJSON.String), &movie.Actresses)
				}
				if genJSON.Valid {
					_ = json.Unmarshal([]byte(genJSON.String), &movie.Genres)
				}

				if movie.Title == "" && dumpDB != nil {
					normID := strings.ToLower(strings.ReplaceAll(movieID, "-", ""))
					var dID, tEn, tJa, mName, rDate, jURL string
					_ = dumpDB.QueryRowContext(ctx, `
						SELECT dvd_id, title_en, title_ja, maker_name_en, release_date, jacket_full_url 
						FROM r18_movies WHERE clean_id = ? OR dvd_id = ? LIMIT 1`, normID, movieID).Scan(&dID, &tEn, &tJa, &mName, &rDate, &jURL)
					if tEn != "" || tJa != "" {
						movie.Title = tEn
						movie.OriginalTitle = tJa
						movie.Maker = mName
						movie.ReleaseDate = rDate
						movie.CoverURL = jURL
					}
				}

				var videoBase string
				if target.targetVideo != "" {
					videoBase = filepath.Base(target.targetVideo)
				}
				htmlPath := filepath.Join(target.folder, "movie.html")
				if err := jellyfin.WriteHTML(movie, nil, htmlPath, videoBase); err == nil {
					atomic.AddInt32(&successCount, 1)
				}

				curr := int(atomic.AddInt32(&processed, 1))
				if emit != nil {
					firstAct := ""
					if len(movie.Actresses) > 0 {
						firstAct = movie.Actresses[0].Name
					}
					emit(ProgressEvent{
						Type:       EventUpdateHTML,
						Current:    curr,
						Total:      total,
						MovieID:    movieID,
						Actress:    firstAct,
						Title:      movie.Title,
						Message:    fmt.Sprintf("[%d/%d] Upgraded %s movie.html", curr, total, movieID),
					})
				}
			}
		}()
	}

	wg.Wait()
	return int(successCount)
}

var knownActressRomajiMap = map[string]string{
	"入田真綾":  "Maaya Irita",
	"佐野ゆま":  "Yuma Sano",
	"五日市芽依": "Mei Itsukaichi",
	"仁藤さや香": "Sayaka Nito",
	"乃坂ひより": "Hiyori Nozaka",
	"凪ひかる":  "Hikaru Nagi",
	"彩月七緒":  "Nao Satsuki",
	"日向かえで": "Kaede Hinata",
	"日向かえで": "Kaede Hinata",
	"月妃さら":  "Sara Tsukihi",
	"逢坂希穂":  "Kiho Ousaka",
	"日向陽葵":  "Himari Hinata",
	"中村彩":   "Sayaka Nakamura",
	"神谷のこ":  "Noko Kamiya",
	"八蜜凛":   "Rin Hachimitsu",
	"北島愛菜":  "Aina Kitajima",
	"小鳥遊もえ": "Moe Takanashi",
	"日向理亜":  "Ria Hinata",
	"清水こなつ": "Konatsu Shimizu",
	"角奈保":   "Naho Sumi",
}

func isPureASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
