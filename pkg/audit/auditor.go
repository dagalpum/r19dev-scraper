package audit

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/jellyfin"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

const (
	MinPosterBytes     = 25 * 1024        // 25 KB
	MinFanartBytes     = 25 * 1024        // 25 KB
	MinScreenshotBytes = 15 * 1024        // 15 KB
	MinNFOBytes        = 80               // 80 Bytes
	MinHTMLBytes       = 150              // 150 Bytes
	MinVideoBytes      = 30 * 1024 * 1024 // 30 MB
)

// MovieAuditStatus represents completeness state of a movie.
type MovieAuditStatus string

const (
	StatusComplete   MovieAuditStatus = "complete"   // All standard assets present
	StatusIncomplete MovieAuditStatus = "incomplete" // Missing some metadata/assets but has video
	StatusNoVideo    MovieAuditStatus = "no_video"   // Folder has no video file
)

// MovieAudit contains the checklist status for a single organized movie folder.
type MovieAudit struct {
	FolderPath      string           `json:"folder_path"`
	RelativePath    string           `json:"relative_path"`
	ActressName     string           `json:"actress_name"`
	MovieID         string           `json:"movie_id"`
	FolderTitle     string           `json:"folder_title"`
	Status          MovieAuditStatus `json:"status"`

	// Video check
	HasVideo        bool     `json:"has_video"`
	VideoFiles      []string `json:"video_files"`
	VideoTotalBytes int64    `json:"video_total_bytes"`

	// Metadata & HTML
	HasNFO   bool   `json:"has_nfo"`
	NFOPath  string `json:"nfo_path"`
	HasHTML  bool   `json:"has_html"`
	HTMLPath string `json:"html_path"`

	// Images
	HasPoster  bool   `json:"has_poster"`
	PosterPath string `json:"poster_path"`
	HasFanart  bool   `json:"has_fanart"`
	FanartPath string `json:"fanart_path"`

	// Screenshots
	ScreenshotsCount int  `json:"screenshots_count"`
	HasScreenshots   bool `json:"has_screenshots"`

	// Database Sync
	InDB bool `json:"in_db"`

	// Media details & Dimensions
	PosterDimensions string `json:"poster_dimensions,omitempty"`
	PosterBytes      int64  `json:"poster_bytes,omitempty"`
	FanartDimensions string `json:"fanart_dimensions,omitempty"`
	FanartBytes      int64  `json:"fanart_bytes,omitempty"`
	IsMultipart      bool   `json:"is_multipart,omitempty"`
	MultiPartCount   int    `json:"multi_part_count,omitempty"`

	// Health & Quality issues
	Warnings       []string `json:"warnings,omitempty"`
	CorruptedFiles []string `json:"corrupted_files,omitempty"`

	// Missing summary
	MissingItems []string `json:"missing_items"`

	// Scraped Movie metadata (populated when loaded)
	MovieMeta *scraper.Movie `json:"movie_meta,omitempty"`
}

// ProgressEventType represents the stage of auditing.
type ProgressEventType string

const (
	ProgressDiscoveringActresses ProgressEventType = "discovering_actresses"
	ProgressDiscoveringMovies    ProgressEventType = "discovering_movies"
	ProgressInspecting           ProgressEventType = "inspecting"
	ProgressFixing               ProgressEventType = "fixing"
	ProgressDone                 ProgressEventType = "done"
)

// AuditProgressEvent contains fine-grained live progress information.
type AuditProgressEvent struct {
	Type ProgressEventType

	// Actress information
	TotalActresses     int
	CurrentActressIdx  int
	CurrentActress     string
	ActressesRemaining int

	// Actress-specific movie counts
	ActressTotalMovies     int
	ActressCurrentMovieIdx int
	ActressMoviesRemaining int

	// Overall movie counts
	TotalMovies     int
	CurrentMovieIdx int
	MoviesRemaining int
	CurrentMovieID  string
	CurrentTitle    string

	// Live statistics
	CompleteCount   int
	IncompleteCount int
	NoVideoCount    int

	// Speed & Time
	Elapsed time.Duration
	Speed   float64 // movies per second
	ETA     time.Duration

	// Status & Details
	Message string
	Detail  string
	Item    *MovieAudit
}

// FixProgressEvent contains real-time step information during single or batch repair.
type FixProgressEvent struct {
	MovieID     string
	Actress     string
	StepIndex   int
	TotalSteps  int
	StepName    string
	Message     string
	Detail      string
	CurrentFile int
	TotalFiles  int
	Err         error
}

// Auditor handles scanning directories and auditing movie folders.
type Auditor struct {
	dbInstance *db.DB
	matcher    *matcher.Matcher
	scraper    *scraper.Client
}

// New creates a new Auditor instance.
func New(d *db.DB) (*Auditor, error) {
	m, err := matcher.New(nil)
	if err != nil {
		return nil, err
	}
	if d == nil {
		d, _ = db.Default()
	}
	return &Auditor{
		dbInstance: d,
		matcher:    m,
		scraper:    scraper.NewClient(15 * time.Second),
	}, nil
}

