package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// Static Files from Embedded FS
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded static filesystem: %w", err)
	}

	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	return mux, nil
}

// Start runs the HTTP listener on the configured port.
func (s *Server) Start(openBrowserOnStart bool) error {
	handler, err := s.Handler()
	if err != nil {
		return err
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
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return server.ListenAndServe()
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

	resp := map[string]any{
		"target_dir":       target,
		"matches":          matches,
		"metadata":         metadataMap,
		"user_states":      userStatesMap,
		"organized_status": detectOrganizedStatus(target, matches, s.db),
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

	doneData, _ := json.Marshal(map[string]any{
		"phase":            "done",
		"target_dir":       target,
		"total":            len(allFiles),
		"matches":          finalMatches,
		"metadata":         metadataMap,
		"user_states":      userStatesMap,
		"organized_status": detectOrganizedStatus(target, finalMatches, s.db),
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

	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Second)
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

	startData, _ := json.Marshal(map[string]any{
		"phase": "start",
		"total": len(validMatches),
	})
	fmt.Fprintf(w, "event: start\ndata: %s\n\n", startData)
	flusher.Flush()

	successCount := 0
	for i, match := range validMatches {
		// Report step: checking metadata
		stepCheck, _ := json.Marshal(map[string]any{
			"movie_id": match.ID,
			"step":     "check_metadata",
			"index":    i + 1,
			"total":    len(validMatches),
			"message":  fmt.Sprintf("กำลังตรวจสอบข้อมูล metadata ของ %s...", match.ID),
		})
		fmt.Fprintf(w, "event: step\ndata: %s\n\n", stepCheck)
		flusher.Flush()

		movie, err := s.scraperClient.Scrape(ctx, match.ID)
		if err != nil {
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

		itemData, _ := json.Marshal(map[string]any{
			"index":         i + 1,
			"total":         len(validMatches),
			"percent":       (i + 1) * 100 / len(validMatches),
			"movie_id":      match.ID,
			"target_folder": res.TargetFolder,
			"target_video":  res.TargetVideo,
			"success":       success,
			"dry_run":       dryRun,
			"message":       fmt.Sprintf("จัดระเบียบ %s สำเร็จ!", match.ID),
		})
		fmt.Fprintf(w, "event: item\ndata: %s\n\n", itemData)
		flusher.Flush()
	}

	doneData, _ := json.Marshal(map[string]any{
		"phase":         "done",
		"total":         len(validMatches),
		"success_count": successCount,
		"message":       fmt.Sprintf("จัดระเบียบเสร็จสมบูรณ์ %d/%d ไฟล์", successCount, len(validMatches)),
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

	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Second)
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

// detectOrganizedStatus checks which movies have already been organized into NAS Jellyfin structure.
func detectOrganizedStatus(targetDir string, matches []matcher.MatchResult, database *db.DB) map[string]bool {
	organizedMap := make(map[string]bool)
	if database != nil {
		if orgs, err := database.GetOrganizedMap(); err == nil && orgs != nil {
			organizedMap = orgs
		}
	}

	checkDest := filepath.Join(targetDir, "JAV_Library")
	for _, m := range matches {
		if m.ID == "" || organizedMap[m.ID] {
			continue
		}
		// If video file itself is already inside a folder with .nfo, it's organized
		dir := filepath.Dir(m.File.Path)
		nfoPath := filepath.Join(dir, m.ID+".nfo")
		if _, err := os.Stat(nfoPath); err == nil {
			organizedMap[m.ID] = true
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
							if strings.Contains(strings.ToUpper(sub.Name()), m.ID) {
								organizedMap[m.ID] = true
								break
							}
						}
					}
				}
			}
		}
	}

	return organizedMap
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
