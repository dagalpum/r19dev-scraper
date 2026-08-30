package scraper

import "time"

// Actress represents performer metadata.
type Actress struct {
	Name     string `json:"name"`
	JaName   string `json:"ja_name,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// Movie represents scraped metadata for a JAV video.
type Movie struct {
	ID                string    `json:"id"`                 // Standard ID, e.g. "MIDA-517"
	CombinedID        string    `json:"combined_id"`        // R18.dev combined ID, e.g. "mida00517"
	Title             string    `json:"title"`              // Title (English or Japanese)
	OriginalTitle     string    `json:"original_title"`     // Japanese Title
	Maker             string    `json:"maker"`              // Studio / Maker (e.g. "MOODYZ")
	Label             string    `json:"label,omitempty"`    // Sub-label
	Series            string    `json:"series,omitempty"`   // Series name
	Director          string    `json:"director,omitempty"` // Director
	ReleaseDate       string    `json:"release_date"`       // YYYY-MM-DD
	RuntimeMinutes    int       `json:"runtime_minutes"`    // Duration in minutes
	Actresses         []Actress `json:"actresses"`          // List of performers
	Genres            []string  `json:"genres"`             // List of genres/categories
	CoverURL          string    `json:"cover_url"`          // Full jacket cover URL
	PosterURL         string    `json:"poster_url"`         // Cropped front poster URL
	SampleScreenshots []string  `json:"sample_screenshots"` // Sample gallery images
	DetailURL         string    `json:"detail_url"`         // Web link
	ScrapedAt         time.Time `json:"scraped_at"`
}