type folderTask struct {
	path               string
	actressName        string
	actressIdx         int
	totalActresses     int
	actressTotalMovies int
	actressMovieIdx    int
}

// AuditDirectoryWithProgress scans and audits movie folders while streaming real-time fine-grained progress.
func (a *Auditor) AuditDirectoryWithProgress(ctx context.Context, rootDir string, progressChan chan<- AuditProgressEvent) ([]*MovieAudit, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("invalid root directory %s: %w", rootDir, err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("cannot access directory %s: %w", absRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absRoot)
	}

	startTime := time.Now()

	// 1. Discover top-level folders (Actresses)
	if progressChan != nil {
		progressChan <- AuditProgressEvent{
			Type:    ProgressDiscoveringActresses,
			Message: fmt.Sprintf("🔍 Discovering actress directories in %s...", filepath.Base(absRoot)),
		}
	}

	topEntries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, err
	}

	type actressGroup struct {
		name    string
		path    string
		folders []string
	}

	var actressList []actressGroup
	var directMovieFolders []string

	for _, entry := range topEntries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "@eaDir" || name == "#recycle" {
			continue
		}

		fullPath := filepath.Join(absRoot, name)

		// Check if top-level directory is directly a movie folder
		if res := a.matcher.MatchFile(scanner.FileInfo{Name: name, Path: fullPath}); res != nil && res.ID != "" {
			directMovieFolders = append(directMovieFolders, fullPath)
			continue
		}

		// Otherwise it is an actress folder
		actressList = append(actressList, actressGroup{
			name: name,
			path: fullPath,
		})
	}

	sort.Slice(actressList, func(i, j int) bool {
		return actressList[i].name < actressList[j].name
	})

	totalActresses := len(actressList)
	if len(directMovieFolders) > 0 {
		totalActresses++
	}

	// 2. Discover movie folders per actress
	var tasks []folderTask
	totalMoviesCount := 0

	for aIdx, act := range actressList {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if progressChan != nil {
			progressChan <- AuditProgressEvent{
				Type:               ProgressDiscoveringMovies,
				TotalActresses:     totalActresses,
				CurrentActressIdx:  aIdx + 1,
				CurrentActress:     act.name,
				ActressesRemaining: totalActresses - (aIdx + 1),
				TotalMovies:        totalMoviesCount,
				Message:            fmt.Sprintf("🔍 [%d/%d] Scanning %s's works...", aIdx+1, totalActresses, act.name),
			}
		}

		subEntries, sErr := os.ReadDir(act.path)
		if sErr != nil {
			continue
		}

		var movieDirs []string
		for _, sub := range subEntries {
			if !sub.IsDir() {
				// Check loose video file directly in actress folder
				if isVideoFile(sub.Name()) {
					movieDirs = append(movieDirs, act.path)
					break
				}
				continue
			}
			subName := sub.Name()
			if strings.HasPrefix(subName, ".") || subName == "extrafanart" || subName == "screenshots" {
				continue
			}
			movieDirs = append(movieDirs, filepath.Join(act.path, subName))
		}

		sort.Strings(movieDirs)
		actressTotal := len(movieDirs)
		totalMoviesCount += actressTotal

		for mIdx, mDir := range movieDirs {
			tasks = append(tasks, folderTask{
				path:               mDir,
				actressName:        act.name,
				actressIdx:         aIdx + 1,
				totalActresses:     totalActresses,
				actressTotalMovies: actressTotal,
				actressMovieIdx:    mIdx + 1,
			})
		}
	}

	// Add direct top-level movies if any
	if len(directMovieFolders) > 0 {
		sort.Strings(directMovieFolders)
		dirTotal := len(directMovieFolders)
		totalMoviesCount += dirTotal
		for mIdx, mDir := range directMovieFolders {
			tasks = append(tasks, folderTask{
				path:               mDir,
				actressName:        "Unknown",
				actressIdx:         totalActresses,
				totalActresses:     totalActresses,
				actressTotalMovies: dirTotal,
				actressMovieIdx:    mIdx + 1,
			})
		}
	}

	// If no actress subfolders were found, fallback to recursive WalkDir
	if len(tasks) == 0 {
		var found []string
		_ = filepath.WalkDir(absRoot, func(p string, d os.DirEntry, wErr error) error {
			if wErr != nil || ctx.Err() != nil {
				return nil
			}
			n := d.Name()
			if strings.HasPrefix(n, ".") || n == "extrafanart" || n == "screenshots" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() && p != absRoot {
				if res := a.matcher.MatchFile(scanner.FileInfo{Name: n, Path: p}); res != nil && res.ID != "" {
					found = append(found, p)
					return filepath.SkipDir
				}
			} else if !d.IsDir() && isVideoFile(n) {
				found = append(found, filepath.Dir(p))
			}
			return nil
		})

		foundMap := make(map[string]bool)
		for _, f := range found {
			if !foundMap[f] {
				foundMap[f] = true
				tasks = append(tasks, folderTask{
					path:               f,
					actressName:        filepath.Base(filepath.Dir(f)),
					actressIdx:         1,
					totalActresses:     1,
					actressTotalMovies: len(found),
					actressMovieIdx:    len(tasks) + 1,
				})
			}
		}
		totalMoviesCount = len(tasks)
	}

	// 3. Concurrently inspect movie folders with real-time reporting
	var mu sync.Mutex
	results := make([]*MovieAudit, 0, len(tasks))

	var completedCount int32
	var completeCount int32
	var incompleteCount int32
	var noVideoCount int32

	totalTasks := len(tasks)
	workerCount := runtime.NumCPU() * 2
	if workerCount < 8 {
		workerCount = 8
	}
	if workerCount > 24 {
		workerCount = 24
	}
	if totalTasks < workerCount {
		workerCount = totalTasks
	}
	if workerCount < 1 {
		workerCount = 1
	}

	taskChan := make(chan folderTask, totalTasks)
	for _, t := range tasks {
		taskChan <- t
	}
	close(taskChan)

	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskChan {
				if ctx.Err() != nil {
					return
				}

				audit, err := a.InspectFolder(ctx, t.path, absRoot)
				if err != nil || audit == nil {
					continue
				}

				// If ActressName was empty, enrich from task
				if audit.ActressName == "" && t.actressName != "" && t.actressName != "Unknown" {
					audit.ActressName = t.actressName
				}

				mu.Lock()
				results = append(results, audit)
				mu.Unlock()

				curCompleted := int(atomic.AddInt32(&completedCount, 1))

				switch audit.Status {
				case StatusComplete:
					atomic.AddInt32(&completeCount, 1)
				case StatusIncomplete:
					atomic.AddInt32(&incompleteCount, 1)
				case StatusNoVideo:
					atomic.AddInt32(&noVideoCount, 1)
				}

				if progressChan != nil {
					elapsed := time.Since(startTime)
					speed := 0.0
					if elapsed.Seconds() > 0 {
						speed = float64(curCompleted) / elapsed.Seconds()
					}
					var eta time.Duration
					if speed > 0 {
						rem := totalTasks - curCompleted
						eta = time.Duration(float64(rem)/speed) * time.Second
					}

					progressChan <- AuditProgressEvent{
						Type:                   ProgressInspecting,
						TotalActresses:         t.totalActresses,
						CurrentActressIdx:      t.actressIdx,
						CurrentActress:         t.actressName,
						ActressesRemaining:     t.totalActresses - t.actressIdx,
						ActressTotalMovies:     t.actressTotalMovies,
						ActressCurrentMovieIdx: t.actressMovieIdx,
						ActressMoviesRemaining: t.actressTotalMovies - t.actressMovieIdx,
						TotalMovies:            totalTasks,
						CurrentMovieIdx:        curCompleted,
						MoviesRemaining:        totalTasks - curCompleted,
						CurrentMovieID:         audit.MovieID,
						CurrentTitle:           audit.FolderTitle,
						CompleteCount:          int(atomic.LoadInt32(&completeCount)),
						IncompleteCount:        int(atomic.LoadInt32(&incompleteCount)),
						NoVideoCount:           int(atomic.LoadInt32(&noVideoCount)),
						Elapsed:                elapsed,
						Speed:                  speed,
						ETA:                    eta,
						Message:                fmt.Sprintf("🔍 [%d/%d] Auditing %s (%s)", curCompleted, totalTasks, audit.MovieID, t.actressName),
						Item:                   audit,
					}
				}
			}
		}()
	}

	wg.Wait()

	// Sort final results
	sort.Slice(results, func(i, j int) bool {
		if results[i].ActressName != results[j].ActressName {
			return results[i].ActressName < results[j].ActressName
		}
		return results[i].MovieID < results[j].MovieID
	})

	if progressChan != nil {
		progressChan <- AuditProgressEvent{
			Type:            ProgressDone,
			TotalMovies:     len(results),
			CompleteCount:   int(atomic.LoadInt32(&completeCount)),
			IncompleteCount: int(atomic.LoadInt32(&incompleteCount)),
			NoVideoCount:    int(atomic.LoadInt32(&noVideoCount)),
			Elapsed:         time.Since(startTime),
			Message:         fmt.Sprintf("✨ Scan completed: audited %d movies in %s", len(results), time.Since(startTime).Round(time.Millisecond)),
		}
	}

	return results, nil
}

