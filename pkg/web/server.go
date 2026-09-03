package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/actress"
	"github.com/dagalp/r19dev-scraper/pkg/cache"
	"github.com/dagalp/r19dev-scraper/pkg/db"
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
	mux.HandleFunc("/api/movie/", s.handleMovie)
	mux.HandleFunc("/api/scrape/", s.handleScrape)
	mux.HandleFunc("/api/images/", s.handleImage)
	mux.HandleFunc("/api/actresses", s.handleActresses)
	mux.HandleFunc("/api/actresses/follow", s.handleActressFollow)
	mux.HandleFunc("/api/actresses/unfollow", s.handleActressUnfollow)
	mux.HandleFunc("/api/actresses/releases", s.handleActressReleases)
	mux.HandleFunc("/api/organize", s.handleOrganize)

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
	if target == "" {
		target = s.targetDir
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
		"target_dir":  target,
		"matches":     matches,
		"metadata":    metadataMap,
		"user_states": userStatesMap,
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
		if srcDir == "" {
			srcDir = s.targetDir
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
			}
		}
	}

	writeJSON(w, map[string]any{
		"results":       results,
		"success_count": successCount,
		"total_count":   len(results),
	})
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
