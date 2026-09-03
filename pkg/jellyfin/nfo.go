package jellyfin

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

// MovieNFO maps standard Kodi / Jellyfin Movie NFO XML format.
type MovieNFO struct {
	XMLName       xml.Name       `xml:"movie"`
	Title         string         `xml:"title"`
	OriginalTitle string         `xml:"originaltitle,omitempty"`
	SortTitle     string         `xml:"sorttitle,omitempty"`
	Set           *SetInfo       `xml:"set,omitempty"`
	UserRating    float64        `xml:"userrating,omitempty"`
	Year          string         `xml:"year,omitempty"`
	Plot          string         `xml:"plot,omitempty"`
	Runtime       int            `xml:"runtime,omitempty"`
	Poster        string         `xml:"poster,omitempty"`
	Fanart        *FanartInfo    `xml:"fanart,omitempty"`
	MPAA          string         `xml:"mpaa,omitempty"`
	ID            string         `xml:"id,omitempty"`
	UniqueID      UniqueIDInfo   `xml:"uniqueid"`
	Genres        []string       `xml:"genre,omitempty"`
	Studio        string         `xml:"studio,omitempty"`
	Director      string         `xml:"director,omitempty"`
	Premiered     string         `xml:"premiered,omitempty"`
	ReleaseDate   string         `xml:"releasedate,omitempty"`
	Actors        []ActorInfo    `xml:"actor,omitempty"`
	Watched       bool           `xml:"watched"`
	PlayCount     int            `xml:"playcount,omitempty"`
}

type SetInfo struct {
	Name string `xml:"name"`
}

type FanartInfo struct {
	Thumb string `xml:"thumb"`
}

type UniqueIDInfo struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

type ActorInfo struct {
	Name  string `xml:"name"`
	Role  string `xml:"role,omitempty"`
	Thumb string `xml:"thumb,omitempty"`
}

// GenerateNFO builds XML bytes for a given Movie and optional UserState.
func GenerateNFO(movie *scraper.Movie, userState *db.UserState) ([]byte, error) {
	if movie == nil {
		return nil, fmt.Errorf("movie is nil")
	}

	year := ""
	if len(movie.ReleaseDate) >= 4 {
		year = movie.ReleaseDate[:4]
	}

	displayTitle := strings.TrimSpace(movie.Title)
	if displayTitle == "" {
		displayTitle = strings.TrimSpace(movie.OriginalTitle)
	}
	formattedTitle := fmt.Sprintf("[%s] %s", movie.ID, displayTitle)
	if displayTitle == "" {
		formattedTitle = movie.ID
	}

	nfo := MovieNFO{
		Title:         formattedTitle,
		OriginalTitle: movie.OriginalTitle,
		SortTitle:     movie.ID,
		Year:          year,
		Plot:          displayTitle,
		Runtime:       movie.RuntimeMinutes,
		Poster:        "poster.jpg",
		Fanart:        &FanartInfo{Thumb: "fanart.jpg"},
		MPAA:          "XXX",
		ID:            movie.ID,
		UniqueID: UniqueIDInfo{
			Type:    "jav",
			Default: "true",
			Value:   movie.ID,
		},
		Studio:      movie.Maker,
		Director:    movie.Director,
		Premiered:   movie.ReleaseDate,
		ReleaseDate: movie.ReleaseDate,
		Genres:      movie.Genres,
	}

	if movie.Maker != "" {
		nfo.Set = &SetInfo{Name: movie.Maker}
	}

	for _, act := range movie.Actresses {
		name := act.Name
		if act.JaName != "" && act.JaName != act.Name {
			name = fmt.Sprintf("%s (%s)", act.Name, act.JaName)
		}
		nfo.Actors = append(nfo.Actors, ActorInfo{
			Name:  name,
			Role:  "Actress",
			Thumb: act.ImageURL,
		})
	}

	if userState != nil {
		if userState.IsWatched {
			nfo.Watched = true
			nfo.PlayCount = 1
		}
		if userState.UserRating > 0 {
			nfo.UserRating = float64(userState.UserRating) * 2.0 // Convert 1-5 to 10-point scale
		}
	}

	data, err := xml.MarshalIndent(nfo, "", "  ")
	if err != nil {
		return nil, err
	}

	xmlHeader := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\" ?>\n")
	return append(xmlHeader, data...), nil
}

// WriteNFO generates and writes the NFO file to destPath.
func WriteNFO(movie *scraper.Movie, userState *db.UserState, destPath string) error {
	data, err := GenerateNFO(movie, userState)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(destPath, data, 0o644)
}

// SanitizeFilename cleans invalid filesystem characters from folder/file names.
func SanitizeFilename(name string) string {
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, " ")
	}
	// Collapse multiple spaces
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}
	return strings.TrimSpace(name)
}