// AuditDirectory discovers and audits all movie folders (synchronous helper).
func (a *Auditor) AuditDirectory(ctx context.Context, rootDir string) ([]*MovieAudit, error) {
	return a.AuditDirectoryWithProgress(ctx, rootDir, nil)
}

// InspectFolder inspects a single movie directory and evaluates its checklist.
func (a *Auditor) InspectFolder(ctx context.Context, folderPath, rootDir string) (*MovieAudit, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}

	folderName := filepath.Base(folderPath)
	parentName := filepath.Base(filepath.Dir(folderPath))
	relPath, _ := filepath.Rel(rootDir, folderPath)

	// Extract JAV ID from folder name or files
	movieID := ""
	folderTitle := folderName
	if res := a.matcher.MatchFile(scanner.FileInfo{Name: folderName, Path: folderPath}); res != nil && res.ID != "" {
		movieID = res.ID
		folderTitle = strings.TrimSpace(strings.TrimPrefix(folderName, res.ID))
		folderTitle = strings.TrimPrefix(folderTitle, "-")
		folderTitle = strings.TrimSpace(folderTitle)
	}

	actressName := ""
	if parentName != "." && parentName != "/" && filepath.Dir(folderPath) != rootDir {
		actressName = parentName
	}

	audit := &MovieAudit{
		FolderPath:   folderPath,
		RelativePath: relPath,
		ActressName:  actressName,
		MovieID:      movieID,
		FolderTitle:  folderTitle,
		VideoFiles:   make([]string, 0),
		MissingItems:   make([]string, 0),
		Warnings:       make([]string, 0),
		CorruptedFiles: make([]string, 0),
	}

	// 1. Check folder naming anomaly (unbalanced brackets etc.)
	if nameWarns := checkFolderNaming(folderName); len(nameWarns) > 0 {
		audit.Warnings = append(audit.Warnings, nameWarns...)
	}

	// 2. Scan entries in folder
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		fullPath := filepath.Join(folderPath, name)

		if entry.IsDir() {
			if lower == "extrafanart" || lower == "screenshots" {
				subEntries, _ := os.ReadDir(fullPath)
				for _, sub := range subEntries {
					subLower := strings.ToLower(sub.Name())
					if !sub.IsDir() && (strings.HasSuffix(subLower, ".jpg") || strings.HasSuffix(subLower, ".png") || strings.HasSuffix(subLower, ".webp")) {
						subPath := filepath.Join(fullPath, sub.Name())
						if sfi, err := sub.Info(); err == nil {
							if sfi.Size() == 0 {
								audit.CorruptedFiles = append(audit.CorruptedFiles, subPath)
								audit.Warnings = append(audit.Warnings, fmt.Sprintf("0-byte corrupted screenshot: extrafanart/%s", sub.Name()))
							} else {
								audit.ScreenshotsCount++
							}
						}
					}
				}
			}
			continue
		}

		fi, fiErr := entry.Info()
		fileSize := int64(0)
		if fiErr == nil {
			fileSize = fi.Size()
		}

		// Check Video Files
		if isVideoFile(name) {
			if fileSize == 0 {
				audit.CorruptedFiles = append(audit.CorruptedFiles, fullPath)
				audit.Warnings = append(audit.Warnings, fmt.Sprintf("0-byte corrupted video file: %s", name))
			} else {
				audit.HasVideo = true
				audit.VideoFiles = append(audit.VideoFiles, fullPath)
				audit.VideoTotalBytes += fileSize
			}
			// If movie ID wasn't found from folder name, try video filename
			if audit.MovieID == "" {
				if res := a.matcher.MatchFile(scanner.FileInfo{Name: name, Path: fullPath}); res != nil && res.ID != "" {
					audit.MovieID = res.ID
				}
			}
		}

		// Check NFO
		if strings.HasSuffix(lower, ".nfo") {
			if fileSize == 0 {
				audit.CorruptedFiles = append(audit.CorruptedFiles, fullPath)
				audit.Warnings = append(audit.Warnings, fmt.Sprintf("0-byte empty NFO file: %s", name))
			} else {
				audit.HasNFO = true
				audit.NFOPath = fullPath
			}
		}

		// Check HTML
		if lower == "movie.html" || lower == "index.html" {
			if fileSize == 0 {
				audit.CorruptedFiles = append(audit.CorruptedFiles, fullPath)
				audit.Warnings = append(audit.Warnings, fmt.Sprintf("0-byte empty HTML file: %s", name))
			} else {
				audit.HasHTML = true
				audit.HTMLPath = fullPath
			}
		}

		// Check Poster
		if lower == "poster.jpg" || lower == "poster.png" || lower == "cover.jpg" || (audit.MovieID != "" && strings.HasPrefix(lower, strings.ToLower(audit.MovieID)+"-poster")) {
			audit.PosterPath = fullPath
			audit.PosterBytes = fileSize
			if fileSize == 0 {
				audit.CorruptedFiles = append(audit.CorruptedFiles, fullPath)
				audit.Warnings = append(audit.Warnings, fmt.Sprintf("0-byte corrupted poster: %s", name))
			} else {
				audit.HasPoster = true
				// Inspect dimensions & aspect ratio
				imgInfo := inspectImage(fullPath, 300, 300, 0.50, 2.20)
				if imgInfo.Width > 0 && imgInfo.Height > 0 {
					audit.PosterDimensions = fmt.Sprintf("%dx%d", imgInfo.Width, imgInfo.Height)
				}
				if imgInfo.Warning != "" {
					audit.Warnings = append(audit.Warnings, fmt.Sprintf("Poster (%s): %s", name, imgInfo.Warning))
				}
			}
		}

		// Check Fanart
		if lower == "fanart.jpg" || lower == "fanart.png" || lower == "backdrop.jpg" || (audit.MovieID != "" && strings.HasPrefix(lower, strings.ToLower(audit.MovieID)+"-fanart")) {
			audit.FanartPath = fullPath
			audit.FanartBytes = fileSize
			if fileSize == 0 {
				audit.CorruptedFiles = append(audit.CorruptedFiles, fullPath)
				audit.Warnings = append(audit.Warnings, fmt.Sprintf("0-byte corrupted fanart: %s", name))
			} else {
				audit.HasFanart = true
				// Inspect dimensions & aspect ratio (fanart is typically landscape 1.0 - 2.6)
				imgInfo := inspectImage(fullPath, 300, 200, 0.90, 2.60)
				if imgInfo.Width > 0 && imgInfo.Height > 0 {
					audit.FanartDimensions = fmt.Sprintf("%dx%d", imgInfo.Width, imgInfo.Height)
				}
				if imgInfo.Warning != "" {
					audit.Warnings = append(audit.Warnings, fmt.Sprintf("Fanart (%s): %s", name, imgInfo.Warning))
				}
			}
		}

		// Check flat screenshots
		if strings.HasPrefix(lower, "fanart") && strings.HasSuffix(lower, ".jpg") && lower != "fanart.jpg" {
			if fileSize > 0 {
				audit.ScreenshotsCount++
			}
		} else if strings.HasPrefix(lower, "sample-") && strings.HasSuffix(lower, ".jpg") {
			if fileSize > 0 {
				audit.ScreenshotsCount++
			}
		}
	}

	if audit.ScreenshotsCount > 0 {
		audit.HasScreenshots = true
	}

	// 3. Multi-part Video Sequence Check
	if len(audit.VideoFiles) > 1 {
		isMp, _, mpWarns := checkMultipartSequence(audit.VideoFiles)
		audit.IsMultipart = isMp
		audit.MultiPartCount = len(audit.VideoFiles)
		if len(mpWarns) > 0 {
			audit.Warnings = append(audit.Warnings, mpWarns...)
		}
	}

	// 4. Video Size Threshold Check
	if audit.HasVideo && audit.VideoTotalBytes < MinVideoBytes {
		audit.Warnings = append(audit.Warnings, fmt.Sprintf("Unusually small video file size (%s, expected >= 30MB)", FormatBytes(audit.VideoTotalBytes)))
	}

	// 5. Check Database
	if a.dbInstance != nil && audit.MovieID != "" {
		if m, err := a.dbInstance.GetMovie(audit.MovieID); err == nil && m != nil {
			audit.InDB = true
			audit.MovieMeta = m
		} else {
			var count int
			_ = a.dbInstance.QueryRow("SELECT COUNT(*) FROM organized_movies WHERE movie_id = ? UNION ALL SELECT COUNT(*) FROM library_files WHERE movie_id = ?", audit.MovieID, audit.MovieID).Scan(&count)
			if count > 0 {
				audit.InDB = true
			}
		}
	}

	// 6. Evaluate Missing Items
	if !audit.HasVideo {
		audit.MissingItems = append(audit.MissingItems, "Video")
	}
	if !audit.HasNFO {
		audit.MissingItems = append(audit.MissingItems, "NFO")
	}
	if !audit.HasHTML {
		audit.MissingItems = append(audit.MissingItems, "HTML")
	}
	if !audit.HasPoster {
		audit.MissingItems = append(audit.MissingItems, "Poster")
	}
	if !audit.HasFanart {
		audit.MissingItems = append(audit.MissingItems, "Fanart")
	}
	if !audit.HasScreenshots {
		audit.MissingItems = append(audit.MissingItems, "Screenshots")
	}

	// 7. Status calculation
	if !audit.HasVideo {
		audit.Status = StatusNoVideo
	} else if len(audit.MissingItems) == 0 {
		audit.Status = StatusComplete
	} else {
		audit.Status = StatusIncomplete
	}

	return audit, nil
}

