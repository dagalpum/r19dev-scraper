package scraper

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var cleanNonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// DumpStore provides instant, sub-millisecond offline lookups from the local R18.dev database dump.
type DumpStore struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

var (
	defaultDumpStore *DumpStore
	dumpOnce         sync.Once
)

// DefaultDumpStore returns a singleton DumpStore if r18_dump.db is found in the application data directory.
func DefaultDumpStore() *DumpStore {
	dumpOnce.Do(func() {
		candidates := []string{}

		// 1. App Support / Config Dir
		if baseDir, err := os.UserConfigDir(); err == nil && baseDir != "" {
			candidates = append(candidates, filepath.Join(baseDir, "r19dev", "r18_dump.db"))
		}
		// 2. User Home Dir fallback
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			candidates = append(candidates,
				filepath.Join(home, "Library", "Application Support", "r19dev", "r18_dump.db"),
				filepath.Join(home, ".config", "r19dev", "r18_dump.db"),
			)
		}
		// 3. Current Working Directory fallback
		candidates = append(candidates, "./r18_dump.db")

		for _, p := range candidates {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
				ds, err := OpenDumpStore(p)
				if err == nil {
					defaultDumpStore = ds
					return
				}
			}
		}
	})
	return defaultDumpStore
}

// OpenDumpStore opens a local r18_dump.db SQLite database in read-only mode.
func OpenDumpStore(dbPath string) (*DumpStore, error) {
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(3000)&_pragma=query_only(true)", absPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open r18_dump.db: %w", err)
	}

	// Verify database connection and tables
	var count int
	if err := conn.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='r18_movies'").Scan(&count); err != nil || count == 0 {
		conn.Close()
		return nil, fmt.Errorf("invalid r18_dump.db: missing r18_movies table")
	}

	return &DumpStore{
		db:   conn,
		path: absPath,
	}, nil
}

// Close closes the underlying SQLite connection.
func (ds *DumpStore) Close() error {
	if ds == nil || ds.db == nil {
		return nil
	}
	return ds.db.Close()
}

// Path returns the filesystem path of the dump database.
func (ds *DumpStore) Path() string {
	if ds == nil {
		return ""
	}
	return ds.path
}

// GetMovie performs an offline lookup by standard ID, DVD ID, or content ID.
func (ds *DumpStore) GetMovie(id string, language string) (*Movie, bool) {
	if ds == nil || ds.db == nil {
		return nil, false
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil, false
	}

	combinedID := NormalizeToCombinedID(trimmed)
	cleanID := strings.ToUpper(cleanNonAlphanumeric.ReplaceAllString(trimmed, ""))
	dvdID := strings.ToUpper(trimmed)

	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var (
		contentID     string
		foundDvdID    string
		titleEN       string
		titleJA       string
		makerNameEN   string
		releaseDate   string
		jacketFullURL string
	)

	// Query r18_movies
	query := `
		SELECT content_id, COALESCE(dvd_id, ''), COALESCE(title_en, ''), COALESCE(title_ja, ''),
		       COALESCE(maker_name_en, ''), COALESCE(release_date, ''), COALESCE(jacket_full_url, '')
		FROM r18_movies
		WHERE content_id = ? OR dvd_id = ? OR clean_id = ?
		LIMIT 1;
	`
	err := ds.db.QueryRow(query, combinedID, dvdID, cleanID).Scan(
		&contentID, &foundDvdID, &titleEN, &titleJA, &makerNameEN, &releaseDate, &jacketFullURL,
	)
	if err != nil {
		return nil, false
	}

	// Resolve Movie ID
	movieID := foundDvdID
	if movieID == "" {
		movieID = dvdID
	}

	// Resolve Title
	title := titleEN
	if language == "ja" {
		if titleJA != "" {
			title = titleJA
		}
	} else {
		// If English requested but titleEN is empty or identical to Japanese, check machine_translations table
		if title == "" || title == titleJA {
			var translated string
			if transErr := ds.db.QueryRow("SELECT target_en FROM translations WHERE source_ja = ? LIMIT 1", titleJA).Scan(&translated); transErr == nil && translated != "" {
				title = translated
			}
		}
		if title == "" {
			title = titleJA
		}
	}

	// Resolve Cover URLs
	coverURL := jacketFullURL
	if coverURL != "" && !strings.HasPrefix(coverURL, "http") {
		coverURL = "https://pics.dmm.co.jp/" + strings.TrimPrefix(coverURL, "/")
		if !strings.HasSuffix(coverURL, ".jpg") && !strings.HasSuffix(coverURL, ".png") {
			coverURL += ".jpg"
		}
	}

	movie := &Movie{
		ID:             movieID,
		CombinedID:     contentID,
		Title:          title,
		OriginalTitle:  titleJA,
		Maker:          makerNameEN,
		ReleaseDate:    releaseDate,
		CoverURL:       coverURL,
		PosterURL:      coverURL,
		DetailURL:      fmt.Sprintf("https://r18.dev/videos/vod/movies/detail/-/combined=%s/", contentID),
		ScrapedAt:      time.Now(),
	}

	// Query Actresses for this content_id
	actressQuery := `
		SELECT a.id, COALESCE(a.name_romaji, ''), COALESCE(a.name_kanji, ''), COALESCE(a.image_url, '')
		FROM video_actresses va
		JOIN actresses a ON va.actress_id = a.id
		WHERE va.content_id = ?;
	`
	rows, err := ds.db.Query(actressQuery, contentID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				actID      int
				nameRomaji string
				nameKanji  string
				thumb      string
			)
			if err := rows.Scan(&actID, &nameRomaji, &nameKanji, &thumb); err == nil {
				name := nameRomaji
				if language == "ja" && nameKanji != "" {
					name = nameKanji
				}
				if name == "" {
					name = nameKanji
				}
				if thumb != "" && !strings.HasPrefix(thumb, "http") {
					thumb = "https://pics.dmm.co.jp/mono/actjpgs/" + thumb
				}

				movie.Actresses = append(movie.Actresses, Actress{
					ID:       actID,
					Name:     name,
					JaName:   nameKanji,
					ImageURL: thumb,
				})
			}
		}
	}

	return movie, true
}
