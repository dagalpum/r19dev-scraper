package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/scraper"
	_ "modernc.org/sqlite"
)

// ActressRecord stores tracked actress metadata.
type ActressRecord struct {
	Name          string     `json:"name"`
	JaName        string     `json:"ja_name"`
	ImageURL      string     `json:"image_url"`
	FollowedAt    time.Time  `json:"followed_at"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	Notes         string     `json:"notes"`
	R18ID         int        `json:"r18_id,omitempty"`
	TotalMovies   int        `json:"total_movies,omitempty"`
	Downloaded    int        `json:"downloaded,omitempty"`
	Watched       int        `json:"watched,omitempty"`
}

// UserState stores user watch history, rating, and favorite status.
type UserState struct {
	MovieID      string    `json:"movie_id"`
	IsDownloaded bool      `json:"is_downloaded"`
	IsWatched    bool      `json:"is_watched"`
	UserRating   int       `json:"user_rating"` // 0 to 5 stars
	IsFavorite   bool      `json:"is_favorite"`
	Notes        string    `json:"notes"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LibraryFileRecord represents a scanned or organized video file on disk.
type LibraryFileRecord struct {
	FilePath      string    `json:"file_path"`
	MovieID       string    `json:"movie_id"`
	SizeBytes     int64     `json:"size_bytes"`
	IsMultiPart   bool      `json:"is_multi_part"`
	PartNumber    int       `json:"part_number"`
	OrganizedPath string    `json:"organized_path"`
	ScannedAt     time.Time `json:"scanned_at"`
}

// OperationRecord stores audit log and status of batch operations (e.g. organize, scrape).
type OperationRecord struct {
	ID           int64     `json:"id"`
	Operation    string    `json:"operation"`   // 'organize', 'scrape'
	TargetPath   string    `json:"target_path"` // source/destination or movie ID
	TotalItems   int       `json:"total_items"`
	SuccessCount int       `json:"success_count"`
	FailCount    int       `json:"fail_count"`
	DryRun       bool      `json:"dry_run"`
	LogText      string    `json:"log_text,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// DB wraps SQLite operations for R19DEV.
type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

var (
	defaultDB *DB
	dbOnce    sync.Once
)

// Default returns the singleton global DB instance located in ~/.cache/r19dev/r19dev.db.
func Default() (*DB, error) {
	var initErr error
	dbOnce.Do(func() {
		baseDir, err := os.UserCacheDir()
		if err != nil || baseDir == "" {
			home, hErr := os.UserHomeDir()
			if hErr == nil {
				baseDir = filepath.Join(home, ".cache")
			} else {
				baseDir = "."
			}
		}
		dbDir := filepath.Join(baseDir, "r19dev")
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			initErr = fmt.Errorf("failed to create database directory: %w", err)
			return
		}
		dbPath := filepath.Join(dbDir, "r19dev.db")
		d, err := Open(dbPath)
		if err != nil {
			initErr = err
			return
		}
		defaultDB = d
	})
	if initErr != nil {
		return nil, initErr
	}
	return defaultDB, nil
}

// Open opens or creates a new SQLite database at dbPath.
func Open(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	d := &DB{conn: conn}
	if err := d.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize sqlite schema: %w", err)
	}

	return d, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.Close()
}

// Query executes a query that returns rows.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.conn.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.conn.QueryRow(query, args...)
}

func (d *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS actresses (
		name TEXT PRIMARY KEY,
		ja_name TEXT,
		image_url TEXT,
		followed_at DATETIME,
		last_checked_at DATETIME,
		notes TEXT,
		r18_id INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS movies (
		id TEXT PRIMARY KEY,
		combined_id TEXT,
		title TEXT,
		original_title TEXT,
		maker TEXT,
		label TEXT,
		director TEXT,
		release_date TEXT,
		runtime_minutes INTEGER,
		cover_url TEXT,
		poster_url TEXT,
		trailer_url TEXT,
		actresses_json TEXT,
		genres_json TEXT,
		screenshots_json TEXT,
		scraped_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS user_state (
		movie_id TEXT PRIMARY KEY,
		is_downloaded BOOLEAN DEFAULT 0,
		is_watched BOOLEAN DEFAULT 0,
		user_rating INTEGER DEFAULT 0,
		is_favorite BOOLEAN DEFAULT 0,
		notes TEXT,
		updated_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS library_files (
		file_path TEXT PRIMARY KEY,
		movie_id TEXT,
		size_bytes INTEGER,
		is_multi_part BOOLEAN DEFAULT 0,
		part_number INTEGER DEFAULT 0,
		organized_path TEXT,
		scanned_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS organized_movies (
		movie_id TEXT PRIMARY KEY,
		target_folder TEXT,
		target_video TEXT,
		organized_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS operation_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		operation TEXT NOT NULL,
		target_path TEXT,
		total_items INTEGER DEFAULT 0,
		success_count INTEGER DEFAULT 0,
		fail_count INTEGER DEFAULT 0,
		dry_run BOOLEAN DEFAULT 0,
		log_text TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_movies_maker ON movies(maker);
	CREATE INDEX IF NOT EXISTS idx_movies_release ON movies(release_date);
	CREATE INDEX IF NOT EXISTS idx_library_movie_id ON library_files(movie_id);
	CREATE INDEX IF NOT EXISTS idx_operation_created ON operation_history(created_at DESC);
	`
	_, err := d.conn.Exec(schema)
	if err != nil {
		return err
	}
	// Safe migration for existing installations
	_, _ = d.conn.Exec("ALTER TABLE actresses ADD COLUMN r18_id INTEGER DEFAULT 0;")
	_ = d.backfillActressR18IDs()
	return nil
}

func (d *DB) backfillActressR18IDs() error {
	rows, err := d.conn.Query("SELECT actresses_json FROM movies WHERE actresses_json != '' AND actresses_json != '[]'")
	if err != nil {
		return err
	}
	defer rows.Close()

	type actHelper struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		JaName   string `json:"ja_name"`
	}

	for rows.Next() {
		var actJSON string
		if err := rows.Scan(&actJSON); err != nil {
			continue
		}
		var acts []actHelper
		if err := json.Unmarshal([]byte(actJSON), &acts); err != nil {
			continue
		}
		for _, act := range acts {
			if act.ID != 0 && act.Name != "" {
				_, _ = d.conn.Exec("UPDATE actresses SET r18_id = ? WHERE (name = ? COLLATE NOCASE OR ja_name = ?) AND (r18_id IS NULL OR r18_id = 0)", act.ID, act.Name, act.JaName)
			}
		}
	}
	return rows.Err()
}