// ImageInfo holds dimensions, format, and aspect ratio metrics.
type ImageInfo struct {
	Width       int
	Height      int
	Size        int64
	AspectRatio float64
	Format      string
	IsValid     bool
	Warning     string
}

// inspectImage decodes image headers to check dimensions and aspect ratio quickly.
func inspectImage(filePath string, minWidth, minHeight int, minAspect, maxAspect float64) ImageInfo {
	info := ImageInfo{}
	fi, err := os.Stat(filePath)
	if err != nil {
		info.Warning = "file not found"
		return info
	}
	info.Size = fi.Size()
	if info.Size == 0 {
		info.Warning = "0-byte empty file"
		return info
	}

	f, err := os.Open(filePath)
	if err != nil {
		info.Warning = "cannot open file"
		return info
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		info.Warning = fmt.Sprintf("unreadable or corrupted image header (%v)", err)
		return info
	}
	info.Width = cfg.Width
	info.Height = cfg.Height
	info.Format = format

	if info.Height > 0 {
		info.AspectRatio = float64(info.Width) / float64(info.Height)
	}

	if info.Width < minWidth || info.Height < minHeight {
		info.Warning = fmt.Sprintf("low resolution (%dx%d, expected >= %dx%d)", info.Width, info.Height, minWidth, minHeight)
		return info
	}

	if minAspect > 0 && maxAspect > 0 && info.AspectRatio > 0 {
		if info.AspectRatio < minAspect || info.AspectRatio > maxAspect {
			info.Warning = fmt.Sprintf("unusual aspect ratio (%.2f:1, expected %.2f - %.2f)", info.AspectRatio, minAspect, maxAspect)
			return info
		}
	}

	info.IsValid = true
	return info
}

