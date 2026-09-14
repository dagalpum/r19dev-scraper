package actress

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/jellyfin"
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
	CombinedID      string   `json:"combined_id,omitempty"`
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
	SkipReason      string   `json:"skip_reason,omitempty"`
}

// DiscoveredActress represents an unfollowed actress who has files in the user's library.
type DiscoveredActress struct {
	Name          string `json:"name"`
	JaName        string `json:"ja_name"`
	ImageURL      string `json:"image_url"`
	MovieCount    int    `json:"movie_count"`
	LatestRelease string `json:"latest_release"`
}

// ActressSummary aggregates release statistics for a followed actress.
type ActressSummary struct {
	Actress            db.ActressRecord `json:"actress"`
	Releases           []ReleaseItem    `json:"releases"`
	SkippedReleases    []ReleaseItem    `json:"skipped_releases,omitempty"`
	SkippedCount       int              `json:"skipped_count"`
	Total              int              `json:"total"`
	Downloaded         int              `json:"downloaded"`
	Missing            int              `json:"missing"`
	Watched            int              `json:"watched"`
	Favorites          int              `json:"favorites"`
	TotalSizeBytes     int64            `json:"total_size_bytes"`
	TopGenres          []GenreCount     `json:"top_genres"`
	DebutDate          string           `json:"debut_date,omitempty"`
	LatestDate         string           `json:"latest_date,omitempty"`
	LatestMovieID      string           `json:"latest_movie_id,omitempty"`
	LatestIsDownloaded bool             `json:"latest_is_downloaded"`
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

// ListDiscoveredActresses finds all actresses who appear in movies in the library/NAS but are not yet followed.
func (s *Service) ListDiscoveredActresses() ([]DiscoveredActress, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
	SELECT 
		json_extract(a.value, '$.name') AS actress_name,
		COALESCE(json_extract(a.value, '$.ja_name'), '') AS ja_name,
		COALESCE(json_extract(a.value, '$.image_url'), '') AS image_url,
		COUNT(DISTINCT m.id) AS movie_count,
		COALESCE(MAX(m.release_date), '') AS latest_release
	FROM movies m
	JOIN json_each(m.actresses_json) a
	LEFT JOIN actresses act ON (
		LOWER(act.name) = LOWER(json_extract(a.value, '$.name'))
		OR (act.ja_name != '' AND LOWER(act.ja_name) = LOWER(json_extract(a.value, '$.name')))
		OR (act.ja_name != '' AND LOWER(act.ja_name) = LOWER(json_extract(a.value, '$.ja_name')))
	)
	WHERE act.name IS NULL
	  AND json_extract(a.value, '$.name') IS NOT NULL
	  AND TRIM(json_extract(a.value, '$.name')) != ''
	  AND (
		m.id IN (SELECT DISTINCT movie_id FROM organized_movies WHERE target_folder != '' OR target_video != '')
		OR m.id IN (SELECT DISTINCT movie_id FROM library_files WHERE file_path != '')
	  )
	GROUP BY LOWER(json_extract(a.value, '$.name'))
	ORDER BY movie_count DESC, actress_name ASC
	`

	rows, err := s.database.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DiscoveredActress
	for rows.Next() {
		var d DiscoveredActress
		if err := rows.Scan(&d.Name, &d.JaName, &d.ImageURL, &d.MovieCount, &d.LatestRelease); err != nil {
			return nil, err
		}
		d.ImageURL = "/api/actresses/avatar/" + url.PathEscape(d.Name)
		results = append(results, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetDiscoveredActressMovies retrieves all local NAS movies featuring an unfollowed/discovered actress.
func (s *Service) GetDiscoveredActressMovies(ctx context.Context, actressName string) ([]ReleaseItem, error) {
	if s.database == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	actressName = strings.TrimSpace(actressName)
	if actressName == "" {
		return nil, fmt.Errorf("actress name cannot be empty")
	}

	query := `
	SELECT 
		m.id, 
		COALESCE(m.combined_id, ''),
		COALESCE(m.title, m.id), 
		COALESCE(m.original_title, ''), 
		COALESCE(m.maker, ''), 
		COALESCE(m.release_date, ''), 
		COALESCE(m.cover_url, ''), 
		COALESCE(MAX(om.target_folder), ''), 
		COALESCE(MAX(om.target_video), ''), 
		COALESCE(MAX(lf.file_path), ''), 
		COALESCE(SUM(lf.size_bytes), 0), 
		COALESCE(m.genres_json, '[]'),
		COALESCE(u.is_watched, 0),
		COALESCE(u.user_rating, 0),
		COALESCE(u.is_favorite, 0)
	FROM movies m
	JOIN json_each(m.actresses_json) a
	LEFT JOIN organized_movies om ON om.movie_id = m.id
	LEFT JOIN library_files lf ON lf.movie_id = m.id
	LEFT JOIN user_state u ON u.movie_id = m.id
	WHERE (
		LOWER(json_extract(a.value, '$.name')) = LOWER(?)
		OR LOWER(json_extract(a.value, '$.ja_name')) = LOWER(?)
		OR LOWER(json_extract(a.value, '$.name')) LIKE '%' || LOWER(?) || '%'
		OR om.target_folder LIKE '%' || ? || '%'
	)
	GROUP BY m.id
	ORDER BY m.release_date DESC;
	`

	rows, err := s.database.Query(query, actressName, actressName, actressName, actressName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawReleases []ReleaseItem
	for rows.Next() {
		var r ReleaseItem
		var orgFolder, orgVideo, libPath, genresJSON string
		var isWatched, isFavorite int
		if err := rows.Scan(
			&r.MovieID, &r.CombinedID, &r.Title, &r.OriginalTitle, &r.Maker, &r.ReleaseDate, &r.CoverURL,
			&orgFolder, &orgVideo, &libPath,
			&r.SizeBytes,
			&genresJSON,
			&isWatched, &r.UserRating, &isFavorite,
		); err != nil {
			return nil, err
		}

		r.IsWatched = isWatched == 1
		r.IsFavorite = isFavorite == 1
		r.OrganizedFolder = orgFolder
		r.OrganizedVideo = orgVideo
		r.LibraryPath = libPath

		// If library_path is present but organized_folder is not, derive folder
		if r.OrganizedFolder == "" && r.LibraryPath != "" {
			r.OrganizedFolder = filepath.Dir(r.LibraryPath)
		}

		// Check if organized folder exists in default organized library if not recorded in DB
		if r.OrganizedFolder == "" && r.MovieID != "" {
			candidates := []string{
				filepath.Join("/Volumes/home/BT/organized", actressName),
			}
			for _, actDir := range candidates {
				if entries, err := os.ReadDir(actDir); err == nil {
					for _, entry := range entries {
						if entry.IsDir() && strings.Contains(strings.ToUpper(entry.Name()), strings.ToUpper(r.MovieID)) {
							foundPath := filepath.Join(actDir, entry.Name())
							r.OrganizedFolder = foundPath
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

		// Only include releases that are downloaded / locally present in NAS
		if r.OrganizedFolder != "" || r.OrganizedVideo != "" || r.LibraryPath != "" {
			r.IsDownloaded = true
		} else {
			// Not downloaded in NAS, skip for discovered local titles view
			continue
		}

		// Calculate size if not recorded but folder/file exists
		if r.SizeBytes == 0 {
			if r.LibraryPath != "" {
				if fi, fErr := os.Stat(r.LibraryPath); fErr == nil {
					r.SizeBytes = fi.Size()
				}
			}
			if r.SizeBytes == 0 && r.OrganizedFolder != "" {
				if entries, err := os.ReadDir(r.OrganizedFolder); err == nil {
					for _, e := range entries {
						if !e.IsDir() {
							if fi, fErr := e.Info(); fErr == nil {
								r.SizeBytes += fi.Size()
							}
						}
					}
				}
			}
		}

		// Parse genres
		if genresJSON != "" && genresJSON != "[]" {
			var parsedGenres []string
			if err := json.Unmarshal([]byte(genresJSON), &parsedGenres); err == nil {
				r.Genres = parsedGenres
			}
		}

		if r.CoverURL != "" {
			r.CoverURL = jellyfin.UpgradeDMMImageURL(r.CoverURL)
		}
		if (r.CoverURL == "" || !strings.HasPrefix(r.CoverURL, "http")) && r.MovieID != "" {
			r.CoverURL = "/api/images/" + r.MovieID
		}

		r.Title = CleanMovieTitle(r.Title)
		rawReleases = append(rawReleases, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	releases := deduplicateReleases(rawReleases)
	return releases, nil
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

	whereClause := "(m.actresses_json LIKE ? COLLATE NOCASE OR om.target_folder LIKE ? COLLATE NOCASE)"
	args := []any{"%" + actressName + "%", "%/" + actressName + "/%"}
	if actRec.JaName != "" {
		whereClause += " OR (m.actresses_json LIKE ? COLLATE NOCASE OR om.target_folder LIKE ? COLLATE NOCASE)"
		args = append(args, "%"+actRec.JaName+"%", "%/"+actRec.JaName+"/%")
	}

	query := fmt.Sprintf(`
	SELECT m.id, COALESCE(m.combined_id, ''), COALESCE(m.title, m.id), COALESCE(m.original_title, ''), COALESCE(m.maker, ''), COALESCE(m.release_date, ''), COALESCE(m.cover_url, ''), COALESCE(m.actresses_json, '[]'),
	       COALESCE(u.is_watched, 0), COALESCE(u.user_rating, 0), COALESCE(u.is_favorite, 0),
	       MAX(lf.file_path),
	       MAX(om.target_folder), MAX(om.target_video),
	       COALESCE(SUM(lf.size_bytes), 0),
	       COALESCE(m.genres_json, '[]')
	FROM movies m
	LEFT JOIN user_state u ON m.id = u.movie_id
	LEFT JOIN library_files lf ON m.id = lf.movie_id
	LEFT JOIN organized_movies om ON m.id = om.movie_id
	WHERE %s
	GROUP BY m.id
	ORDER BY m.release_date DESC
	`, whereClause)

	rows, err := s.database.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawReleases []ReleaseItem
	var skippedReleases []ReleaseItem
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
			&r.MovieID, &r.CombinedID, &r.Title, &r.OriginalTitle, &r.Maker, &r.ReleaseDate, &r.CoverURL, &actJSON,
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

		var parsedActs []any
		if actJSON != "" && actJSON != "[]" {
			_ = json.Unmarshal([]byte(actJSON), &parsedActs)
		}
		actCount := len(parsedActs)

		if r.CoverURL != "" {
			r.CoverURL = jellyfin.UpgradeDMMImageURL(r.CoverURL)
		}
		if (r.CoverURL == "" || !strings.HasPrefix(r.CoverURL, "http")) && r.MovieID != "" {
			r.CoverURL = "/api/images/" + r.MovieID
		}

		// Clean movie title
		r.Title = CleanMovieTitle(r.Title)

		// Filter out non-video books, variety shows, director cut re-issues, and multi-actress omnibus
		if !r.IsDownloaded && !r.IsWatched && !r.IsFavorite {
			if shouldSkip, skipReason := CheckFilmographyInclusion(r.MovieID, r.Title, r.OriginalTitle, r.CoverURL, r.Genres, actCount); shouldSkip {
				r.SkipReason = skipReason
				skippedReleases = append(skippedReleases, r)
				continue
			}
		}

		rawReleases = append(rawReleases, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Group 2: Deduplicate multi-format SKU duplicates (e.g. PPPD-485 vs PPP-485, EBDB-998 vs EBD-1013)
	releases := deduplicateReleases(rawReleases)

	// Tally statistics and genres from deduplicated releases
	latestMovieID := ""
	latestIsDownloaded := false

	for _, r := range releases {
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

		if r.ReleaseDate != "" {
			if latestDate == "" || r.ReleaseDate > latestDate {
				latestDate = r.ReleaseDate
				latestMovieID = r.MovieID
				latestIsDownloaded = r.IsDownloaded
			}
			if debutDate == "" || r.ReleaseDate < debutDate {
				debutDate = r.ReleaseDate
			}
		}
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
		Actress:            actRec,
		Releases:           releases,
		SkippedReleases:    skippedReleases,
		SkippedCount:       len(skippedReleases),
		Total:              len(releases),
		Downloaded:         downloadedCount,
		Missing:            len(releases) - downloadedCount,
		Watched:            watchedCount,
		Favorites:          favCount,
		TotalSizeBytes:     totalSizeBytes,
		TopGenres:          topGenres,
		DebutDate:          debutDate,
		LatestDate:         latestDate,
		LatestMovieID:      latestMovieID,
		LatestIsDownloaded: latestIsDownloaded,
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

var (
	promoSkuRegex         = regexp.MustCompile(`^(?:TK[A-Z]{3,6}[-_]?\d+|[A-Z]9[A-Z]{2,6}[-_]?\d+|9[A-Z]{3,6}\d+|(?:77|88)[A-Z]{3,6}[-_]?\d+)`)
	compilationRegex      = regexp.MustCompile(`(?i)\d+連発|\d+連射|\d+時間(?:BOX|ベスト)?|ベストセレクション|BESTセレクション|総集編|オムニバス|傑作選`)
	multiBodyRegex        = regexp.MustCompile(`\d+体(?:\d+分)?`)
	titleDedupeCleanRegex = regexp.MustCompile(`(?i)【.*?】|（.*?）|\(.*?\)|\[.*?\]|ブルーレイエディション|ディレクターズカット版?|未公開映像収録(?:のプレミアムエディション)?|2枚組|[_\s\-]`)
	marketingPrefixRegex  = regexp.MustCompile(`^【(?:数量限定|FANZA限定|DMM限定|期間限定|初回限定|先行配信|特装版|限定)】\s*`)
	formatTagRegex        = regexp.MustCompile(`(?i)[（(【\[](?:ブルーレイディスク|Blu-ray\s*Disc)[）)】\]]`)
	promoGoodsSuffixRegex = regexp.MustCompile(`(?i)\s*(?:生写真\d*枚セット|生写真\d*枚付き?|生写真セット|生写真付き?|生写真|チェキ\d*枚セット|チェキ\d*枚付き?|チェキセット|チェキ付き?|チェキ|購入特典付き?|キーホルダーセット|アクリルスタンド付き?)\s*$`)
)

// CleanMovieTitle cleans up marketing prefixes, bluray disk tags, and bonus goods suffixes from movie titles.
func CleanMovieTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	t = marketingPrefixRegex.ReplaceAllString(t, "")
	t = formatTagRegex.ReplaceAllString(t, "")
	t = promoGoodsSuffixRegex.ReplaceAllString(t, "")
	t = formatTagRegex.ReplaceAllString(t, "")
	t = promoGoodsSuffixRegex.ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}

func normalizeTitleForDedupe(title string) string {
	t := titleDedupeCleanRegex.ReplaceAllString(title, "")
	return strings.ToLower(strings.TrimSpace(t))
}

func canonicalBaseID(id string) string {
	upper := strings.ToUpper(strings.TrimSpace(id))
	upper = strings.TrimPrefix(upper, "TK")
	upper = strings.TrimSuffix(upper, "-EC")
	upper = strings.TrimSuffix(upper, "-T-EC")
	upper = strings.TrimSuffix(upper, "EC")
	upper = strings.ReplaceAll(upper, "BD-", "-")
	return upper
}

// scoreReleaseCanon returns a higher score for standard canonical release IDs (e.g. PPPD-485 > PPP-485, BOMN-169 > BOM-169)
func scoreReleaseCanon(r ReleaseItem) int {
	score := 0
	upperID := strings.ToUpper(r.MovieID)

	// Demote promotional/bonus SKUs (TK..., EC suffixes)
	if strings.HasPrefix(upperID, "TK") || strings.HasSuffix(upperID, "-EC") || strings.HasSuffix(upperID, "-T-EC") || strings.HasSuffix(upperID, "EC") {
		score -= 100
	}

	parts := strings.Split(r.MovieID, "-")
	prefix := parts[0]
	// Prefer standard 4-letter prefixes (PPPD, BOMN, MMND, MIZD) over 3-letter outlet codes (PPP, BOM, MMN, MIZ)
	score += len(prefix) * 10
	if strings.HasSuffix(prefix, "D") || strings.HasSuffix(prefix, "N") || strings.HasSuffix(prefix, "B") {
		score += 5
	}
	return score
}

// deduplicateReleases keeps downloaded copies first, and collapses unowned multi-format duplicate SKUs
// preferring canonical primary IDs (e.g. CJOD-510 over TKCJOD-510, PPPD-485 over PPP-485).
func deduplicateReleases(items []ReleaseItem) []ReleaseItem {
	var result []ReleaseItem
	titleGroups := make(map[string][]ReleaseItem)
	var orderedKeys []string

	for _, item := range items {
		// Downloaded, watched, or favorited items are ALWAYS preserved directly
		if item.IsDownloaded || item.IsWatched || item.IsFavorite {
			result = append(result, item)
			continue
		}

		// Prefer grouping by normalized clean Japanese title if available
		var key string
		if item.OriginalTitle != "" {
			cleanJa := normalizeTitleForDedupe(CleanMovieTitle(item.OriginalTitle))
			if len(cleanJa) >= 6 {
				key = "JA:" + cleanJa
			}
		}

		if key == "" {
			cleanT := normalizeTitleForDedupe(CleanMovieTitle(item.Title))
			if len(cleanT) >= 6 {
				key = "EN:" + cleanT
			}
		}

		// Fallback: If title is short or missing, group by canonical base ID if it is a promotional/variant SKU (starts with TK or ends with EC)
		if key == "" {
			upperID := strings.ToUpper(item.MovieID)
			if strings.HasPrefix(upperID, "TK") || strings.HasSuffix(upperID, "-EC") || strings.HasSuffix(upperID, "-T-EC") || strings.HasSuffix(upperID, "EC") {
				baseID := canonicalBaseID(item.MovieID)
				if baseID != "" && strings.Contains(baseID, "-") {
					key = "ID:" + baseID
				}
			}
		}

		if key == "" {
			result = append(result, item)
			continue
		}

		if _, exists := titleGroups[key]; !exists {
			orderedKeys = append(orderedKeys, key)
		}
		titleGroups[key] = append(titleGroups[key], item)
	}

	// Index already downloaded titles so unowned duplicate variants don't show up
	seenDownloaded := make(map[string]bool)
	for _, item := range result {
		if item.OriginalTitle != "" {
			cleanJa := normalizeTitleForDedupe(CleanMovieTitle(item.OriginalTitle))
			if len(cleanJa) >= 6 {
				seenDownloaded["JA:"+cleanJa] = true
			}
		}
		cleanT := normalizeTitleForDedupe(CleanMovieTitle(item.Title))
		if len(cleanT) >= 6 {
			seenDownloaded["EN:"+cleanT] = true
		}
		upperID := strings.ToUpper(item.MovieID)
		if strings.HasPrefix(upperID, "TK") || strings.HasSuffix(upperID, "-EC") || strings.HasSuffix(upperID, "-T-EC") || strings.HasSuffix(upperID, "EC") {
			baseID := canonicalBaseID(item.MovieID)
			if baseID != "" && strings.Contains(baseID, "-") {
				seenDownloaded["ID:"+baseID] = true
			}
		}
	}

	// For each unowned title group, pick the most canonical release
	for _, key := range orderedKeys {
		if seenDownloaded[key] {
			continue
		}
		group := titleGroups[key]
		if len(group) == 1 {
			result = append(result, group[0])
			continue
		}

		// Sort group: highest canonical score first (standard ID > TK variant), then earliest release date
		sort.SliceStable(group, func(i, j int) bool {
			scoreI := scoreReleaseCanon(group[i])
			scoreJ := scoreReleaseCanon(group[j])
			if scoreI != scoreJ {
				return scoreI > scoreJ
			}
			dateI := group[i].ReleaseDate
			dateJ := group[j].ReleaseDate
			if dateI != "" && dateJ != "" && dateI != dateJ {
				return dateI < dateJ
			}
			return group[i].MovieID < group[j].MovieID
		})

		result = append(result, group[0])
	}

	// Re-sort all results by release date DESC
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ReleaseDate > result[j].ReleaseDate
	})

	return result
}

// CheckFilmographyInclusion evaluates whether a movie should be included in the primary filmography,
// or whether it represents a non-video item, variety show, clip compilation, or duplicate variant.
// Returns (shouldSkip bool, skipReason string).
func CheckFilmographyInclusion(movieID, title, originalTitle, coverURL string, genres []string, actressCount ...int) (bool, string) {
	upperID := strings.ToUpper(strings.TrimSpace(movieID))
	allText := strings.TrimSpace(title + " " + originalTitle)
	allLower := strings.ToLower(allText)

	// 1. Non-video digital e-book / photobook checks
	if strings.Contains(coverURL, "ebook-assets") || strings.Contains(coverURL, "/e-book/") {
		return true, "Photobook / Digital Book"
	}
	if strings.HasPrefix(upperID, "B600") || strings.HasPrefix(upperID, "D600") || strings.HasPrefix(upperID, "DG") {
		return true, "Photobook / Digital Book"
	}
	for _, kw := range []string{"写真集", "デジタル写真集", "ポーズブック", "フォトブック", "電子書籍", "photobook", "photo book"} {
		if strings.Contains(allLower, kw) {
			return true, "Photobook / Digital Book"
		}
	}

	// 2. Variety talk show series & known compilation series (e.g. KCKC-, MLTN-, BMW-)
	if strings.HasPrefix(upperID, "KCKC") || strings.HasPrefix(upperID, "MLTN") || strings.HasPrefix(upperID, "BMW") ||
		strings.Contains(allText, "カチコチTV") || strings.Contains(allText, "カチコチ") {
		return true, "Variety / Talk Show"
	}

	// 3. Online autograph sessions, event tickets & non-video participation goods
	if strings.Contains(allText, "オンラインサイン会") ||
		strings.Contains(allText, "参加URL付き") ||
		strings.Contains(allText, "参加URL付") ||
		strings.Contains(allText, "参加権付き") ||
		strings.Contains(allText, "参加権付") {
		return true, "Event / Autograph Session"
	}

	// 4. Re-issue Director's Cut / Remaster duplicates (e.g. SSIS-160 to SSIS-165, JQRE-027 AI Remaster)
	if strings.Contains(allText, "未公開映像収録") ||
		strings.Contains(allText, "ディレクターズカット") ||
		strings.Contains(allText, "AIリマスター") ||
		strings.Contains(allLower, "ai remaster") ||
		strings.Contains(allText, "デジタルリマスター") ||
		strings.Contains(allLower, "digital remaster") ||
		strings.Contains(allText, "リマスター") ||
		strings.Contains(allLower, "remaster") ||
		strings.Contains(allText, "復刻") ||
		strings.HasPrefix(upperID, "JQRE") {
		return true, "Remaster / Director's Cut"
	}

	// 5. Promotional SKU prefixes: TK... (e.g. TKCJOD-510, TKMFYD-123, TKCAWB-040), C9, E9, S9, N9, L9, K9, KA9, KC9, TK9, 9 followed by letters, -EC, -T-EC
	if promoSkuRegex.MatchString(upperID) || strings.HasSuffix(upperID, "-EC") || strings.HasSuffix(upperID, "-T-EC") || strings.HasSuffix(upperID, "EC") {
		return true, "Promotional SKU Variant"
	}

	// 6. Title markers (Cheki sets, goods bundles, purchase bonuses)
	if strings.Contains(allText, "キーホルダーセット") ||
		strings.Contains(allText, "チェキセット") ||
		strings.Contains(allText, "チェキ付き") ||
		strings.Contains(allText, "チェキ付") ||
		strings.Contains(allText, "購入特典付き") ||
		strings.Contains(allText, "購入特典付") ||
		strings.Contains(allLower, "cheki set") ||
		strings.Contains(allLower, "cheki") ||
		strings.Contains(allLower, "polaroid set") {
		return true, "Promotional Bundle Variant"
	}

	// 7. Tag / Genre checks (R18 / DMM categories)
	for _, g := range genres {
		gNorm := strings.ToLower(strings.TrimSpace(g))
		if gNorm == "anime" || gNorm == "animation" || gNorm == "game" || gNorm == "comic" || gNorm == "manga" {
			return true, "Non-Video Media"
		}
		if gNorm == "photo book" || gNorm == "digital photo" || gNorm == "collection of photographs" {
			return true, "Photobook / Digital Book"
		}
		if gNorm == "special offers and set products" || strings.Contains(gNorm, "set products") {
			return true, "Promotional Bundle Variant"
		}
		if gNorm == "includes event participation rights" || strings.Contains(gNorm, "event participation") {
			return true, "Event / Autograph Session"
		}
		if gNorm == "compilation" || gNorm == "omnibus" {
			return true, "Omnibus Compilation"
		}
	}

	// 8. Known Omnibus series (RBB-, MKCK-)
	if strings.HasPrefix(upperID, "RBB") || strings.HasPrefix(upperID, "MKCK") {
		return true, "Omnibus Compilation"
	}

	// 9. Title markers (compilations)
	if strings.Contains(allText, "総集編") ||
		strings.Contains(allText, "オムニバス") ||
		strings.Contains(allText, "傑作選") ||
		compilationRegex.MatchString(allText) {
		return true, "Omnibus Compilation"
	}

	// 10. Multi-actress omnibus compilation checks (Group 1)
	// Exemption for Group 5: Official anniversary crossover harem works are genuine productions, not clip omnibus
	isAnniversary := strings.Contains(allText, "周年") ||
		strings.Contains(allLower, "anniversary") ||
		strings.Contains(allText, "記念作品") ||
		strings.Contains(allText, "創立")

	if !isAnniversary {
		actCount := 0
		if len(actressCount) > 0 {
			actCount = actressCount[0]
		}

		// Multi-actress omnibus (e.g. MKCK-417 with 74 actresses, RBB-279 with 49 actresses, REbecca STARS with 12 actresses)
		if actCount >= 10 || strings.Contains(allText, "REbecca STARS") {
			return true, "Omnibus Compilation"
		}

		// Multi-actress (>= 5) with Over 4 Hours tag or long minute titles
		if actCount >= 5 {
			for _, g := range genres {
				if strings.EqualFold(strings.TrimSpace(g), "over 4 hours") {
					return true, "Omnibus Compilation"
				}
			}
			if strings.Contains(allLower, "600min") || strings.Contains(allLower, "480分") || strings.Contains(allLower, "300分") {
				return true, "Omnibus Compilation"
			}
		}

		// Multi-body title patterns (e.g. RKI-114: 50体480分) or 600min omnibus
		if multiBodyRegex.MatchString(allText) || strings.Contains(allLower, "600min") {
			return true, "Omnibus Compilation"
		}
	}

	return false, ""
}

// IsPromotionalOrDuplicateVariant checks if a title/product represents a promotional variant,
// event ticket, digital photo book, duplicate bundle, or omnibus compilation.
func IsPromotionalOrDuplicateVariant(movieID, title string, args ...any) bool {
	originalTitle := ""
	coverURL := ""
	var genres []string
	var actCount []int

	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			if originalTitle == "" && coverURL == "" && !strings.HasPrefix(v, "http") {
				originalTitle = v
			} else if coverURL == "" {
				coverURL = v
			}
		case []string:
			genres = v
		case int:
			actCount = append(actCount, v)
		}
	}

	skip, _ := CheckFilmographyInclusion(movieID, title, originalTitle, coverURL, genres, actCount...)
	return skip
}

func isPromotionalGenre(genre string) bool {
	gNorm := strings.ToLower(strings.TrimSpace(genre))
	// 1. Promotional & Non-movie tags
	if gNorm == "special offers and set products" ||
		gNorm == "includes event participation rights" ||
		gNorm == "compilation" ||
		gNorm == "collection of photographs" ||
		gNorm == "omnibus" ||
		gNorm == "anime" ||
		gNorm == "animation" ||
		gNorm == "game" ||
		gNorm == "comic" ||
		gNorm == "manga" ||
		strings.Contains(gNorm, "set products") ||
		strings.Contains(gNorm, "event participation") ||
		strings.Contains(gNorm, "photo book") ||
		strings.Contains(gNorm, "digital photo") ||
		strings.Contains(gNorm, "photograph") ||
		strings.Contains(gNorm, "compilation") {
		return true
	}

	// 2. Technical specs, distribution channels, and formats (not acting themes)
	switch gNorm {
	case "exclusive distribution",
		"featured actress",
		"hi-def",
		"4k",
		"8kvr",
		"high-quality vr",
		"vr exclusive",
		"over 4 hours",
		"documentary",
		"debut":
		return true
	}

	return false
}



