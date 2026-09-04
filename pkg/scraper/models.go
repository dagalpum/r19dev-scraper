package scraper

import "time"

// Actress represents performer metadata.
type Actress struct {
	ID       int    `json:"id,omitempty"`        // R18.dev / DMM Actress ID (e.g. 1109487)
	Name     string `json:"name"`                // Romaji / English Name (e.g. "Sakura Miura")
	JaName   string `json:"ja_name,omitempty"`   // Kanji / Japanese Name (e.g. "水卜さくら")
	ImageURL string `json:"image_url,omitempty"` // Actress thumbnail
}

// Movie represents scraped metadata for a JAV video.
type Movie struct {
	ID                string    `json:"id"`                 // Standard ID, e.g. "MIDA-517"
	CombinedID        string    `json:"combined_id"`        // R18.dev combined ID, e.g. "mida00517"
	Title             string    `json:"title"`              // Primary Title (English if language="en")
	OriginalTitle     string    `json:"original_title"`     // Original Japanese Title
	Maker             string    `json:"maker"`              // Studio / Maker (e.g. "MOODYZ")
	Label             string    `json:"label,omitempty"`    // Sub-label (e.g. "MOODYZ DIVA")
	Series            string    `json:"series,omitempty"`   // Series name
	Director          string    `json:"director,omitempty"` // Director (e.g. "Amazing Meat")
	ReleaseDate       string    `json:"release_date"`       // YYYY-MM-DD
	RuntimeMinutes    int       `json:"runtime_minutes"`    // Duration in minutes
	Actresses         []Actress `json:"actresses"`          // List of performers
	Genres            []string  `json:"genres"`             // List of genres/categories (English)
	CoverURL          string    `json:"cover_url"`          // Full jacket cover URL
	PosterURL         string    `json:"poster_url"`         // Front poster URL
	TrailerURL        string    `json:"trailer_url"`        // Sample trailer video URL
	SampleScreenshots []string  `json:"sample_screenshots"` // Sample gallery images
	DetailURL         string    `json:"detail_url"`         // Web link
	ScrapedAt         time.Time `json:"scraped_at"`
}
