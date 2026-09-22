package torrent

import (
	"time"
)

// TorrentItem represents a parsed and scored torrent release from Sukebei/Nyaa.
type TorrentItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Link          string    `json:"link"`          // Direct .torrent download URL
	ViewURL       string    `json:"view_url"`      // Sukebei web page URL
	InfoHash      string    `json:"info_hash"`     // BT infoHash (hex)
	MagnetURL     string    `json:"magnet_url"`    // Formatted magnet:?xt=urn:btih:...
	SizeFormatted string    `json:"size_formatted"`
	SizeBytes     int64     `json:"size_bytes"`
	PubDate       time.Time `json:"pub_date"`
	Seeders       int       `json:"seeders"`
	Leechers      int       `json:"leechers"`
	Downloads     int       `json:"downloads"`
	Category      string    `json:"category"`
	IsTrusted     bool      `json:"is_trusted"`

	// Smart Scoring Metadata
	Resolution    string `json:"resolution"`     // "4K", "1080p", "720p", "SD"
	IsUncensored  bool   `json:"is_uncensored"`  // [UN], [無碼], [Uncensored]
	HasSubtitles  bool   `json:"has_subtitles"`  // [C], [ch], [中文字幕]
	Score         int    `json:"score"`          // Calculated smart ranking score
	IsRecommended bool   `json:"is_recommended"` // Highest scored matching release
}

// TorrentSearchResult represents the response of a torrent search.
type TorrentSearchResult struct {
	Query       string        `json:"query"`
	Total       int           `json:"total"`
	Recommended *TorrentItem  `json:"recommended,omitempty"`
	Items       []TorrentItem `json:"items"`
}

// TransmissionConfig holds credentials and endpoint configuration for Transmission.
type TransmissionConfig struct {
	URL         string `json:"url"`          // e.g. "http://192.168.1.189:9091"
	Username    string `json:"username"`
	Password    string `json:"password"`
	DownloadDir string `json:"download_dir"` // Optional custom download directory
}

// TransmissionTorrent represents a torrent in Transmission's active/completed queue.
type TransmissionTorrent struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	HashString   string  `json:"hash_string"`
	Status       int     `json:"status"` // 0: paused, 4: download, 6: seed
	StatusText   string  `json:"status_text"`
	PercentDone  float64 `json:"percent_done"`  // 0.0 - 1.0
	RateDownload int64   `json:"rate_download"` // bytes/sec
	RateUpload   int64   `json:"rate_upload"`   // bytes/sec
	ETA          int64   `json:"eta"`           // seconds remaining (-1 if unknown)
	TotalSize    int64   `json:"total_size"`    // bytes
	Downloaded   int64   `json:"downloaded"`    // bytes
	DownloadDir  string  `json:"download_dir"`
	IsFinished   bool    `json:"is_finished"`
	Error        int     `json:"error"`
	ErrorString  string  `json:"error_string"`
}

// DownloadQueueItem represents an entry in R19DEV's download & staging pipeline.
type DownloadQueueItem struct {
	ID             int       `json:"id"`
	MovieID        string    `json:"movie_id"`
	CombinedID     string    `json:"combined_id,omitempty"`
	MovieTitle     string    `json:"movie_title,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	ActressName    string    `json:"actress_name,omitempty"`
	TorrentHash    string    `json:"torrent_hash"`
	TorrentTitle   string    `json:"torrent_title"`
	TorrentURL     string    `json:"torrent_url"`
	MagnetURL      string    `json:"magnet_url"`
	FileSizeBytes  int64     `json:"file_size_bytes"`
	QualityTag     string    `json:"quality_tag"`
	Status         string    `json:"status"` // "queued", "downloading", "staging", "organized", "error"
	ProgressPct    float64   `json:"progress_pct"`
	DownloadSpeed  int64     `json:"download_speed"`
	ETASeconds     int64     `json:"eta_seconds"`
	TransmissionID int       `json:"transmission_id"`
	DownloadPath   string    `json:"download_path"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
