package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/actress"
	"github.com/dagalp/r19dev-scraper/pkg/cache"
	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/jellyfin"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/organizer"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

//go:embed static/*
var staticFS embed.FS

// Server represents the HTTP web service for R19DEV Studio.
type Server struct {
	targetDir      string
	port           int
	scanner        *scanner.Scanner
	matcher        *matcher.Matcher
	scraperClient  *scraper.Client
	actressService *actress.Service
	db             *db.DB
}

// Config holds initialization parameters for the web server.
type Config struct {
	TargetDir string
	Port      int
	Language  string
}

// NewServer initializes a new Web Server instance.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.TargetDir == "" {
		cfg.TargetDir = "."
	}
	absTarget, err := filepath.Abs(cfg.TargetDir)
	if err != nil {
		absTarget = cfg.TargetDir
	}

	sc := scanner.New(scanner.DefaultConfig())
	mc, err := matcher.New(matcher.DefaultConfig())
	if err != nil {
		return nil, err
	}

	scClient := scraper.NewClient(15 * time.Second)
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	scClient.SetLanguage(cfg.Language)
	scClient.SetCache(cache.Default())

	database, err := db.Default()
	if err != nil {
		return nil, err
	}

	actSvc := actress.New(database, scClient)

	return &Server{
		targetDir:      absTarget,
		port:           cfg.Port,
		scanner:        sc,
		matcher:        mc,
		scraperClient:  scClient,
		actressService: actSvc,
		db:             database,
	}, nil
}

// Handler builds and returns the configured http.Handler for the web server.
func (s *Server) Handler() (http.Handler, error) {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/scan", s.handleScan)
	mux.HandleFunc("/api/scan/stream", s.handleScanStream)
	mux.HandleFunc("/api/movie/", s.handleMovie)
	mux.HandleFunc("/api/scrape/", s.handleScrape)
	mux.HandleFunc("/api/scrape/stream", s.handleScrapeStream)
	mux.HandleFunc("/api/images/", s.handleImage)
	mux.HandleFunc("/api/proxy-image", s.handleProxyImage)
	mux.HandleFunc("/api/actresses", s.handleActresses)
	mux.HandleFunc("/api/actresses/follow", s.handleActressFollow)
	mux.HandleFunc("/api/actresses/unfollow", s.handleActressUnfollow)
	mux.HandleFunc("/api/actresses/releases", s.handleActressReleases)
	mux.HandleFunc("/api/organize", s.handleOrganize)
	mux.HandleFunc("/api/organize/stream", s.handleOrganizeStream)
	mux.HandleFunc("/api/open-folder", s.handleOpenFolder)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/history/detail", s.handleHistoryDetail)

	// Static Files from Embedded FS
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded static filesystem: %w", err)
	}

	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	return s.loggingMiddleware(mux), nil
}

// responseWriterWrapper wraps http.ResponseWriter to capture HTTP status codes for logging.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterWrapper) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// loggingMiddleware provides continuous, colorized HTTP access logs in the terminal.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)

		// Omit noisy vendor static assets unless error
		if strings.HasPrefix(r.URL.Path, "/vendor/") && wrapped.statusCode < 400 {
			return
		}

		color := "\033[32m" // Green
		if wrapped.statusCode >= 300 && wrapped.statusCode < 400 {
			color = "\033[36m" // Cyan
		} else if wrapped.statusCode >= 400 && wrapped.statusCode < 500 {
			color = "\033[33m" // Yellow
		} else if wrapped.statusCode >= 500 {
			color = "\033[31m" // Red
		}
		reset := "\033[0m"

		fmt.Printf("[%s] %s%d%s %-6s %s (%v)\n",
			time.Now().Format("15:04:05"),
			color, wrapped.statusCode, reset,
			r.Method, r.URL.Path,
			duration.Round(time.Millisecond),
		)
	})
}

