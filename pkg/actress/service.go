package actress

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

// GenreCount represents genre tag with its frequency count.
type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// ReleaseItem holds filmography release details with download, watch, and rating status.
type ReleaseItem struct {
	MovieID         string   `json:"movie_id"`
	Title           string   `json:"title"`
	OriginalTitle   string   `json:"original_title"`
	Maker           string   `json:"maker"`
	ReleaseDate     string   `json:"release_date"`
	CoverURL        string   `json:"cover_url"`
	IsDownloaded    bool     `json:"is_downloaded"`
	IsWatched       bool     `json:"is_watched"`
	UserRating      int      `json:"user_rating"`
	IsFavorite      bool     `json:"is_favorite"`
	LibraryPath     string   `json:"library_path,omitempty"`
	OrganizedFolder string   `json:"organized_folder,omitempty"`
	OrganizedVideo  string   `json:"organized_video,omitempty"`
	SizeBytes       int64    `json:"size_bytes,omitempty"`
	Genres          []string `json:"genres,omitempty"`
}

// ActressSummary aggregates release statistics for a followed actress.
type ActressSummary struct {
	Actress        db.ActressRecord `json:"actress"`
	Releases       []ReleaseItem    `json:"releases"`
	Total          int              `json:"total"`
	Downloaded     int              `json:"downloaded"`
	Missing        int              `json:"missing"`
	Watched        int              `json:"watched"`
	Favorites      int              `json:"favorites"`
	TotalSizeBytes int64            `json:"total_size_bytes"`
	TopGenres      []GenreCount     `json:"top_genres"`
	DebutDate      string           `json:"debut_date,omitempty"`
	LatestDate     string           `json:"latest_date,omitempty"`
}

// Service manages actress tracking and new release detection.
type Service struct {
	database *db.DB
	scraper  *scraper.Client
}

// New creates a new Actress Service.
func New(d *db.DB, client *scraper.Client) *Service {
	if d == nil {
		d, _ = db.Default()
	}
	if client == nil {
		client = scraper.NewClient(15 * time.Second)
	}
	return &Service{
		database: d,
		scraper:  client,
	}
}

// Follow tracks an actress by name.
func (s *Service) Follow(name, jaName, imageURL string) error {
	if s.database == nil {
		return fmt.Errorf("database not initialized")
	}
	return s.database.FollowActress(name, jaName, imageURL)
}

// Unfollow stops tracking an actress.
func (s *Service) Unfollow(name string) error {
	if s.database == nil {
		return fmt.Errorf("database not initialized")
	}
	return s.database.UnfollowActress(name)
}

// IsFollowed checks if an actress is currently followed.
func (s *Service) IsFollowed(name string) (bool, error) {
	if s.database == nil {
		return false, nil
	}
	return s.database.IsActressFollowed(name)
}

// ListFollowed returns all tracked actresses.
func (s *Service) ListFollowed() ([]db.ActressRecord, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return s.database.ListFollowedActresses()
}

