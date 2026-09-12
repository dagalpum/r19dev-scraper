package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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
	path string
	mu   sync.RWMutex
}

var (
	defaultDB *DB
	dbOnce    sync.Once
)

// Default returns the singleton global DB instance located in ~/Library/Application Support/r19dev/r19dev.db (macOS)
// or ~/.config/r19dev/r19dev.db (Linux).
func Default() (*DB, error) {
	var initErr error
	dbOnce.Do(func() {
		// Prefer UserConfigDir (~/Library/Application Support on macOS, ~/.config on Linux)
		baseDir, err := os.UserConfigDir()
		if err != nil || baseDir == "" {
			home, hErr := os.UserHomeDir()
			if hErr == nil {
				baseDir = filepath.Join(home, "Library", "Application Support")
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

		// Seamless Migration from legacy Cache directory (~/Library/Caches or ~/.cache)
		if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
			legacyDir, lErr := os.UserCacheDir()
			if lErr == nil && legacyDir != "" {
				legacyDBPath := filepath.Join(legacyDir, "r19dev", "r19dev.db")
				if _, legStatErr := os.Stat(legacyDBPath); legStatErr == nil {
					// Copy legacy DB to new Application Support location
					if data, readErr := os.ReadFile(legacyDBPath); readErr == nil {
						if writeErr := os.WriteFile(dbPath, data, 0o644); writeErr == nil {
							fmt.Printf("📦 [DB Migration] Safely migrated database from %s -> %s\n", legacyDBPath, dbPath)
						}
					}
				}
			}
		}

		// Fallback Restore or Sync from NAS backup
		nasBackupCandidates := []string{
			"/Volumes/home/BT/organized/.r19dev_backup.db",
			"/Volumes/home/BT/2026/organized/.r19dev_backup.db",
		}
		_ = SyncWithBackupCandidates(dbPath, nasBackupCandidates)

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

	d := &DB{conn: conn, path: dbPath}
	if err := d.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize sqlite schema: %w", err)
	}

	return d, nil
}

// Path returns the filesystem path of the SQLite database.
func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// BackupTo creates an atomic, crash-consistent SQLite backup snapshot at targetFile.
// It snapshots to a local temp file via VACUUM INTO first (avoiding SMB/NFS fsctl/locking issues on macOS),
// then copies the clean database file to targetFile.
func (d *DB) BackupTo(targetFile string) error {
	if d == nil || d.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	targetFile = filepath.Clean(targetFile)
	if err := os.MkdirAll(filepath.Dir(targetFile), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for backup: %w", err)
	}

	// Always VACUUM INTO a local temporary file first to avoid SMB/NFS fsctl issues on macOS
	tmpFile, err := os.CreateTemp("", "r19dev_backup_*.db")
	if err != nil {
		return fmt.Errorf("failed to create temp backup file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(tmpPath) // VACUUM INTO requires target file not exist
	defer os.Remove(tmpPath)

	if _, err := d.conn.Exec("VACUUM INTO ?", tmpPath); err != nil {
		return fmt.Errorf("VACUUM INTO failed: %w", err)
	}

	// Copy atomic snapshot to target destination (works seamlessly across local, SMB, NFS)
	src, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to open temp backup: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create target backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy backup to target: %w", err)
	}

	return nil
}

// GetLatestActivityTime returns the newest activity timestamp recorded in the database.
// It inspects operation_history.created_at, user_state.updated_at, organized_movies.organized_at,
// and movies.scraped_at.
func (d *DB) GetLatestActivityTime() (time.Time, error) {
	if d == nil || d.conn == nil {
		return time.Time{}, fmt.Errorf("database not initialized")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	return queryLatestActivity(d.conn)
}

// InspectLatestActivityTime opens an existing SQLite database at dbPath in read-only mode
// and returns its latest activity timestamp.
func InspectLatestActivityTime(dbPath string) (time.Time, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return time.Time{}, err
	}
	conn, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	return queryLatestActivity(conn)
}

func queryLatestActivity(conn *sql.DB) (time.Time, error) {
	query := `
	SELECT MAX(ts) FROM (
		SELECT MAX(created_at) AS ts FROM operation_history
		UNION ALL
		SELECT MAX(updated_at) AS ts FROM user_state
		UNION ALL
		SELECT MAX(organized_at) AS ts FROM organized_movies
		UNION ALL
		SELECT MAX(scraped_at) AS ts FROM movies
	)`

	var latest sql.NullString
	if err := conn.QueryRow(query).Scan(&latest); err != nil {
		return time.Time{}, err
	}
	str := strings.TrimSpace(latest.String)
	if idx := strings.Index(str, " m="); idx != -1 {
		str = str[:idx]
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, str); err == nil {
			return t, nil
		}
	}
	return time.Time{}, nil
}

// SyncAction represents the outcome of database comparison.
type SyncAction string

const (
	SyncActionNone       SyncAction = "none"
	SyncActionRestored   SyncAction = "restored"
	SyncActionSynced     SyncAction = "synced"
	SyncActionLocalNewer SyncAction = "local_is_newer"
)

// SyncResult details the outcome of comparing and syncing local DB with a backup candidate.
type SyncResult struct {
	Action     SyncAction `json:"action"`
	LocalPath  string     `json:"local_path"`
	Candidate  string     `json:"candidate_path"`
	LocalTime  time.Time  `json:"local_time,omitempty"`
	BackupTime time.Time  `json:"backup_time,omitempty"`
	Message    string     `json:"message"`
}

// SyncWithBackupCandidates compares localDBPath with candidate backup files.
// If localDBPath does not exist, it restores from the newest valid candidate.
// If localDBPath exists, it checks if any candidate has newer database activity (based on MAX(ts) of internal records).
// If a candidate is newer, it backs up localDBPath to localDBPath + ".bak" and syncs from candidate.
func SyncWithBackupCandidates(localDBPath string, candidates []string) SyncResult {
	res := SyncResult{
		Action:    SyncActionNone,
		LocalPath: localDBPath,
		Message:   "No sync required",
	}

	// 1. Check if local DB does not exist -> Restore
	if _, statErr := os.Stat(localDBPath); os.IsNotExist(statErr) {
		for _, cand := range candidates {
			if _, bErr := os.Stat(cand); bErr == nil {
				candTime, _ := InspectLatestActivityTime(cand)
				if data, rErr := os.ReadFile(cand); rErr == nil {
					if wErr := os.WriteFile(localDBPath, data, 0o644); wErr == nil {
						res.Action = SyncActionRestored
						res.Candidate = cand
						res.BackupTime = candTime
						res.Message = fmt.Sprintf("Restored database from NAS backup: %s", cand)
						fmt.Printf("📦 [DB Restore] %s\n", res.Message)
						return res
					}
				}
			}
		}
		res.Message = "No valid backup candidate found for restore"
		return res
	}

	// 2. Local DB exists -> Compare internal activity timestamps
	localTime, lErr := InspectLatestActivityTime(localDBPath)
	if lErr != nil {
		localTime = time.Time{}
	}
	res.LocalTime = localTime

	for _, cand := range candidates {
		if _, bErr := os.Stat(cand); bErr == nil {
			candTime, nErr := InspectLatestActivityTime(cand)
			if nErr == nil && !candTime.IsZero() {
				if candTime.After(localTime) {
					// Candidate has newer data! Create safety .bak of local DB
					if localData, rErr := os.ReadFile(localDBPath); rErr == nil {
						_ = os.WriteFile(localDBPath+".bak", localData, 0o644)
					}
					if candData, rErr := os.ReadFile(cand); rErr == nil {
						if wErr := os.WriteFile(localDBPath, candData, 0o644); wErr == nil {
							_ = os.Remove(localDBPath + "-wal")
							_ = os.Remove(localDBPath + "-shm")
							res.Action = SyncActionSynced
							res.Candidate = cand
							res.BackupTime = candTime
							res.Message = fmt.Sprintf("Synced newer activity from %s (%s > %s)",
								cand, candTime.Format("2006-01-02 15:04:05"), localTime.Format("2006-01-02 15:04:05"))
							fmt.Printf("📦 [DB Sync] %s\n", res.Message)
							return res
						}
					}
				} else {
					res.Action = SyncActionLocalNewer
					res.Candidate = cand
					res.BackupTime = candTime
					res.Message = fmt.Sprintf("Local database is newer or equal (%s >= %s)",
						localTime.Format("2006-01-02 15:04:05"), candTime.Format("2006-01-02 15:04:05"))
				}
			}
		}
	}

	return res
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
	_, _ = d.purgePromotionalVariantsLocked()
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
		combined_id = CASE WHEN excluded.combined_id != '' THEN excluded.combined_id ELSE movies.combined_id END,
		title = CASE WHEN excluded.title != '' THEN excluded.title ELSE movies.title END,
		original_title = CASE WHEN excluded.original_title != '' THEN excluded.original_title ELSE movies.original_title END,
		maker = CASE WHEN excluded.maker != '' THEN excluded.maker ELSE movies.maker END,
		label = CASE WHEN excluded.label != '' THEN excluded.label ELSE movies.label END,
		director = CASE WHEN excluded.director != '' THEN excluded.director ELSE movies.director END,
		release_date = CASE WHEN excluded.release_date != '' THEN excluded.release_date ELSE movies.release_date END,
		runtime_minutes = CASE WHEN excluded.runtime_minutes > 0 THEN excluded.runtime_minutes ELSE movies.runtime_minutes END,
		cover_url = CASE WHEN excluded.cover_url != '' THEN excluded.cover_url ELSE movies.cover_url END,
		poster_url = CASE WHEN excluded.poster_url != '' THEN excluded.poster_url ELSE movies.poster_url END,
		trailer_url = CASE WHEN excluded.trailer_url != '' THEN excluded.trailer_url ELSE movies.trailer_url END,
		actresses_json = CASE WHEN excluded.actresses_json != '' AND excluded.actresses_json != '[]' THEN excluded.actresses_json ELSE movies.actresses_json END,
		genres_json = CASE WHEN excluded.genres_json != '' AND excluded.genres_json != '[]' THEN excluded.genres_json ELSE movies.genres_json END,
		screenshots_json = CASE WHEN excluded.screenshots_json != '' AND excluded.screenshots_json != '[]' THEN excluded.screenshots_json ELSE movies.screenshots_json END,
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

// PurgePromotionalVariants deletes phantom promotional / set product duplicate SKUs
// (e.g. C9FWAY095, E9FWAY095, S9FWAY095, Special Offers tag) that are not owned or tracked in user state.
func (d *DB) PurgePromotionalVariants() (int64, error) {
	if d == nil || d.conn == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.purgePromotionalVariantsLocked()
}

func (d *DB) purgePromotionalVariantsLocked() (int64, error) {
	query := `
	DELETE FROM movies
	WHERE (
		id GLOB '[CESNK9]9*'
		OR genres_json LIKE '%Special Offers And Set Products%'
		OR genres_json LIKE '%Includes Event Participation Rights%'
		OR genres_json LIKE '%Collection Of Photographs%'
		OR title LIKE '%オンラインサイン会%'
		OR title LIKE '%購入特典付き%'
		OR title LIKE '%購入特典付%'
		OR title LIKE '%チェキ付き%'
		OR title LIKE '%チェキ付%'
		OR title LIKE '%チェキセット%'
		OR id LIKE 'KCKC%'
		OR id LIKE 'MLTN%'
		OR title LIKE '%カチコチTV%'
		OR title LIKE '%未公開映像収録%'
		OR title LIKE '%ディレクターズカット%'
	)
	AND id NOT IN (SELECT movie_id FROM user_state)
	AND id NOT IN (SELECT movie_id FROM library_files)
	AND id NOT IN (SELECT movie_id FROM organized_movies);
	`
	res, err := d.conn.Exec(query)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

