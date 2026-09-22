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

// MovieActressRecord stores a relational mapping between a movie and an actress.
type MovieActressRecord struct {
	MovieID       string    `json:"movie_id"`
	ActressID     string    `json:"actress_id"`
	ActressName   string    `json:"actress_name"`
	ActressJaName string    `json:"actress_ja_name"`
	Source        string    `json:"source"` // 'dmm', 'fc2', 'custom', 'folder'
	CreatedAt     time.Time `json:"created_at"`
}

// GenerateActressID produces a deterministic, namespaced actress ID.
// - If dmmID > 0: "dmm:{dmmID}"
// - If name starts with "FC2" or seller tag: "seller:{slug}" or "fc2:{slug}"
// - Otherwise: "custom:{slug}"
func GenerateActressID(dmmID int, name string) string {
	if dmmID > 0 {
		return fmt.Sprintf("dmm:%d", dmmID)
	}
	clean := strings.TrimSpace(strings.ToLower(name))
	if clean == "" || clean == "unknown actress" || clean == "unknown" || clean == "素人" {
		return "custom:unknown"
	}
	var b strings.Builder
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('_')
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "custom:unknown"
	}
	if strings.HasPrefix(slug, "fc2") {
		return "fc2:" + slug
	}
	if strings.HasPrefix(slug, "seller_") || strings.HasPrefix(slug, "good0") {
		return "seller:" + slug
	}
	return "custom:" + slug
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
	openPath := dbPath
	// If path is on a network mount (e.g. /Volumes/), copy to local temp file first to avoid Darwin smbfs fsctl limitations
	if strings.HasPrefix(dbPath, "/Volumes/") {
		tmp, err := os.CreateTemp("", "r19dev_inspect_*.db")
		if err == nil {
			tempFile := tmp.Name()
			src, sErr := os.Open(dbPath)
			if sErr == nil {
				_, _ = io.Copy(tmp, src)
				_ = src.Close()
			}
			_ = tmp.Close()
			openPath = tempFile
			defer os.Remove(tempFile)
		}
	}
	conn, err := sql.Open("sqlite", openPath+"?mode=ro&_pragma=query_only(true)")
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
		series TEXT,
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

	CREATE TABLE IF NOT EXISTS movie_actresses (
		movie_id TEXT NOT NULL,
		actress_id TEXT NOT NULL,
		actress_name TEXT NOT NULL,
		actress_ja_name TEXT DEFAULT '',
		source TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (movie_id, actress_id)
	);
	CREATE INDEX IF NOT EXISTS idx_ma_actress_id ON movie_actresses(actress_id);
	CREATE INDEX IF NOT EXISTS idx_ma_movie_id ON movie_actresses(movie_id);
	CREATE INDEX IF NOT EXISTS idx_ma_actress_name ON movie_actresses(actress_name COLLATE NOCASE);

	CREATE TABLE IF NOT EXISTS app_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS download_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		movie_id TEXT NOT NULL,
		combined_id TEXT DEFAULT '',
		movie_title TEXT DEFAULT '',
		cover_url TEXT DEFAULT '',
		actress_name TEXT DEFAULT '',
		torrent_hash TEXT DEFAULT '',
		torrent_title TEXT DEFAULT '',
		torrent_url TEXT DEFAULT '',
		magnet_url TEXT DEFAULT '',
		file_size_bytes INTEGER DEFAULT 0,
		quality_tag TEXT DEFAULT '',
		status TEXT DEFAULT 'queued',
		progress_pct REAL DEFAULT 0.0,
		download_speed INTEGER DEFAULT 0,
		eta_seconds INTEGER DEFAULT 0,
		transmission_id INTEGER DEFAULT 0,
		download_path TEXT DEFAULT '',
		error_message TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_dq_movie_id ON download_queue(movie_id);
	CREATE INDEX IF NOT EXISTS idx_dq_status ON download_queue(status);
	`
	_, err := d.conn.Exec(schema)
	if err != nil {
		return err
	}
	// Safe migration for existing installations
	_, _ = d.conn.Exec("ALTER TABLE movies ADD COLUMN series TEXT;")
	_, _ = d.conn.Exec("ALTER TABLE actresses ADD COLUMN r18_id INTEGER DEFAULT 0;")
	_ = d.backfillActressR18IDs()
	_, _ = d.purgePromotionalVariantsLocked()
	_ = d.autoMigrateMovieActresses()
	return nil
}

func (d *DB) backfillActressR18IDs() error {
	var count int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM actresses WHERE r18_id IS NULL OR r18_id = 0").Scan(&count)
	if count == 0 {
		return nil
	}

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

	rows, err := d.conn.Query("SELECT name, COALESCE(ja_name, ''), COALESCE(image_url, ''), followed_at, last_checked_at, COALESCE(notes, ''), COALESCE(r18_id, 0) FROM actresses ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActressRecord
	for rows.Next() {
		var a ActressRecord
		var lastChecked sql.NullTime
		var followedRaw any
		if err := rows.Scan(&a.Name, &a.JaName, &a.ImageURL, &followedRaw, &lastChecked, &a.Notes, &a.R18ID); err != nil {
			return nil, err
		}
		if lastChecked.Valid {
			a.LastCheckedAt = &lastChecked.Time
		}
		if followedRaw != nil {
			switch v := followedRaw.(type) {
			case time.Time:
				a.FollowedAt = v
			case string:
				t, err := time.Parse("2006-01-02 15:04:05", strings.Split(v, ".")[0])
				if err == nil {
					a.FollowedAt = t
				} else if t2, err2 := time.Parse(time.RFC3339, v); err2 == nil {
					a.FollowedAt = t2
				}
			case []byte:
				str := string(v)
				t, err := time.Parse("2006-01-02 15:04:05", strings.Split(str, ".")[0])
				if err == nil {
					a.FollowedAt = t
				} else if t2, err2 := time.Parse(time.RFC3339, str); err2 == nil {
					a.FollowedAt = t2
				}
			}
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

	// 1. Normalize ID to standard canonical JAV ID format (e.g. 1dldss00559 -> DLDSS-559)
	canonicalID := scraper.NormalizeToCanonicalID(m.ID)
	if canonicalID != "" {
		m.ID = canonicalID
	}
	if m.CombinedID == "" {
		m.CombinedID = scraper.NormalizeToCombinedID(m.ID)
	}

	// 2. Ingestion Gatekeeper: If unowned, reject promotional variants, goods bundles, and omnibus compilations
	d.mu.RLock()
	var ownedCount int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM library_files WHERE movie_id = ? UNION ALL SELECT COUNT(*) FROM organized_movies WHERE movie_id = ?", m.ID, m.ID).Scan(&ownedCount)
	d.mu.RUnlock()

	if ownedCount == 0 {
		if shouldSkip, _ := scraper.IsPromotionalOrOmnibusVariantWithDetails(m.ID, m.Title, m.OriginalTitle, m.Label, m.Series, m.CoverURL, m.Genres, len(m.Actresses)); shouldSkip {
			return nil // Safely discard unowned promotional variant / omnibus compilation!
		}
	}

	actressesJSON, _ := json.Marshal(m.Actresses)
	genresJSON, _ := json.Marshal(m.Genres)
	screenshotsJSON, _ := json.Marshal(m.SampleScreenshots)

	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO movies (
		id, combined_id, title, original_title, maker, label, series, director,
		release_date, runtime_minutes, cover_url, poster_url, trailer_url,
		actresses_json, genres_json, screenshots_json, scraped_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		combined_id = CASE WHEN excluded.combined_id != '' THEN excluded.combined_id ELSE movies.combined_id END,
		title = CASE WHEN excluded.title != '' THEN excluded.title ELSE movies.title END,
		original_title = CASE WHEN excluded.original_title != '' THEN excluded.original_title ELSE movies.original_title END,
		maker = CASE WHEN excluded.maker != '' THEN excluded.maker ELSE movies.maker END,
		label = CASE WHEN excluded.label != '' THEN excluded.label ELSE movies.label END,
		series = CASE WHEN excluded.series != '' THEN excluded.series ELSE movies.series END,
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
		m.ID, m.CombinedID, m.Title, m.OriginalTitle, m.Maker, m.Label, m.Series, m.Director,
		m.ReleaseDate, m.RuntimeMinutes, m.CoverURL, m.PosterURL, m.TrailerURL,
		string(actressesJSON), string(genresJSON), string(screenshotsJSON), time.Now(),
	)
	if err == nil {
		for _, act := range m.Actresses {
			name := strings.TrimSpace(act.Name)
			if name != "" {
				actressID := GenerateActressID(act.ID, name)
				source := "dmm"
				if act.ID == 0 {
					source = "custom"
				}
				maQuery := `
				INSERT INTO movie_actresses (movie_id, actress_id, actress_name, actress_ja_name, source)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(movie_id, actress_id) DO UPDATE SET
					actress_name = excluded.actress_name,
					actress_ja_name = CASE WHEN excluded.actress_ja_name != '' THEN excluded.actress_ja_name ELSE movie_actresses.actress_ja_name END,
					source = excluded.source;
				`
				_, _ = d.conn.Exec(maQuery, m.ID, actressID, name, act.JaName, source)
			}
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
	SELECT id, COALESCE(combined_id, ''), COALESCE(title, ''), COALESCE(original_title, ''),
	       COALESCE(maker, ''), COALESCE(label, ''), COALESCE(series, ''), COALESCE(director, ''),
	       COALESCE(release_date, ''), COALESCE(runtime_minutes, 0),
	       COALESCE(cover_url, ''), COALESCE(poster_url, ''), COALESCE(trailer_url, ''),
	       COALESCE(actresses_json, '[]'), COALESCE(genres_json, '[]'), COALESCE(screenshots_json, '[]'),
	       scraped_at
	FROM movies WHERE id = ? OR combined_id = ?
	`
	combinedID := scraper.NormalizeToCombinedID(id)

	var m scraper.Movie
	var actJSON, genJSON, scJSON string
	var scrapedAt sql.NullTime
	err := d.conn.QueryRow(query, id, combinedID).Scan(
		&m.ID, &m.CombinedID, &m.Title, &m.OriginalTitle, &m.Maker, &m.Label, &m.Series, &m.Director,
		&m.ReleaseDate, &m.RuntimeMinutes, &m.CoverURL, &m.PosterURL, &m.TrailerURL,
		&actJSON, &genJSON, &scJSON, &scrapedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if scrapedAt.Valid {
		m.ScrapedAt = scrapedAt.Time
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
	// 1. Normalize raw content IDs (e.g. 1dldss00559 -> DLDSS-559, n_1544prian048 -> PRIAN-048) for genuine movies
	rows, err := d.conn.Query(`
		SELECT id FROM movies 
		WHERE id GLOB '1[a-zA-Z][a-zA-Z]*' OR id GLOB 'n_[0-9]*' OR id GLOB '13[a-zA-Z]*'
	`)
	if err == nil {
		type renItem struct {
			oldID string
			newID string
		}
		var renames []renItem
		for rows.Next() {
			var oldID string
			if err := rows.Scan(&oldID); err == nil {
				newID := scraper.NormalizeToCanonicalID(oldID)
				if newID != "" && newID != strings.ToUpper(oldID) {
					renames = append(renames, renItem{oldID: oldID, newID: newID})
				}
			}
		}
		_ = rows.Err()
		rows.Close()

		for _, ren := range renames {
			var exists int
			_ = d.conn.QueryRow("SELECT COUNT(*) FROM movies WHERE id = ?", ren.newID).Scan(&exists)
			if exists > 0 {
				// Target already exists, delete old unnormalized duplicate
				_, _ = d.conn.Exec("DELETE FROM movies WHERE id = ?", ren.oldID)
				_, _ = d.conn.Exec("DELETE FROM movie_actresses WHERE movie_id = ?", ren.oldID)
			} else {
				// Rename old ID to canonical ID
				_, _ = d.conn.Exec("UPDATE movies SET id = ? WHERE id = ?", ren.newID, ren.oldID)
				_, _ = d.conn.Exec("UPDATE movie_actresses SET movie_id = ? WHERE movie_id = ?", ren.newID, ren.oldID)
			}
		}
	}

	// 2. Purge unowned promotional variants, goods bundles, and omnibus compilations
	query := `
	DELETE FROM movies
	WHERE (
		id GLOB '[CESNK9]9*'
		OR id GLOB '*TK*'
		OR id GLOB '*EC'
		OR id GLOB '*-EC'
		OR id GLOB '*-T-EC'
		OR id GLOB 'S209*'
		OR id GLOB 'C209*'
		OR id GLOB 'E209*'
		OR id GLOB 'IPOK*'
		OR id GLOB 'IDBD*'
		OR id GLOB 'PBD*'
		OR id GLOB 'OBST*'
		OR id GLOB 'SDDE*'
		OR id GLOB 'MIZD*'
		OR id GLOB 'MIDD*'
		OR id GLOB 'RBB*'
		OR id GLOB 'MKCK*'
		OR id GLOB 'MKMP*'
		OR id GLOB 'OFJE*'
		OR id GLOB '*OFJE*'
		OR id GLOB 'SETH*'
		OR id GLOB '*SETH*'
		OR id GLOB 'OFRF*'
		OR id GLOB 'OFMA*'
		OR id GLOB 'KCKC*'
		OR id GLOB 'MLTN*'
		OR id GLOB 'BMW*'
		OR id GLOB 'B600*'
		OR id GLOB 'D600*'
		OR id GLOB '1[a-zA-Z][a-zA-Z]*'
		OR id GLOB '13[a-zA-Z]*'
		OR id GLOB 'n_[0-9]*'
		OR label LIKE '%BEST%'
		OR label LIKE '%ベスト%'
		OR label LIKE '%総集編%'
		OR label LIKE '%Compilation%'
		OR label LIKE '%Omnibus%'
		OR label LIKE '%Selection%'
		OR label LIKE '%セレクション%'
		OR series LIKE '%BEST%'
		OR series LIKE '%ベスト%'
		OR series LIKE '%総集編%'
		OR series LIKE '%Compilation%'
		OR series LIKE '%Omnibus%'
		OR series LIKE '%Selection%'
		OR series LIKE '%セレクション%'
		OR genres_json LIKE '%Special Offers And Set Products%'
		OR genres_json LIKE '%Includes Event Participation Rights%'
		OR genres_json LIKE '%Collection Of Photographs%'
		OR genres_json LIKE '%"Compilation"%'
		OR genres_json LIKE '%"Omnibus"%'
		OR title LIKE '%オンラインサイン会%'
		OR title LIKE '%購入特典付き%'
		OR title LIKE '%購入特典付%'
		OR title LIKE '%チェキ付き%'
		OR title LIKE '%チェキ付%'
		OR title LIKE '%チェキセット%'
		OR title LIKE '%パンティ%'
		OR original_title LIKE '%パンティ%'
		OR title LIKE '%生写真%'
		OR original_title LIKE '%生写真%'
		OR title LIKE '%ポラロイド%'
		OR original_title LIKE '%ポラロイド%'
		OR title LIKE '%数量限定%'
		OR original_title LIKE '%数量限定%'
		OR title LIKE '%限定特典%'
		OR original_title LIKE '%限定特典%'
		OR title LIKE '%グッズ付き%'
		OR original_title LIKE '%グッズ付き%'
		OR title LIKE '%キーホルダー%'
		OR original_title LIKE '%キーホルダー%'
		OR title LIKE '%BEST%'
		OR original_title LIKE '%BEST%'
		OR title LIKE '%ベスト%'
		OR original_title LIKE '%ベスト%'
		OR title LIKE '%総集編%'
		OR original_title LIKE '%総集編%'
		OR title LIKE '%オムニバス%'
		OR original_title LIKE '%オムニバス%'
		OR title LIKE '%傑作選%'
		OR original_title LIKE '%傑作選%'
		OR title LIKE '%名場面%'
		OR original_title LIKE '%名場面%'
		OR title LIKE '%全集%'
		OR original_title LIKE '%全集%'
		OR title LIKE '%メモリアル%'
		OR original_title LIKE '%メモリアル%'
		OR title LIKE '%プレミアムベスト%'
		OR original_title LIKE '%プレミアムベスト%'
		OR title LIKE '%ベストセレクション%'
		OR original_title LIKE '%ベストセレクション%'
		OR title LIKE '%連発%'
		OR original_title LIKE '%連発%'
		OR title LIKE '%連射%'
		OR original_title LIKE '%連射%'
		OR title LIKE '%時間BOX%'
		OR original_title LIKE '%時間BOX%'
		OR title GLOB '*[1-9]*本番*'
		OR original_title GLOB '*[1-9]*本番*'
		OR title GLOB '*[1-9]*人*'
		OR original_title GLOB '*[1-9]*人*'
		OR title GLOB '*[1-9]*体*'
		OR original_title GLOB '*[1-9]*体*'
		OR title LIKE '%カチコチTV%'
		OR title LIKE '%未公開映像収録%'
		OR title LIKE '%ディレクターズカット%'
	)
	AND id NOT IN (SELECT movie_id FROM user_state WHERE is_watched = 1 OR is_favorite = 1)
	AND id NOT IN (SELECT movie_id FROM library_files WHERE file_path != '')
	AND id NOT IN (SELECT movie_id FROM organized_movies WHERE target_folder != '' OR target_video != '');
	`
	res, err := d.conn.Exec(query)
	if err != nil {
		return 0, err
	}
	deleted, _ := res.RowsAffected()

	// 3. Dynamic Filter Sweep: query remaining unowned movies and verify against active FilterConfig
	remRows, rErr := d.conn.Query(`
		SELECT id, title, original_title, label, series, cover_url, genres_json
		FROM movies
		WHERE id NOT IN (SELECT movie_id FROM user_state WHERE is_watched = 1 OR is_favorite = 1)
		  AND id NOT IN (SELECT movie_id FROM library_files WHERE file_path != '')
		  AND id NOT IN (SELECT movie_id FROM organized_movies WHERE target_folder != '' OR target_video != '');
	`)
	if rErr == nil {
		var dynamicDeleteIDs []string
		for remRows.Next() {
			var mid, mtitle, morig, mlabel, mseries, mcover, mgenresJSON string
			if scanErr := remRows.Scan(&mid, &mtitle, &morig, &mlabel, &mseries, &mcover, &mgenresJSON); scanErr == nil {
				var genres []string
				_ = json.Unmarshal([]byte(mgenresJSON), &genres)
				if shouldSkip, _ := scraper.IsPromotionalOrOmnibusVariantWithDetails(mid, mtitle, morig, mlabel, mseries, mcover, genres); shouldSkip {
					dynamicDeleteIDs = append(dynamicDeleteIDs, mid)
				}
			}
		}
		_ = remRows.Err()
		remRows.Close()

		for _, dynID := range dynamicDeleteIDs {
			if _, delErr := d.conn.Exec("DELETE FROM movies WHERE id = ?", dynID); delErr == nil {
				deleted++
			}
		}
	}

	// Clean orphaned movie_actresses
	_, _ = d.conn.Exec("DELETE FROM movie_actresses WHERE movie_id NOT IN (SELECT id FROM movies)")

	return deleted, nil
}

// LinkMovieActress creates or updates a relational link between a movie and an actress.
func (d *DB) LinkMovieActress(movieID, actressID, name, jaName, source string) error {
	if d == nil || d.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	movieID = strings.TrimSpace(movieID)
	actressID = strings.TrimSpace(actressID)
	if movieID == "" || actressID == "" {
		return fmt.Errorf("movie_id and actress_id required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO movie_actresses (movie_id, actress_id, actress_name, actress_ja_name, source)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(movie_id, actress_id) DO UPDATE SET
		actress_name = excluded.actress_name,
		actress_ja_name = CASE WHEN excluded.actress_ja_name != '' THEN excluded.actress_ja_name ELSE movie_actresses.actress_ja_name END,
		source = excluded.source;
	`
	_, err := d.conn.Exec(query, movieID, actressID, name, jaName, source)
	return err
}

// GetMovieActresses returns all performers linked to a movie.
func (d *DB) GetMovieActresses(movieID string) ([]MovieActressRecord, error) {
	if d == nil || d.conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT movie_id, actress_id, actress_name, actress_ja_name, source, created_at 
	          FROM movie_actresses WHERE movie_id = ? ORDER BY actress_name ASC`
	rows, err := d.conn.Query(query, movieID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MovieActressRecord
	for rows.Next() {
		var r MovieActressRecord
		if err := rows.Scan(&r.MovieID, &r.ActressID, &r.ActressName, &r.ActressJaName, &r.Source, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// GetActressMovieIDs returns all movie IDs associated with an actress_id or exact actress_name.
func (d *DB) GetActressMovieIDs(actressIDOrName string) ([]string, error) {
	if d == nil || d.conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT DISTINCT movie_id FROM movie_actresses 
	WHERE actress_id = ? 
	   OR actress_name = ? COLLATE NOCASE 
	   OR actress_ja_name = ?
	`
	rows, err := d.conn.Query(query, actressIDOrName, actressIDOrName, actressIDOrName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movieIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			movieIDs = append(movieIDs, id)
		}
	}
	return movieIDs, rows.Err()
}

func (d *DB) autoMigrateMovieActresses() error {
	var count int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM movie_actresses").Scan(&count)
	if count > 0 {
		return nil
	}
	// Run initial backfill asynchronously or synchronously
	go func() {
		_, _ = d.BackfillMovieActresses("")
	}()
	return nil
}

// BackfillMovieActresses populates movie_actresses from existing movies.actresses_json and organized_movies.
func (d *DB) BackfillMovieActresses(dumpDBPath string) (int, error) {
	if d == nil || d.conn == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	if dumpDBPath == "" {
		dumpDBPath = filepath.Join(filepath.Dir(d.path), "r18_dump.db")
	}

	// Cache actress name -> r18_id
	actressIDs := make(map[string]int)
	d.mu.RLock()
	rows, err := d.conn.Query("SELECT name, COALESCE(ja_name, ''), r18_id FROM actresses WHERE r18_id > 0")
	if err == nil {
		for rows.Next() {
			var n, jn string
			var id int
			if err := rows.Scan(&n, &jn, &id); err == nil && id > 0 {
				actressIDs[strings.ToLower(n)] = id
				if jn != "" {
					actressIDs[strings.ToLower(jn)] = id
				}
			}
		}
		_ = rows.Err()
		rows.Close()
	}
	d.mu.RUnlock()

	// If dump DB exists, also load additional actress IDs on demand
	var dumpDB *sql.DB
	if _, err := os.Stat(dumpDBPath); err == nil {
		dumpDB, _ = sql.Open("sqlite", dumpDBPath+"?mode=ro&_pragma=query_only(true)")
		if dumpDB != nil {
			defer dumpDB.Close()
		}
	}

	getActressDMMID := func(name, jaName string) int {
		if id, ok := actressIDs[strings.ToLower(name)]; ok && id > 0 {
			return id
		}
		if jaName != "" {
			if id, ok := actressIDs[strings.ToLower(jaName)]; ok && id > 0 {
				return id
			}
		}
		if dumpDB != nil {
			var dmmID int
			_ = dumpDB.QueryRow("SELECT id FROM actresses WHERE name_romaji = ? COLLATE NOCASE OR name_kanji = ? LIMIT 1", name, jaName).Scan(&dmmID)
			if dmmID > 0 {
				actressIDs[strings.ToLower(name)] = dmmID
				return dmmID
			}
		}
		return 0
	}

	d.mu.RLock()
	mRows, err := d.conn.Query("SELECT id, COALESCE(actresses_json, '[]') FROM movies")
	if err != nil {
		d.mu.RUnlock()
		return 0, err
	}
	type item struct {
		id   string
		json string
	}
	var items []item
	for mRows.Next() {
		var it item
		if err := mRows.Scan(&it.id, &it.json); err == nil {
			items = append(items, it)
		}
	}
	_ = mRows.Err()
	mRows.Close()
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO movie_actresses (movie_id, actress_id, actress_name, actress_ja_name, source)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(movie_id, actress_id) DO UPDATE SET
			actress_name = excluded.actress_name,
			actress_ja_name = CASE WHEN excluded.actress_ja_name != '' THEN excluded.actress_ja_name ELSE movie_actresses.actress_ja_name END,
			source = excluded.source;
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	type actressObj struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		JaName string `json:"ja_name"`
	}

	for _, it := range items {
		if it.json == "" || it.json == "[]" {
			continue
		}
		var acts []actressObj
		if err := json.Unmarshal([]byte(it.json), &acts); err != nil {
			continue
		}
		for _, act := range acts {
			name := strings.TrimSpace(act.Name)
			if name == "" {
				continue
			}
			dmmID := act.ID
			if dmmID == 0 {
				dmmID = getActressDMMID(name, act.JaName)
			}
			source := "dmm"
			if dmmID == 0 {
				source = "custom"
			}
			actressID := GenerateActressID(dmmID, name)
			if _, err := stmt.Exec(it.id, actressID, name, act.JaName, source); err == nil {
				count++
			}
		}
	}

	// Also link organized_movies folders that might not have actresses in JSON (e.g. FC2)
	orgRows, err := tx.Query(`
		SELECT om.movie_id, om.target_folder 
		FROM organized_movies om 
		WHERE om.target_folder != ''
	`)
	if err == nil {
		type orgItem struct {
			movieID string
			folder  string
		}
		var orgs []orgItem
		for orgRows.Next() {
			var o orgItem
			if err := orgRows.Scan(&o.movieID, &o.folder); err == nil {
				orgs = append(orgs, o)
			}
		}
		_ = orgRows.Err()
		orgRows.Close()

		for _, o := range orgs {
			cleanFolder := filepath.Clean(o.folder)
			parent := filepath.Dir(cleanFolder)
			actressDir := filepath.Base(parent)
			if actressDir == "" || actressDir == "." || actressDir == "/" || actressDir == "organized" {
				continue
			}
			var exists int
			_ = tx.QueryRow("SELECT COUNT(*) FROM movie_actresses WHERE movie_id = ?", o.movieID).Scan(&exists)
			if exists == 0 {
				dmmID := getActressDMMID(actressDir, "")
				source := "folder"
				if dmmID > 0 {
					source = "dmm"
				}
				actressID := GenerateActressID(dmmID, actressDir)
				if _, err := stmt.Exec(o.movieID, actressID, actressDir, "", source); err == nil {
					count++
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return count, nil
}

// DownloadQueueRecord represents an active or completed download item in SQLite.
type DownloadQueueRecord struct {
	ID             int       `json:"id"`
	MovieID        string    `json:"movie_id"`
	CombinedID     string    `json:"combined_id"`
	MovieTitle     string    `json:"movie_title"`
	CoverURL       string    `json:"cover_url"`
	ActressName    string    `json:"actress_name"`
	TorrentHash    string    `json:"torrent_hash"`
	TorrentTitle   string    `json:"torrent_title"`
	TorrentURL     string    `json:"torrent_url"`
	MagnetURL      string    `json:"magnet_url"`
	FileSizeBytes  int64     `json:"file_size_bytes"`
	QualityTag     string    `json:"quality_tag"`
	Status         string    `json:"status"` // queued, downloading, staging, organized, error
	ProgressPct    float64   `json:"progress_pct"`
	DownloadSpeed  int64     `json:"download_speed"`
	ETASeconds     int64     `json:"eta_seconds"`
	TransmissionID int       `json:"transmission_id"`
	DownloadPath   string    `json:"download_path"`
	ErrorMessage   string    `json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GetSetting retrieves a setting value by key or returns defaultVal if missing.
func (d *DB) GetSetting(key string, defaultVal string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var val string
	err := d.conn.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

// SetSetting stores or updates a setting key-value pair.
func (d *DB) SetSetting(key string, val string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		INSERT INTO app_settings (key, value, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, val)
	return err
}

// GetAllSettings returns all configured settings as a map.
func (d *DB) GetAllSettings() (map[string]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query("SELECT key, value FROM app_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			settings[k] = v
		}
	}
	return settings, nil
}

// AddToDownloadQueue inserts a new movie/torrent into the download queue.
func (d *DB) AddToDownloadQueue(item *DownloadQueueRecord) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.conn.Exec(`
		INSERT INTO download_queue (
			movie_id, combined_id, movie_title, cover_url, actress_name,
			torrent_hash, torrent_title, torrent_url, magnet_url,
			file_size_bytes, quality_tag, status, progress_pct,
			download_speed, eta_seconds, transmission_id, download_path,
			error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, item.MovieID, item.CombinedID, item.MovieTitle, item.CoverURL, item.ActressName,
		item.TorrentHash, item.TorrentTitle, item.TorrentURL, item.MagnetURL,
		item.FileSizeBytes, item.QualityTag, item.Status, item.ProgressPct,
		item.DownloadSpeed, item.ETASeconds, item.TransmissionID, item.DownloadPath,
		item.ErrorMessage,
	)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// GetDownloadQueue returns all items in the download queue ordered by updated_at DESC.
func (d *DB) GetDownloadQueue() ([]DownloadQueueRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`
		SELECT id, movie_id, combined_id, movie_title, cover_url, actress_name,
		       torrent_hash, torrent_title, torrent_url, magnet_url,
		       file_size_bytes, quality_tag, status, progress_pct,
		       download_speed, eta_seconds, transmission_id, download_path,
		       error_message, created_at, updated_at
		FROM download_queue
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DownloadQueueRecord
	for rows.Next() {
		var it DownloadQueueRecord
		var cAt, uAt string
		if err := rows.Scan(
			&it.ID, &it.MovieID, &it.CombinedID, &it.MovieTitle, &it.CoverURL, &it.ActressName,
			&it.TorrentHash, &it.TorrentTitle, &it.TorrentURL, &it.MagnetURL,
			&it.FileSizeBytes, &it.QualityTag, &it.Status, &it.ProgressPct,
			&it.DownloadSpeed, &it.ETASeconds, &it.TransmissionID, &it.DownloadPath,
			&it.ErrorMessage, &cAt, &uAt,
		); err != nil {
			continue
		}
		it.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", cAt)
		it.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", uAt)
		items = append(items, it)
	}
	return items, nil
}

// GetDownloadQueueByMovieID retrieves active or most recent queue record for a movie ID.
func (d *DB) GetDownloadQueueByMovieID(movieID string) (*DownloadQueueRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var it DownloadQueueRecord
	var cAt, uAt string
	err := d.conn.QueryRow(`
		SELECT id, movie_id, combined_id, movie_title, cover_url, actress_name,
		       torrent_hash, torrent_title, torrent_url, magnet_url,
		       file_size_bytes, quality_tag, status, progress_pct,
		       download_speed, eta_seconds, transmission_id, download_path,
		       error_message, created_at, updated_at
		FROM download_queue
		WHERE movie_id = ?
		ORDER BY id DESC LIMIT 1
	`, movieID).Scan(
		&it.ID, &it.MovieID, &it.CombinedID, &it.MovieTitle, &it.CoverURL, &it.ActressName,
		&it.TorrentHash, &it.TorrentTitle, &it.TorrentURL, &it.MagnetURL,
		&it.FileSizeBytes, &it.QualityTag, &it.Status, &it.ProgressPct,
		&it.DownloadSpeed, &it.ETASeconds, &it.TransmissionID, &it.DownloadPath,
		&it.ErrorMessage, &cAt, &uAt,
	)
	if err != nil {
		return nil, err
	}
	it.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", cAt)
	it.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", uAt)
	return &it, nil
}

// UpdateDownloadQueueStatus updates status and progress for a download queue item.
func (d *DB) UpdateDownloadQueueStatus(id int, status string, progressPct float64, speed int64, eta int64, errorMsg string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		UPDATE download_queue
		SET status = ?, progress_pct = ?, download_speed = ?, eta_seconds = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, progressPct, speed, eta, errorMsg, id)
	return err
}

// UpdateDownloadQueueTransmission updates transmission ID and hash for a queue item.
func (d *DB) UpdateDownloadQueueTransmission(id int, transmissionID int, hash string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		UPDATE download_queue
		SET transmission_id = ?, torrent_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, transmissionID, hash, id)
	return err
}

// RemoveFromDownloadQueue deletes an entry from the download queue.
func (d *DB) RemoveFromDownloadQueue(id int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM download_queue WHERE id = ?", id)
	return err
}