// --- Actress Operations ---

// FollowActress adds an actress to the tracking list.
func (d *DB) FollowActress(name, jaName, imageURL string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("actress name cannot be empty")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO actresses (name, ja_name, image_url, followed_at, notes)
	VALUES (?, ?, ?, ?, '')
	ON CONFLICT(name) DO UPDATE SET
		ja_name = CASE WHEN excluded.ja_name != '' THEN excluded.ja_name ELSE actresses.ja_name END,
		image_url = CASE WHEN excluded.image_url != '' THEN excluded.image_url ELSE actresses.image_url END;
	`
	_, err := d.conn.Exec(query, name, jaName, imageURL, time.Now())
	return err
}

// UnfollowActress removes an actress from the tracking list.
func (d *DB) UnfollowActress(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM actresses WHERE name = ? COLLATE NOCASE", name)
	return err
}

// IsActressFollowed checks if an actress is currently followed.
func (d *DB) IsActressFollowed(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM actresses WHERE name = ? COLLATE NOCASE", name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListFollowedActresses returns all tracked actresses.
func (d *DB) ListFollowedActresses() ([]ActressRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query("SELECT name, ja_name, image_url, followed_at, last_checked_at, notes, COALESCE(r18_id, 0) FROM actresses ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActressRecord
	for rows.Next() {
		var a ActressRecord
		var lastChecked sql.NullTime
		if err := rows.Scan(&a.Name, &a.JaName, &a.ImageURL, &a.FollowedAt, &lastChecked, &a.Notes, &a.R18ID); err != nil {
			return nil, err
		}
		if lastChecked.Valid {
			a.LastCheckedAt = &lastChecked.Time
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// UpdateActressLastChecked updates the last_checked_at timestamp for an actress.
func (d *DB) UpdateActressLastChecked(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("UPDATE actresses SET last_checked_at = ? WHERE name = ? COLLATE NOCASE", time.Now(), name)
	return err
}

// --- Movie Metadata Operations ---

// SaveMovie inserts or updates a movie record in the database.
func (d *DB) SaveMovie(m *scraper.Movie) error {
	if m == nil || m.ID == "" {
		return fmt.Errorf("invalid movie record")
	}

	actressesJSON, _ := json.Marshal(m.Actresses)
	genresJSON, _ := json.Marshal(m.Genres)
	screenshotsJSON, _ := json.Marshal(m.SampleScreenshots)

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO movies (
		id, combined_id, title, original_title, maker, label, director,
		release_date, runtime_minutes, cover_url, poster_url, trailer_url,
		actresses_json, genres_json, screenshots_json, scraped_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		combined_id = excluded.combined_id,
		title = excluded.title,
		original_title = excluded.original_title,
		maker = excluded.maker,
		label = excluded.label,
		director = excluded.director,
		release_date = excluded.release_date,
		runtime_minutes = excluded.runtime_minutes,
		cover_url = excluded.cover_url,
		poster_url = excluded.poster_url,
		trailer_url = excluded.trailer_url,
		actresses_json = excluded.actresses_json,
		genres_json = excluded.genres_json,
		screenshots_json = excluded.screenshots_json,
		scraped_at = excluded.scraped_at;
	`

	_, err := d.conn.Exec(query,
		m.ID, m.CombinedID, m.Title, m.OriginalTitle, m.Maker, m.Label, m.Director,
		m.ReleaseDate, m.RuntimeMinutes, m.CoverURL, m.PosterURL, m.TrailerURL,
		string(actressesJSON), string(genresJSON), string(screenshotsJSON), time.Now(),
	)
	if err == nil {
		for _, act := range m.Actresses {
			if act.ID != 0 && act.Name != "" {
				_, _ = d.conn.Exec("UPDATE actresses SET r18_id = ? WHERE (name = ? COLLATE NOCASE OR ja_name = ?) AND (r18_id IS NULL OR r18_id = 0)", act.ID, act.Name, act.JaName)
			}
		}
	}
	return err
}