var (
	multipartRegex = regexp.MustCompile(`(?i)(?:[-_ .](?:cd|part|disc|dvd))([0-9]+)`)
)

// checkMultipartSequence verifies if multi-part files form a continuous 1..N sequence without gaps.
func checkMultipartSequence(videoPaths []string) (isMultipart bool, missingParts []int, warnings []string) {
	if len(videoPaths) <= 1 {
		return false, nil, nil
	}

	parts := make(map[int]string)
	var unlabelled []string

	for _, p := range videoPaths {
		base := filepath.Base(p)
		matches := multipartRegex.FindStringSubmatch(base)
		if len(matches) > 1 {
			idx, _ := strconv.Atoi(matches[1])
			if idx > 0 {
				parts[idx] = base
			} else {
				unlabelled = append(unlabelled, base)
			}
		} else {
			unlabelled = append(unlabelled, base)
		}
	}

	if len(parts) == 0 {
		return false, nil, []string{fmt.Sprintf("Multiple video files found (%d files) without CD/part sequence identifiers", len(videoPaths))}
	}

	isMultipart = true

	// If CD1 is missing but there is exactly 1 unlabelled base file (e.g. PPPD-830.mp4 + PPPD-830-cd2.mp4)
	if _, hasPart1 := parts[1]; !hasPart1 && len(unlabelled) == 1 {
		parts[1] = unlabelled[0]
		targetName := parts[2]
		if targetName == "" {
			for _, v := range parts {
				targetName = v
				break
			}
		}
		warnings = append(warnings, fmt.Sprintf("Multi-part note: '%s' treated as CD1 alongside '%s' (recommend renaming to -cd1 for Jellyfin)", unlabelled[0], targetName))
		unlabelled = nil
	}

	maxPart := 0
	for idx := range parts {
		if idx > maxPart {
			maxPart = idx
		}
	}

	for i := 1; i <= maxPart; i++ {
		if _, exists := parts[i]; !exists {
			missingParts = append(missingParts, i)
			warnings = append(warnings, fmt.Sprintf("Missing multi-part video CD/Part %d (found %d of %d parts)", i, len(parts), maxPart))
		}
	}

	return isMultipart, missingParts, warnings
}