// Start runs the HTTP listener on the configured port.
// If the configured port is already in use, it automatically falls back to the next available port.
func (s *Server) Start(openBrowserOnStart bool) error {
	handler, err := s.Handler()
	if err != nil {
		return err
	}

	var ln net.Listener
	originalPort := s.port
	for p := s.port; p <= s.port+20; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			ln = l
			s.port = p
			break
		}
	}
	if ln == nil {
		return fmt.Errorf("could not bind to port %d or any alternate port up to %d: address already in use", originalPort, originalPort+20)
	}
	defer ln.Close()

	if s.port != originalPort {
		fmt.Printf("⚠️  Port %d is already in use, automatically switched to port %d\n", originalPort, s.port)
	}

	serverURL := fmt.Sprintf("http://localhost:%d", s.port)
	fmt.Printf("🚀 R19DEV Studio Web UI running at: %s\n", serverURL)
	fmt.Printf("📂 Target Directory: %s\n", s.targetDir)

	if openBrowserOnStart {
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = OpenBrowser(serverURL)
		}()
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return server.Serve(ln)
}

// --- REST Handlers ---

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("path")
	if target == "" || target == "." {
		target = s.targetDir
	}
	if abs, err := filepath.Abs(target); err == nil && abs != "" {
		target = abs
	}

	scanRes, err := s.scanner.Scan(target)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	matches := s.matcher.Match(scanRes.Files)

	// Collect known metadata and user states for all matched items
	metadataMap := make(map[string]*scraper.Movie)
	userStatesMap := make(map[string]*db.UserState)

	for _, m := range matches {
		if m.ID == "" {
			continue
		}
		// Try DB first
		if s.db != nil {
			if mov, _ := s.db.GetMovie(m.ID); mov != nil {
				metadataMap[m.ID] = mov
			}
			if uState, _ := s.db.GetUserState(m.ID); uState != nil {
				userStatesMap[m.ID] = uState
			}
		}
		// Try Cache if not in DB
		if _, ok := metadataMap[m.ID]; !ok {
			if mov, found := cache.Default().GetMovie(m.ID); found && mov != nil {
				metadataMap[m.ID] = mov
			}
		}
	}

	orgStatus, orgFolders := detectOrganizedStatus(target, matches, s.db)
	resp := map[string]any{
		"target_dir":        target,
		"matches":           matches,
		"metadata":          metadataMap,
		"user_states":       userStatesMap,
		"organized_status":  orgStatus,
		"organized_folders": orgFolders,
	}

	writeJSON(w, resp)
}

func (s *Server) handleMovie(w http.ResponseWriter, r *http.Request) {
	// Format: /api/movie/{id} or /api/movie/{id}/state
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		writeJSONError(w, "missing movie id", http.StatusBadRequest)
		return
	}
	id := strings.ToUpper(pathParts[2])

	// Sub-route: /api/movie/{id}/state
	if len(pathParts) >= 4 && pathParts[3] == "state" {
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var state db.UserState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			writeJSONError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		state.MovieID = id
		if s.db != nil {
			_ = s.db.SetUserState(state)
		}
		writeJSON(w, map[string]any{"success": true, "state": state})
		return
	}

	// GET /api/movie/{id}
	if s.db != nil {
		if mov, _ := s.db.GetMovie(id); mov != nil {
			writeJSON(w, mov)
			return
		}
	}
	if mov, found := cache.Default().GetMovie(id); found && mov != nil {
		writeJSON(w, mov)
		return
	}

	writeJSONError(w, "movie not found", http.StatusNotFound)
}

func (s *Server) handleScrape(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/api/scrape/"))
	if id == "" {
		writeJSONError(w, "missing movie id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	movie, err := s.scraperClient.Scrape(ctx, id)
	if err != nil {
		writeJSONError(w, fmt.Sprintf("scrape failed: %v", err), http.StatusBadGateway)
		return
	}

	if s.db != nil {
		_ = s.db.SaveMovie(movie)
	}

	writeJSON(w, movie)
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/api/images/"))
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if imgBytes, found := cache.Default().GetImage(id); found && len(imgBytes) > 0 {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(imgBytes)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleActresses(w http.ResponseWriter, r *http.Request) {
	list, err := s.actressService.ListFollowed()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"actresses": list})
}

func (s *Server) handleActressFollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name     string `json:"name"`
		JaName   string `json:"ja_name"`
		ImageURL string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.actressService.Follow(req.Name, req.JaName, req.ImageURL); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleActressUnfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := s.actressService.Unfollow(req.Name); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleActressReleases(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	summaries, err := s.actressService.CheckAllFollowed(ctx)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"actresses": summaries})
}