// GetMovie retrieves a movie record by JAV ID.
func (d *DB) GetMovie(id string) (*scraper.Movie, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return nil, fmt.Errorf("movie ID is empty")
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT id, combined_id, title, original_title, maker, label, director,
	       release_date, runtime_minutes, cover_url, poster_url, trailer_url,
	       actresses_json, genres_json, screenshots_json, scraped_at
	FROM movies WHERE id = ? OR combined_id = ?
	`
	combinedID := scraper.NormalizeToCombinedID(id)

	var m scraper.Movie
	var actJSON, genJSON, scJSON string
	err := d.conn.QueryRow(query, id, combinedID).Scan(
		&m.ID, &m.CombinedID, &m.Title, &m.OriginalTitle, &m.Maker, &m.Label, &m.Director,
		&m.ReleaseDate, &m.RuntimeMinutes, &m.CoverURL, &m.PosterURL, &m.TrailerURL,
		&actJSON, &genJSON, &scJSON, &m.ScrapedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(actJSON), &m.Actresses)
	_ = json.Unmarshal([]byte(genJSON), &m.Genres)
	_ = json.Unmarshal([]byte(scJSON), &m.SampleScreenshots)

	return &m, nil
}

// --- User State & Rating Operations ---

// SetUserState updates or inserts watch, rating, and favorite status.
func (d *DB) SetUserState(state UserState) error {
	state.MovieID = strings.ToUpper(strings.TrimSpace(state.MovieID))
	if state.MovieID == "" {
		return fmt.Errorf("movie ID cannot be empty")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO user_state (movie_id, is_downloaded, is_watched, user_rating, is_favorite, notes, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(movie_id) DO UPDATE SET
		is_downloaded = excluded.is_downloaded,
		is_watched = excluded.is_watched,
		user_rating = excluded.user_rating,
		is_favorite = excluded.is_favorite,
		notes = excluded.notes,
		updated_at = excluded.updated_at;
	`
	_, err := d.conn.Exec(query,
		state.MovieID, state.IsDownloaded, state.IsWatched, state.UserRating, state.IsFavorite, state.Notes, time.Now(),
	)
	return err
}