// GetActressSummary retrieves all known releases for an actress, cross-referencing download and watch status.
func (s *Service) GetActressSummary(ctx context.Context, actressName string) (*ActressSummary, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	actressName = strings.TrimSpace(actressName)
	if actressName == "" {
		return nil, fmt.Errorf("actress name cannot be empty")
	}

	// Update last checked timestamp
	_ = s.database.UpdateActressLastChecked(actressName)

	// Load actress info from DB if available
	actRec := db.ActressRecord{
		Name: actressName,
	}
	if followed, err := s.database.ListFollowedActresses(); err == nil {
		for _, a := range followed {
			if strings.EqualFold(a.Name, actressName) {
				actRec = a
				break
			}
		}
	}

	// Query movies containing the actress name in actresses_json
	query := `
	SELECT m.id, COALESCE(m.title, m.id), COALESCE(m.original_title, ''), COALESCE(m.maker, ''), COALESCE(m.release_date, ''), COALESCE(m.cover_url, ''), COALESCE(m.actresses_json, '[]'),
	       COALESCE(u.is_watched, 0), COALESCE(u.user_rating, 0), COALESCE(u.is_favorite, 0),
	       MAX(lf.file_path),
	       MAX(om.target_folder), MAX(om.target_video),
	       COALESCE(SUM(lf.size_bytes), 0),
	       COALESCE(m.genres_json, '[]')
	FROM movies m
	LEFT JOIN user_state u ON m.id = u.movie_id
	LEFT JOIN library_files lf ON m.id = lf.movie_id
	LEFT JOIN organized_movies om ON m.id = om.movie_id
	WHERE m.actresses_json LIKE ? COLLATE NOCASE
	GROUP BY m.id
	ORDER BY m.release_date DESC
	`

	rows, err := s.database.Query(query, "%"+actressName+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []ReleaseItem
	downloadedCount := 0
	watchedCount := 0
	favCount := 0
	var totalSizeBytes int64
	genreFreq := make(map[string]int)
	debutDate := ""
	latestDate := ""

	for rows.Next() {
		var r ReleaseItem
		var actJSON string
		var libPath sql.NullString
		var orgFolder sql.NullString
		var orgVideo sql.NullString
		var sizeBytes int64
		var genresJSON string
		if err := rows.Scan(
			&r.MovieID, &r.Title, &r.OriginalTitle, &r.Maker, &r.ReleaseDate, &r.CoverURL, &actJSON,
			&r.IsWatched, &r.UserRating, &r.IsFavorite,
			&libPath,
			&orgFolder, &orgVideo,
			&sizeBytes,
			&genresJSON,
		); err != nil {
			return nil, err
		}

		if orgFolder.Valid && orgFolder.String != "" {
			r.OrganizedFolder = orgFolder.String
		}
		if orgVideo.Valid && orgVideo.String != "" {
			r.OrganizedVideo = orgVideo.String
		}
		if libPath.Valid && libPath.String != "" {
			r.LibraryPath = libPath.String
		}

		// Check if organized folder exists in default organized library (/Volumes/home/BT/organized) if not in DB
		if r.OrganizedFolder == "" && r.MovieID != "" {
			candidates := []string{
				filepath.Join("/Volumes/home/BT/organized", actressName),
				filepath.Join("/Volumes/home/BT/organized", actRec.Name),
			}
			for _, actDir := range candidates {
				if entries, err := os.ReadDir(actDir); err == nil {
					for _, entry := range entries {
						if entry.IsDir() && strings.Contains(strings.ToUpper(entry.Name()), strings.ToUpper(r.MovieID)) {
							foundPath := filepath.Join(actDir, entry.Name())
							r.OrganizedFolder = foundPath
							// Cache to database organized_movies
							_ = s.database.SetOrganized(r.MovieID, foundPath, "")
							break
						}
					}
				}
				if r.OrganizedFolder != "" {
					break
				}
			}
		}

		// Calculate size if not recorded in library_files but folder exists
		if sizeBytes == 0 && r.OrganizedFolder != "" {
			if entries, err := os.ReadDir(r.OrganizedFolder); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						if fi, fErr := e.Info(); fErr == nil {
							sizeBytes += fi.Size()
						}
					}
				}
			}
		}
		r.SizeBytes = sizeBytes

		// Parse genres
		if genresJSON != "" && genresJSON != "[]" {
			var parsedGenres []string
			if err := json.Unmarshal([]byte(genresJSON), &parsedGenres); err == nil {
				r.Genres = parsedGenres
			}
		}

		// A movie is downloaded/present if it has an organized folder, organized video, or library file
		if r.OrganizedFolder != "" || r.OrganizedVideo != "" || r.LibraryPath != "" {
			r.IsDownloaded = true
		}

		// Filter out promotional/set duplicates (e.g. C9FWAY095, E9FWAY095, S9FWAY095, Special Offers tag) and photobooks
		if !r.IsDownloaded && !r.IsWatched && !r.IsFavorite {
			if IsPromotionalOrDuplicateVariant(r.MovieID, r.Title, r.CoverURL, r.Genres) {
				continue
			}
		}

		// Tally genre frequencies for genuine releases only
		for _, g := range r.Genres {
			g = strings.TrimSpace(g)
			if g != "" && !isPromotionalGenre(g) {
				genreFreq[g]++
			}
		}

		if r.IsDownloaded {
			downloadedCount++
			totalSizeBytes += r.SizeBytes
		}
		if r.IsWatched {
			watchedCount++
		}
		if r.IsFavorite {
			favCount++
		}
		if r.CoverURL == "" && r.MovieID != "" {
			r.CoverURL = "/api/images/" + r.MovieID
		}

		if r.ReleaseDate != "" {
			if latestDate == "" || r.ReleaseDate > latestDate {
				latestDate = r.ReleaseDate
			}
			if debutDate == "" || r.ReleaseDate < debutDate {
				debutDate = r.ReleaseDate
			}
		}

		releases = append(releases, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	topGenres := make([]GenreCount, 0)
	for g, count := range genreFreq {
		topGenres = append(topGenres, GenreCount{Genre: g, Count: count})
	}
	sort.Slice(topGenres, func(i, j int) bool {
		if topGenres[i].Count == topGenres[j].Count {
			return topGenres[i].Genre < topGenres[j].Genre
		}
		return topGenres[i].Count > topGenres[j].Count
	})
	if len(topGenres) > 8 {
		topGenres = topGenres[:8]
	}

	summary := &ActressSummary{
		Actress:        actRec,
		Releases:       releases,
		Total:          len(releases),
		Downloaded:     downloadedCount,
		Missing:        len(releases) - downloadedCount,
		Watched:        watchedCount,
		Favorites:      favCount,
		TotalSizeBytes: totalSizeBytes,
		TopGenres:      topGenres,
		DebutDate:      debutDate,
		LatestDate:     latestDate,
	}

	return summary, nil
}

