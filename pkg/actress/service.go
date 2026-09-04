package actress

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

// ReleaseItem holds filmography release details with download, watch, and rating status.
type ReleaseItem struct {
	MovieID         string `json:"movie_id"`
	Title           string `json:"title"`
	OriginalTitle   string `json:"original_title"`
	Maker           string `json:"maker"`
	ReleaseDate     string `json:"release_date"`
	CoverURL        string `json:"cover_url"`
	IsDownloaded    bool   `json:"is_downloaded"`
	IsWatched       bool   `json:"is_watched"`
	UserRating      int    `json:"user_rating"`
	IsFavorite      bool   `json:"is_favorite"`
	LibraryPath     string `json:"library_path,omitempty"`
	OrganizedFolder string `json:"organized_folder,omitempty"`
	OrganizedVideo  string `json:"organized_video,omitempty"`
}

// ActressSummary aggregates release statistics for a followed actress.
type ActressSummary struct {
	Actress    db.ActressRecord `json:"actress"`
	Releases   []ReleaseItem    `json:"releases"`
	Total      int              `json:"total"`
	Downloaded int              `json:"downloaded"`
	Missing    int              `json:"missing"`
	Watched    int              `json:"watched"`
	Favorites  int              `json:"favorites"`
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
	SELECT m.id, m.title, m.original_title, m.maker, m.release_date, m.cover_url, m.actresses_json,
	       COALESCE(u.is_watched, 0), COALESCE(u.user_rating, 0), COALESCE(u.is_favorite, 0),
	       MAX(lf.file_path),
	       MAX(om.target_folder), MAX(om.target_video)
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

	for rows.Next() {
		var r ReleaseItem
		var actJSON string
		var libPath sql.NullString
		var orgFolder sql.NullString
		var orgVideo sql.NullString
		if err := rows.Scan(
			&r.MovieID, &r.Title, &r.OriginalTitle, &r.Maker, &r.ReleaseDate, &r.CoverURL, &actJSON,
			&r.IsWatched, &r.UserRating, &r.IsFavorite,
			&libPath,
			&orgFolder, &orgVideo,
		); err != nil {
			return nil, err
		}

		if libPath.Valid && libPath.String != "" {
			r.IsDownloaded = true
			r.LibraryPath = libPath.String
			downloadedCount++
		}
		if orgFolder.Valid {
			r.OrganizedFolder = orgFolder.String
		}
		if orgVideo.Valid {
			r.OrganizedVideo = orgVideo.String
		}
		if r.IsWatched {
			watchedCount++
		}
		if r.IsFavorite {
			favCount++
		}

		releases = append(releases, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary := &ActressSummary{
		Actress:    actRec,
		Releases:   releases,
		Total:      len(releases),
		Downloaded: downloadedCount,
		Missing:    len(releases) - downloadedCount,
		Watched:    watchedCount,
		Favorites:  favCount,
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