// GetUserState retrieves user status for a movie.
func (d *DB) GetUserState(movieID string) (*UserState, error) {
	movieID = strings.ToUpper(strings.TrimSpace(movieID))
	if movieID == "" {
		return nil, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var state UserState
	query := `SELECT movie_id, is_downloaded, is_watched, user_rating, is_favorite, notes, updated_at FROM user_state WHERE movie_id = ?`
	err := d.conn.QueryRow(query, movieID).Scan(
		&state.MovieID, &state.IsDownloaded, &state.IsWatched, &state.UserRating, &state.IsFavorite, &state.Notes, &state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// ToggleWatched toggles watched status for a movie ID.
func (d *DB) ToggleWatched(movieID string) (bool, error) {
	st, err := d.GetUserState(movieID)
	if err != nil {
		return false, err
	}
	newWatched := true
	if st != nil {
		newWatched = !st.IsWatched
		st.IsWatched = newWatched
		st.UpdatedAt = time.Now()
		return newWatched, d.SetUserState(*st)
	}
	return true, d.SetUserState(UserState{
		MovieID:   movieID,
		IsWatched: true,
		UpdatedAt: time.Now(),
	})
}

// SetRating sets 1-5 star rating for a movie ID.
func (d *DB) SetRating(movieID string, rating int) error {
	if rating < 0 {
		rating = 0
	}
	if rating > 5 {
		rating = 5
	}
	st, err := d.GetUserState(movieID)
	if err != nil {
		return err
	}
	if st != nil {
		st.UserRating = rating
		st.UpdatedAt = time.Now()
		return d.SetUserState(*st)
	}
	return d.SetUserState(UserState{
		MovieID:    movieID,
		UserRating: rating,
		UpdatedAt:  time.Now(),
	})
}

// ToggleFavorite toggles favorite status for a movie ID.
func (d *DB) ToggleFavorite(movieID string) (bool, error) {
	st, err := d.GetUserState(movieID)
	if err != nil {
		return false, err
	}
	newFav := true
	if st != nil {
		newFav = !st.IsFavorite
		st.IsFavorite = newFav
		st.UpdatedAt = time.Now()
		return newFav, d.SetUserState(*st)
	}
	return true, d.SetUserState(UserState{
		MovieID:    movieID,
		IsFavorite: true,
		UpdatedAt:  time.Now(),
	})
}

// --- Library Files Operations ---

// UpsertLibraryFile records or updates a scanned video file in the database.
func (d *DB) UpsertLibraryFile(rec LibraryFileRecord) error {
	if rec.FilePath == "" || rec.MovieID == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO library_files (file_path, movie_id, size_bytes, is_multi_part, part_number, organized_path, scanned_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(file_path) DO UPDATE SET
		movie_id = excluded.movie_id,
		size_bytes = excluded.size_bytes,
		is_multi_part = excluded.is_multi_part,
		part_number = excluded.part_number,
		organized_path = excluded.organized_path,
		scanned_at = excluded.scanned_at;
	`
	_, err := d.conn.Exec(query,
		rec.FilePath, rec.MovieID, rec.SizeBytes, rec.IsMultiPart, rec.PartNumber, rec.OrganizedPath, time.Now(),
	)
	return err
}

// HasMovieInLibrary checks if a movie ID exists in scanned library files.
func (d *DB) HasMovieInLibrary(movieID string) (bool, error) {
	movieID = strings.ToUpper(strings.TrimSpace(movieID))
	if movieID == "" {
		return false, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM library_files WHERE movie_id = ?", movieID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetOrganized records a movie as successfully organized to the target path.
func (d *DB) SetOrganized(movieID, targetFolder, targetVideo string) error {
	movieID = strings.ToUpper(strings.TrimSpace(movieID))
	if movieID == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO organized_movies (movie_id, target_folder, target_video, organized_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(movie_id) DO UPDATE SET
		target_folder = excluded.target_folder,
		target_video = excluded.target_video,
		organized_at = excluded.organized_at;
	`
	_, err := d.conn.Exec(query, movieID, targetFolder, targetVideo, time.Now())
	return err
}

// GetOrganizedMap returns a map of all movie IDs that have been organized.
func (d *DB) GetOrganizedMap() (map[string]bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`SELECT movie_id FROM organized_movies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil && id != "" {
			result[strings.ToUpper(id)] = true
		}
	}
	return result, rows.Err()
}

// IsOrganized checks if a movie ID has been organized into Jellyfin NAS.
func (d *DB) IsOrganized(movieID string) (bool, error) {
	movieID = strings.ToUpper(strings.TrimSpace(movieID))
	if movieID == "" {
		return false, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM organized_movies WHERE movie_id = ?`, movieID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetOrganizedDetails returns target_folder and target_video for a movie ID.
func (d *DB) GetOrganizedDetails(movieID string) (targetFolder, targetVideo string, err error) {
	movieID = strings.ToUpper(strings.TrimSpace(movieID))
	if movieID == "" {
		return "", "", nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	err = d.conn.QueryRow(`SELECT target_folder, target_video FROM organized_movies WHERE movie_id = ?`, movieID).Scan(&targetFolder, &targetVideo)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return targetFolder, targetVideo, err
}

// GetOrganizedFolderMap returns a map of movie_id -> target_folder.
func (d *DB) GetOrganizedFolderMap() (map[string]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`SELECT movie_id, target_folder FROM organized_movies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var id, folder string
		if err := rows.Scan(&id, &folder); err == nil && id != "" {
			result[strings.ToUpper(id)] = folder
		}
	}
	return result, rows.Err()
}

// --- Operation History / Log Audit Operations ---

// AddOperationHistory saves an operation run to SQLite and automatically prunes entries older than 30 days.
func (d *DB) AddOperationHistory(op, target string, total, success, fail int, dryRun bool, logText string) (int64, error) {
	if d == nil || d.conn == nil {
		return 0, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO operation_history (operation, target_path, total_items, success_count, fail_count, dry_run, log_text, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := d.conn.Exec(query, op, target, total, success, fail, dryRun, logText, time.Now())
	if err != nil {
		return 0, err
	}

	// Auto-prune: Keep maximum 100 operations or records from the last 30 days to avoid clutter
	_, _ = d.conn.Exec(`DELETE FROM operation_history WHERE created_at < datetime('now', '-30 days')`)
	_, _ = d.conn.Exec(`DELETE FROM operation_history WHERE id NOT IN (SELECT id FROM operation_history ORDER BY id DESC LIMIT 100)`)

	return res.LastInsertId()
}

// GetOperationHistory returns recent operation records. If withLogs is false, log_text is omitted for speed.
func (d *DB) GetOperationHistory(limit int, withLogs bool) ([]OperationRecord, error) {
	if d == nil || d.conn == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var query string
	if withLogs {
		query = `SELECT id, operation, target_path, total_items, success_count, fail_count, dry_run, log_text, created_at 
		         FROM operation_history ORDER BY id DESC LIMIT ?`
	} else {
		query = `SELECT id, operation, target_path, total_items, success_count, fail_count, dry_run, '', created_at 
		         FROM operation_history ORDER BY id DESC LIMIT ?`
	}

	rows, err := d.conn.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []OperationRecord
	for rows.Next() {
		var rec OperationRecord
		var createdAt time.Time
		if err := rows.Scan(&rec.ID, &rec.Operation, &rec.TargetPath, &rec.TotalItems, &rec.SuccessCount, &rec.FailCount, &rec.DryRun, &rec.LogText, &createdAt); err == nil {
			rec.CreatedAt = createdAt
			records = append(records, rec)
		}
	}
	return records, rows.Err()
}

// GetOperationDetail returns a single operation record with full log_text.
func (d *DB) GetOperationDetail(id int64) (*OperationRecord, error) {
	if d == nil || d.conn == nil {
		return nil, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	var rec OperationRecord
	var createdAt time.Time
	query := `SELECT id, operation, target_path, total_items, success_count, fail_count, dry_run, log_text, created_at 
	          FROM operation_history WHERE id = ?`
	err := d.conn.QueryRow(query, id).Scan(&rec.ID, &rec.Operation, &rec.TargetPath, &rec.TotalItems, &rec.SuccessCount, &rec.FailCount, &rec.DryRun, &rec.LogText, &createdAt)
	if err != nil {
		return nil, err
	}
	rec.CreatedAt = createdAt
	return &rec, nil
}

// ClearOperationHistory clears all recorded operation logs.
func (d *DB) ClearOperationHistory() error {
	if d == nil || d.conn == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec(`DELETE FROM operation_history`)
	return err
}