// CheckAllFollowed checks new releases for all followed actresses.
func (s *Service) CheckAllFollowed(ctx context.Context) ([]ActressSummary, error) {
	actresses, err := s.ListFollowed()
	if err != nil {
		return nil, err
	}

	var results []ActressSummary
	for _, a := range actresses {
		summary, err := s.GetActressSummary(ctx, a.Name)
		if err == nil && summary != nil {
			summary.Actress = a
			results = append(results, *summary)
		}
	}
	return results, nil
}

var promoSkuRegex = regexp.MustCompile(`^(?:[A-Z]9[A-Z]{2,6}[-_]?\d+|9[A-Z]{3,6}\d+)`)

// IsPromotionalOrDuplicateVariant checks if a release is a duplicate promotional bundle,
// online event ticket, set product SKU (e.g. C9FWAY095, E9FWAY095, S9FWAY095, L9MIDA438, Special Offers tag),
// or non-video digital photobook / magazine.
func IsPromotionalOrDuplicateVariant(movieID, title, coverURL string, genres []string) bool {
	upperID := strings.ToUpper(strings.TrimSpace(movieID))
	tl := strings.ToLower(title)

	// 1. Non-video digital e-book / photobook checks
	if strings.Contains(coverURL, "ebook-assets") || strings.Contains(coverURL, "/e-book/") {
		return true
	}
	for _, kw := range []string{"写真集", "デジタル写真集", "ポーズブック", "フォトブック", "電子書籍", "photobook", "photo book"} {
		if strings.Contains(tl, kw) {
			return true
		}
	}

	// 2. Tag / Genre checks (R18 / DMM categories)
	for _, g := range genres {
		gNorm := strings.ToLower(strings.TrimSpace(g))
		if gNorm == "special offers and set products" ||
			gNorm == "includes event participation rights" ||
			strings.Contains(gNorm, "set products") ||
			strings.Contains(gNorm, "event participation") ||
			strings.Contains(gNorm, "photo book") ||
			strings.Contains(gNorm, "digital photo") {
			return true
		}
	}

	// 3. Promotional SKU prefixes: C9, E9, S9, N9, L9, K9, KA9, KC9, TK9, 9 followed by letters
	if promoSkuRegex.MatchString(upperID) {
		return true
	}

	// 4. Title markers (Online autograph sessions, multiple purchase bundle promotions, goods sets)
	if strings.Contains(title, "オンラインサイン会") ||
		strings.Contains(title, "購入特典付き") ||
		strings.Contains(title, "購入特典付") ||
		strings.Contains(title, "参加URL付き") ||
		strings.Contains(title, "参加URL付") ||
		strings.Contains(title, "参加権付き") ||
		strings.Contains(title, "キーホルダーセット") ||
		strings.Contains(title, "チェキセット") {
		return true
	}

	return false
}

func isPromotionalGenre(genre string) bool {
	gNorm := strings.ToLower(strings.TrimSpace(genre))
	return gNorm == "special offers and set products" ||
		gNorm == "includes event participation rights" ||
		strings.Contains(gNorm, "set products") ||
		strings.Contains(gNorm, "event participation") ||
		strings.Contains(gNorm, "photo book") ||
		strings.Contains(gNorm, "digital photo")
}