func (s *Server) handleOrganize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
		DryRun      bool   `json:"dry_run"`
		SourceFile  string `json:"source_file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Destination == "" {
		writeJSONError(w, "destination path is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var matches []matcher.MatchResult

	// Single file organize
	if req.SourceFile != "" {
		mc, _ := matcher.New(matcher.DefaultConfig())
		fInfo := scanner.FileInfo{
			Path: req.SourceFile,
			Name: filepath.Base(req.SourceFile),
		}
		matches = mc.Match([]scanner.FileInfo{fInfo})
	} else {
		// Scan directory
		srcDir := req.Source
		if srcDir == "" || srcDir == "." {
			srcDir = s.targetDir
		}
		if abs, err := filepath.Abs(srcDir); err == nil && abs != "" {
			srcDir = abs
		}
		scanRes, err := s.scanner.Scan(srcDir)
		if err != nil {
			writeJSONError(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
			return
		}
		matches = s.matcher.Match(scanRes.Files)
	}

	var results []organizer.OrganizeResult
	successCount := 0

	for _, match := range matches {
		if match.ID == "" {
			continue
		}
		movie, err := s.scraperClient.Scrape(ctx, match.ID)
		if err != nil {
			continue
		}

		var uState *db.UserState
		if s.db != nil {
			uState, _ = s.db.GetUserState(match.ID)
		}

		res, err := organizer.OrganizeMatch(ctx, &match, movie, uState, req.Destination, req.DryRun)
		if err == nil && res != nil {
			results = append(results, *res)
			if res.Success {
				successCount++
				if !req.DryRun && s.db != nil {
					_ = s.db.SetOrganized(match.ID, res.TargetFolder, res.TargetVideo)
				}
			}
		}
	}

	writeJSON(w, map[string]any{
		"results":       results,
		"success_count": successCount,
		"total_count":   len(results),
	})
}

func (s *Server) handleScanStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	target := r.URL.Query().Get("path")
	if target == "" || target == "." {
		target = s.targetDir
	}
	if abs, err := filepath.Abs(target); err == nil && abs != "" {
		target = abs
	}

	ch := make(chan []scanner.FileInfo, 20)
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	go func() {
		defer close(ch)
		_, _ = s.scanner.ScanStream(ctx, target, 5, ch)
	}()

	var allFiles []scanner.FileInfo
	for chunk := range ch {
		allFiles = append(allFiles, chunk...)
		matches := s.matcher.Match(allFiles)
		data, _ := json.Marshal(map[string]any{
			"phase":      "scanning",
			"discovered": len(allFiles),
			"matched":    len(matches),
			"chunk_size": len(chunk),
		})
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Done scanning, compile full results
	finalMatches := s.matcher.Match(allFiles)
	metadataMap := make(map[string]*scraper.Movie)
	userStatesMap := make(map[string]*db.UserState)

	for _, m := range finalMatches {
		if m.ID == "" {
			continue
		}
		if s.db != nil {
			if mov, _ := s.db.GetMovie(m.ID); mov != nil {
				metadataMap[m.ID] = mov
			}
			if uState, _ := s.db.GetUserState(m.ID); uState != nil {
				userStatesMap[m.ID] = uState
			}
		}
		if _, ok := metadataMap[m.ID]; !ok {
			if mov, found := cache.Default().GetMovie(m.ID); found && mov != nil {
				metadataMap[m.ID] = mov
			}
		}
	}

	orgStatus, orgFolders := detectOrganizedStatus(target, finalMatches, s.db)
	doneData, _ := json.Marshal(map[string]any{
		"phase":             "done",
		"target_dir":        target,
		"total":             len(allFiles),
		"matches":           finalMatches,
		"metadata":          metadataMap,
		"user_states":       userStatesMap,
		"organized_status":  orgStatus,
		"organized_folders": orgFolders,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

func (s *Server) handleOrganizeStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	srcDir := r.URL.Query().Get("source")
	destDir := r.URL.Query().Get("destination")
	dryRun := r.URL.Query().Get("dry_run") == "true"
	singleFile := r.URL.Query().Get("source_file")
	targetMovieID := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("movie_id")))

	if destDir == "" {
		errData, _ := json.Marshal(map[string]any{"error": "destination is required"})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
		flusher.Flush()
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Second)
	defer cancel()

	var matches []matcher.MatchResult
	if singleFile != "" {
		mc, _ := matcher.New(matcher.DefaultConfig())
		matches = mc.Match([]scanner.FileInfo{{
			Path: singleFile,
			Name: filepath.Base(singleFile),
		}})
	} else {
		if srcDir == "" || srcDir == "." {
			srcDir = s.targetDir
		}
		if abs, err := filepath.Abs(srcDir); err == nil && abs != "" {
			srcDir = abs
		}
		scanRes, err := s.scanner.Scan(srcDir)
		if err != nil {
			errData, _ := json.Marshal(map[string]any{"error": err.Error()})
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
			flusher.Flush()
			return
		}
		matches = s.matcher.Match(scanRes.Files)
	}

	var validMatches []matcher.MatchResult
	for _, m := range matches {
		if m.ID != "" {
			if targetMovieID != "" && m.ID != targetMovieID {
				continue
			}
			validMatches = append(validMatches, m)
		}
	}

	var logBuf strings.Builder
	logBuf.WriteString(fmt.Sprintf("🚀 Starting organize from %s -> %s (DryRun: %v)...\n\n", srcDir, destDir, dryRun))

	startData, _ := json.Marshal(map[string]any{
		"phase": "start",
		"total": len(validMatches),
	})
	fmt.Fprintf(w, "event: start\ndata: %s\n\n", startData)
	flusher.Flush()

	successCount := 0
	for i, match := range validMatches {
		// Report step: checking metadata
		stepMsg := fmt.Sprintf("กำลังตรวจสอบข้อมูล metadata ของ %s...", match.ID)
		logBuf.WriteString(fmt.Sprintf("   → %s\n", stepMsg))
		stepCheck, _ := json.Marshal(map[string]any{
			"movie_id": match.ID,
			"step":     "check_metadata",
			"index":    i + 1,
			"total":    len(validMatches),
			"message":  stepMsg,
		})
		fmt.Fprintf(w, "event: step\ndata: %s\n\n", stepCheck)
		flusher.Flush()

		movie, err := s.scraperClient.Scrape(ctx, match.ID)
		if err != nil {
			logBuf.WriteString(fmt.Sprintf("[FAIL] %s -> Error: %v\n", match.ID, err))
			itemData, _ := json.Marshal(map[string]any{
				"index":    i + 1,
				"total":    len(validMatches),
				"percent":  (i + 1) * 100 / len(validMatches),
				"movie_id": match.ID,
				"success":  false,
				"error":    err.Error(),
			})
			fmt.Fprintf(w, "event: item\ndata: %s\n\n", itemData)
			flusher.Flush()
			continue
		}

		var uState *db.UserState
		if s.db != nil {
			uState, _ = s.db.GetUserState(match.ID)
		}

		// Progress reporter callback for granular steps inside OrganizeMatch
		stepReporter := func(step string, current, total int, message string) {
			logBuf.WriteString(fmt.Sprintf("   → %s\n", message))
			stepJSON, _ := json.Marshal(map[string]any{
				"movie_id":     match.ID,
				"step":         step,
				"step_current": current,
				"step_total":   total,
				"message":      message,
				"index":        i + 1,
				"total":        len(validMatches),
			})
			fmt.Fprintf(w, "event: step\ndata: %s\n\n", stepJSON)
			flusher.Flush()
		}

		res, err := organizer.OrganizeMatchWithProgress(ctx, &match, movie, uState, destDir, dryRun, stepReporter)
		success := err == nil && res != nil && res.Success
		if success {
			successCount++
			if !dryRun && s.db != nil {
				_ = s.db.SetOrganized(match.ID, res.TargetFolder, res.TargetVideo)
			}
		}

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else if res != nil && res.Error != "" {
			errMsg = res.Error
		}

		targetFolder := ""
		targetVideo := ""
		if res != nil {
			targetFolder = res.TargetFolder
			targetVideo = res.TargetVideo
		}

		status := "[MOVED]"
		if dryRun {
			status = "[PLAN]"
		}
		if !success {
			status = "[FAIL]"
		}
		logBuf.WriteString(fmt.Sprintf("%s %s -> %s\n", status, match.ID, targetFolder))
		if targetVideo != "" {
			logBuf.WriteString(fmt.Sprintf("   Video: %s\n", targetVideo))
		}
		if errMsg != "" {
			logBuf.WriteString(fmt.Sprintf("   ❌ ข้อผิดพลาด: %s\n", errMsg))
		}

		itemData, _ := json.Marshal(map[string]any{
			"index":         i + 1,
			"total":         len(validMatches),
			"percent":       (i + 1) * 100 / len(validMatches),
			"movie_id":      match.ID,
			"target_folder": targetFolder,
			"target_video":  targetVideo,
			"success":       success,
			"dry_run":       dryRun,
			"error":         errMsg,
			"message":       fmt.Sprintf("จัดระเบียบ %s สำเร็จ!", match.ID),
		})
		fmt.Fprintf(w, "event: item\ndata: %s\n\n", itemData)
		flusher.Flush()
	}

	doneMsg := fmt.Sprintf("จัดระเบียบเสร็จสมบูรณ์ %d/%d ไฟล์", successCount, len(validMatches))
	logBuf.WriteString(fmt.Sprintf("\n✨ Complete! Successfully processed %d/%d movies.\n", successCount, len(validMatches)))

	// Save to SQLite operation_history
	if s.db != nil && len(validMatches) > 0 {
		failCount := len(validMatches) - successCount
		_, _ = s.db.AddOperationHistory("organize", destDir, len(validMatches), successCount, failCount, dryRun, logBuf.String())
	}

	doneData, _ := json.Marshal(map[string]any{
		"phase":         "done",
		"total":         len(validMatches),
		"success_count": successCount,
		"message":       doneMsg,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

func (s *Server) handleScrapeStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	targetID := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("id")))
	srcDir := r.URL.Query().Get("path")
	if srcDir == "" || srcDir == "." {
		srcDir = s.targetDir
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	ctx, cancel := context.WithTimeout(r.Context(), 1800*time.Second)
	defer cancel()

	var idsToScrape []string
	if targetID != "" {
		idsToScrape = []string{targetID}
	} else {
		scanRes, err := s.scanner.Scan(srcDir)
		if err != nil {
			errData, _ := json.Marshal(map[string]any{"error": err.Error()})
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
			flusher.Flush()
			return
		}
		matches := s.matcher.Match(scanRes.Files)
		seen := make(map[string]bool)
		for _, m := range matches {
			if m.ID != "" && !seen[m.ID] {
				seen[m.ID] = true
				if s.db != nil {
					if existing, _ := s.db.GetMovie(m.ID); existing != nil {
						continue
					}
				}
				idsToScrape = append(idsToScrape, m.ID)
			}
		}
	}

	startData, _ := json.Marshal(map[string]any{
		"phase": "start",
		"total": len(idsToScrape),
	})
	fmt.Fprintf(w, "event: start\ndata: %s\n\n", startData)
	flusher.Flush()

	successCount := 0
	for i, id := range idsToScrape {
		// Step 1: Querying R18.dev API
		step1, _ := json.Marshal(map[string]any{
			"movie_id": id,
			"step":     "fetch_api",
			"index":    i + 1,
			"total":    len(idsToScrape),
			"percent":  (i * 100) / len(idsToScrape),
			"message":  fmt.Sprintf("กำลังดึงข้อมูล %s จาก R18.dev API...", id),
		})
		fmt.Fprintf(w, "event: step\ndata: %s\n\n", step1)
		flusher.Flush()

		movie, err := s.scraperClient.Scrape(ctx, id)
		if err != nil {
			itemData, _ := json.Marshal(map[string]any{
				"movie_id": id,
				"index":    i + 1,
				"total":    len(idsToScrape),
				"percent":  (i + 1) * 100 / len(idsToScrape),
				"success":  false,
				"error":    err.Error(),
			})
			fmt.Fprintf(w, "event: item\ndata: %s\n\n", itemData)
			flusher.Flush()
			continue
		}

		// Step 2: Caching image and saving to database
		step2, _ := json.Marshal(map[string]any{
			"movie_id": id,
			"step":     "save_db",
			"index":    i + 1,
			"total":    len(idsToScrape),
			"percent":  (i*100 + 70) / len(idsToScrape),
			"message":  fmt.Sprintf("กำลังบันทึกข้อมูลและภาพปก %s ลงฐานข้อมูล...", id),
		})
		fmt.Fprintf(w, "event: step\ndata: %s\n\n", step2)
		flusher.Flush()

		if s.db != nil {
			_ = s.db.SaveMovie(movie)
		}
		successCount++

		itemData, _ := json.Marshal(map[string]any{
			"movie_id": id,
			"index":    i + 1,
			"total":    len(idsToScrape),
			"percent":  (i + 1) * 100 / len(idsToScrape),
			"success":  true,
			"movie":    movie,
			"message":  fmt.Sprintf("Scrape %s สำเร็จ!", id),
		})
		fmt.Fprintf(w, "event: item\ndata: %s\n\n", itemData)
		flusher.Flush()
	}

	doneData, _ := json.Marshal(map[string]any{
		"phase":         "done",
		"total":         len(idsToScrape),
		"success_count": successCount,
		"message":       fmt.Sprintf("Scrape เสร็จสมบูรณ์ %d/%d เรื่อง", successCount, len(idsToScrape)),
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneData)
	flusher.Flush()
}

func (s *Server) handleProxyImage(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, "missing url parameter", http.StatusBadRequest)
		return
	}

	upgradedURL := jellyfin.UpgradeDMMImageURL(rawURL)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upgradedURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", scraper.DefaultUA)
	req.Header.Set("Referer", "https://r18.dev/")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to raw URL
		if upgradedURL != rawURL {
			reqOrig, oErr := http.NewRequestWithContext(r.Context(), http.MethodGet, rawURL, nil)
			if oErr == nil {
				reqOrig.Header.Set("User-Agent", scraper.DefaultUA)
				resp, err = client.Do(reqOrig)
			}
		}
	}

	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, resp.Body)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

// detectOrganizedStatus checks which movies have already been organized into NAS Jellyfin structure,
// returning both a boolean map and a folder path map.
func detectOrganizedStatus(targetDir string, matches []matcher.MatchResult, database *db.DB) (map[string]bool, map[string]string) {
	organizedMap := make(map[string]bool)
	folderMap := make(map[string]string)

	if database != nil {
		if orgs, err := database.GetOrganizedMap(); err == nil && orgs != nil {
			organizedMap = orgs
		}
		if folders, err := database.GetOrganizedFolderMap(); err == nil && folders != nil {
			folderMap = folders
		}
	}

	checkDest := filepath.Join(targetDir, "JAV_Library")
	for _, m := range matches {
		if m.ID == "" || (organizedMap[m.ID] && folderMap[m.ID] != "") {
			continue
		}
		// If video file itself is already inside a folder with .nfo, it's organized
		dir := filepath.Dir(m.File.Path)
		nfoPath := filepath.Join(dir, m.ID+".nfo")
		if _, err := os.Stat(nfoPath); err == nil {
			organizedMap[m.ID] = true
			folderMap[m.ID] = dir
			if database != nil {
				_ = database.SetOrganized(m.ID, dir, m.File.Path)
			}
			continue
		}
		// If JAV_Library has this ID
		if entries, err := os.ReadDir(checkDest); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					subPath := filepath.Join(checkDest, entry.Name())
					if subEntries, sErr := os.ReadDir(subPath); sErr == nil {
						for _, sub := range subEntries {
							if sub.IsDir() && strings.Contains(strings.ToUpper(sub.Name()), m.ID) {
								foundFolder := filepath.Join(subPath, sub.Name())
								organizedMap[m.ID] = true
								folderMap[m.ID] = foundFolder
								if database != nil {
									_ = database.SetOrganized(m.ID, foundFolder, "")
								}
								break
							}
						}
					}
				}
			}
		}
	}

	return organizedMap, folderMap
}

func (s *Server) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path    string `json:"path"`
		MovieID string `json:"movie_id"`
		Actress string `json:"actress"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	targetPath := strings.TrimSpace(req.Path)
	if targetPath == "" && req.MovieID != "" {
		if s.db != nil {
			targetFolder, _, _ := s.db.GetOrganizedDetails(req.MovieID)
			targetPath = targetFolder
		}
	}

	// Fallback to searching by Actress name
	if targetPath == "" && req.Actress != "" {
		candidates := []string{
			filepath.Join(s.targetDir, "JAV_Library", req.Actress),
			filepath.Join(s.targetDir, req.Actress),
			filepath.Join("/Volumes/home/BT/2026/JAV_Library", req.Actress),
		}
		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				targetPath = cand
				break
			}
		}
	}

	// Fallback to searching in JAV_Library if not in DB
	if targetPath == "" && req.MovieID != "" {
		candidates := []string{
			filepath.Join(s.targetDir, "JAV_Library"),
			s.targetDir,
			"/Volumes/home/BT/2026/JAV_Library",
		}
		for _, libDir := range candidates {
			if entries, err := os.ReadDir(libDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						subPath := filepath.Join(libDir, entry.Name())
						if subEntries, sErr := os.ReadDir(subPath); sErr == nil {
							for _, sub := range subEntries {
								if sub.IsDir() && strings.Contains(strings.ToUpper(sub.Name()), req.MovieID) {
									targetPath = filepath.Join(subPath, sub.Name())
									break
								}
							}
						}
					}
					if targetPath != "" {
						break
					}
				}
			}
			if targetPath != "" {
				break
			}
		}
	}

	if targetPath == "" {
		fmt.Printf("⚠️  [Finder] Folder path not found for movie_id: '%s', actress: '%s', path: '%s'\n", req.MovieID, req.Actress, req.Path)
		writeJSONError(w, "folder path not found for movie", http.StatusNotFound)
		return
	}

	// Verify target exists
	if fi, err := os.Stat(targetPath); err != nil {
		fmt.Printf("⚠️  [Finder] Path not found on filesystem: %s (%v)\n", targetPath, err)
		writeJSONError(w, fmt.Sprintf("path not found: %v", err), http.StatusNotFound)
		return
	} else if !fi.IsDir() {
		targetPath = filepath.Dir(targetPath)
	}

	if err := OpenFolder(targetPath); err != nil {
		fmt.Printf("❌ [Finder] Error opening folder: %v\n", err)
		writeJSONError(w, fmt.Sprintf("failed to open folder: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("📂 [Finder] Successfully opened in Finder: %s\n", targetPath)
	writeJSON(w, map[string]any{"success": true, "path": targetPath})
}

// OpenFolder opens the specified directory in the OS file manager (Finder on macOS, Explorer on Windows, xdg-open on Linux).
func OpenFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Run()
}

// OpenBrowser opens the given URL in the user's default web browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// --- Operation History Handlers ---

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSONError(w, "database not available", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.db.ClearOperationHistory(); err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"success": true, "message": "History cleared"})
		return
	}

	limit := 50
	records, err := s.db.GetOperationHistory(limit, false)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"history": records})
}

func (s *Server) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSONError(w, "database not available", http.StatusServiceUnavailable)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, "valid id parameter required", http.StatusBadRequest)
		return
	}

	record, err := s.db.GetOperationDetail(id)
	if err != nil || record == nil {
		writeJSONError(w, "history record not found", http.StatusNotFound)
		return
	}
	writeJSON(w, record)
}