// checkFolderNaming checks for unbalanced brackets or naming anomalies.
func checkFolderNaming(folderName string) []string {
	var warnings []string

	// Check square brackets
	openSquare := strings.Count(folderName, "[")
	closeSquare := strings.Count(folderName, "]")
	if openSquare != closeSquare {
		warnings = append(warnings, fmt.Sprintf("Unbalanced brackets '[ ]' in folder name (found %d '[', %d ']')", openSquare, closeSquare))
	}

	// Check parentheses
	openParen := strings.Count(folderName, "(")
	closeParen := strings.Count(folderName, ")")
	if openParen != closeParen {
		warnings = append(warnings, fmt.Sprintf("Unbalanced parentheses '( )' in folder name (found %d '(', %d ')')", openParen, closeParen))
	}

	// Check Japanese brackets 【 】
	openJp := strings.Count(folderName, "【")
	closeJp := strings.Count(folderName, "】")
	if openJp != closeJp {
		warnings = append(warnings, fmt.Sprintf("Unbalanced Japanese brackets '【 】' in folder name (found %d '【', %d '】')", openJp, closeJp))
	}

	return warnings
}

// FixMovieWithProgress downloads missing assets, generates metadata, updates DB, and re-inspects disk state with real-time events.
func (a *Auditor) FixMovieWithProgress(ctx context.Context, item *MovieAudit, progressChan chan<- FixProgressEvent) error {
	if item == nil || item.MovieID == "" {
		return fmt.Errorf("invalid movie item or missing Movie ID")
	}

	report := func(stepIdx, totalSteps int, stepName, msg, detail string, curFile, totFiles int, err error) {
		if progressChan != nil {
			progressChan <- FixProgressEvent{
				MovieID:     item.MovieID,
				Actress:     item.ActressName,
				StepIndex:   stepIdx,
				TotalSteps:  totalSteps,
				StepName:    stepName,
				Message:     msg,
				Detail:      detail,
				CurrentFile: curFile,
				TotalFiles:  totFiles,
				Err:         err,
			}
		}
	}

	totalSteps := 6

	// 1. Fetch metadata (DB or Scraper)
	report(1, totalSteps, "fetch_meta", fmt.Sprintf("Fetching metadata for %s...", item.MovieID), "", 0, 0, nil)
	var movie *scraper.Movie
	if a.dbInstance != nil {
		movie, _ = a.dbInstance.GetMovie(item.MovieID)
	}
	if movie == nil || movie.CoverURL == "" || len(movie.SampleScreenshots) == 0 {
		if a.scraper != nil {
			scraped, sErr := a.scraper.ScrapeOnline(ctx, item.MovieID)
			if sErr == nil && scraped != nil {
				movie = scraped
				if a.dbInstance != nil {
					_ = a.dbInstance.SaveMovie(movie)
				}
			} else if movie == nil {
				if fallbackM, fErr := a.scraper.Scrape(ctx, item.MovieID); fErr == nil && fallbackM != nil {
					movie = fallbackM
				} else {
					report(1, totalSteps, "fetch_meta_warning", fmt.Sprintf("Warning: online scraper error: %v", sErr), "", 0, 0, sErr)
				}
			}
		}
	}

	if movie == nil {
		movie = &scraper.Movie{
			ID:    item.MovieID,
			Title: item.FolderTitle,
		}
	}

	// 2. Load UserState
	var userState *db.UserState
	if a.dbInstance != nil {
		userState, _ = a.dbInstance.GetUserState(item.MovieID)
	}

	// 3. Fix Poster
	if !item.HasPoster {
		url := movie.CoverURL
		if url == "" {
			url = movie.PosterURL
		}
		if url != "" {
			report(2, totalSteps, "download_poster", "Downloading high-resolution poster.jpg (Cover)...", url, 1, 1, nil)
			posterPath := filepath.Join(item.FolderPath, "poster.jpg")
			if err := jellyfin.DownloadAsset(ctx, url, posterPath); err == nil {
				item.HasPoster = true
				item.PosterPath = posterPath
			} else {
				report(2, totalSteps, "download_poster_error", fmt.Sprintf("Failed to download poster: %v", err), "", 0, 0, err)
			}
		} else {
			report(2, totalSteps, "download_poster_skip", "No poster URL available from metadata provider", "", 0, 0, nil)
		}
	} else {
		report(2, totalSteps, "poster_ok", "Poster (cover) already present on disk", "", 1, 1, nil)
	}

	// 4. Fix Fanart
	if !item.HasFanart {
		if movie.CoverURL != "" {
			report(3, totalSteps, "download_fanart", "Downloading backdrop fanart.jpg...", movie.CoverURL, 1, 1, nil)
			fanartPath := filepath.Join(item.FolderPath, "fanart.jpg")
			if err := jellyfin.DownloadAsset(ctx, movie.CoverURL, fanartPath); err == nil {
				item.HasFanart = true
				item.FanartPath = fanartPath
			} else {
				report(3, totalSteps, "download_fanart_error", fmt.Sprintf("Failed to download fanart: %v", err), "", 0, 0, err)
			}
		} else {
			report(3, totalSteps, "download_fanart_skip", "No backdrop URL available from metadata provider", "", 0, 0, nil)
		}
	} else {
		report(3, totalSteps, "fanart_ok", "Backdrop fanart already present on disk", "", 1, 1, nil)
	}

	// 5. Fix Screenshots concurrently
	totSamples := len(movie.SampleScreenshots)
	if !item.HasScreenshots || item.ScreenshotsCount == 0 {
		if totSamples > 0 {
			extraDir := filepath.Join(item.FolderPath, "extrafanart")
			_ = os.MkdirAll(extraDir, 0o755)
			var successScreenshots int32
			var completedScreenshots int32
			sem := make(chan struct{}, 5)
			var wg sync.WaitGroup

			for i, rawURL := range movie.SampleScreenshots {
				idx := i + 1
				url := rawURL
				samplePath := filepath.Join(extraDir, fmt.Sprintf("fanart%d.jpg", idx))

				wg.Add(1)
				go func(num int, u, dest string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if err := jellyfin.DownloadAsset(ctx, u, dest); err == nil {
						atomic.AddInt32(&successScreenshots, 1)
					}
					done := atomic.AddInt32(&completedScreenshots, 1)
					report(4, totalSteps, "download_screenshot", fmt.Sprintf("Downloading sample screenshot %d of %d (fanart%d.jpg)...", done, totSamples, num), u, int(done), totSamples, nil)
				}(idx, url, samplePath)
			}
			wg.Wait()

			if successScreenshots > 0 {
				item.HasScreenshots = true
				item.ScreenshotsCount = int(successScreenshots)
			}
		} else {
			report(4, totalSteps, "screenshots_na", "No sample screenshots available from provider (N/A)", "", 0, 0, nil)
			item.HasScreenshots = true
			item.ScreenshotsCount = 0
		}
	} else {
		report(4, totalSteps, "screenshots_ok", fmt.Sprintf("%d screenshots already present on disk", item.ScreenshotsCount), "", item.ScreenshotsCount, item.ScreenshotsCount, nil)
	}

	// 6. Fix NFO & HTML
	report(5, totalSteps, "generate_metadata", fmt.Sprintf("Generating Jellyfin .nfo XML & Cinematic movie.html for %s...", item.MovieID), "", 1, 2, nil)
	nfoPath := filepath.Join(item.FolderPath, item.MovieID+".nfo")
	if !item.HasNFO || item.NFOPath == "" {
		if err := jellyfin.WriteNFO(movie, userState, nfoPath); err == nil {
			item.HasNFO = true
			item.NFOPath = nfoPath
		}
	}

	htmlPath := filepath.Join(item.FolderPath, "movie.html")
	if !item.HasHTML || item.HTMLPath == "" {
		videoRelName := ""
		if len(item.VideoFiles) > 0 {
			videoRelName = filepath.Base(item.VideoFiles[0])
		}
		if err := jellyfin.WriteHTML(movie, userState, htmlPath, videoRelName); err == nil {
			item.HasHTML = true
			item.HTMLPath = htmlPath
		}
	}

	// 7. Update SQLite DB & Re-verify disk
	report(6, totalSteps, "recheck_and_save", "Updating SQLite database & re-verifying disk status...", "", 2, 2, nil)
	if a.dbInstance != nil {
		if len(item.VideoFiles) > 0 {
			_ = a.dbInstance.UpsertLibraryFile(db.LibraryFileRecord{
				FilePath:      item.VideoFiles[0],
				MovieID:       item.MovieID,
				SizeBytes:     item.VideoTotalBytes,
				OrganizedPath: item.FolderPath,
				ScannedAt:     time.Now(),
			})
			_ = a.dbInstance.SetOrganized(item.MovieID, item.FolderPath, item.VideoFiles[0])
			item.InDB = true
		}
	}

	// Re-inspect folder directly from disk to guarantee state accuracy
	if rechecked, err := a.InspectFolder(ctx, item.FolderPath, filepath.Dir(item.FolderPath)); err == nil && rechecked != nil {
		item.HasVideo = rechecked.HasVideo
		item.VideoFiles = rechecked.VideoFiles
		item.VideoTotalBytes = rechecked.VideoTotalBytes
		item.HasNFO = rechecked.HasNFO
		item.NFOPath = rechecked.NFOPath
		item.HasHTML = rechecked.HasHTML
		item.HTMLPath = rechecked.HTMLPath
		item.HasPoster = rechecked.HasPoster
		item.PosterPath = rechecked.PosterPath
		item.HasFanart = rechecked.HasFanart
		item.FanartPath = rechecked.FanartPath
		item.HasScreenshots = rechecked.HasScreenshots
		item.ScreenshotsCount = rechecked.ScreenshotsCount
		item.InDB = rechecked.InDB
		item.MissingItems = rechecked.MissingItems
		item.Status = rechecked.Status
		item.PosterDimensions = rechecked.PosterDimensions
		item.PosterBytes = rechecked.PosterBytes
		item.FanartDimensions = rechecked.FanartDimensions
		item.FanartBytes = rechecked.FanartBytes
		item.IsMultipart = rechecked.IsMultipart
		item.MultiPartCount = rechecked.MultiPartCount
		item.Warnings = rechecked.Warnings
		item.CorruptedFiles = rechecked.CorruptedFiles
	}

	if len(item.MissingItems) > 0 {
		return fmt.Errorf("%d items still missing on disk: %s", len(item.MissingItems), strings.Join(item.MissingItems, ", "))
	}

	return nil
}

// FixMovie downloads missing assets and regenerates NFO/HTML for an incomplete movie (synchronous helper).
func (a *Auditor) FixMovie(ctx context.Context, item *MovieAudit, reporter func(step string, msg string)) error {
	var ch chan FixProgressEvent
	if reporter != nil {
		ch = make(chan FixProgressEvent, 50)
		go func() {
			for ev := range ch {
				reporter(ev.StepName, ev.Message)
			}
		}()
	}
	err := a.FixMovieWithProgress(ctx, item, ch)
	if ch != nil {
		close(ch)
	}
	return err
}

func isVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp4", ".mkv", ".avi", ".wmv", ".ts", ".m4v", ".iso", ".mov", ".flv":
		return true
	default:
		return false
	}
}

// FormatBytes converts bytes to human-readable string (e.g. 4.2 GB).
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
