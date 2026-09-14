package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dagalp/r19dev-scraper/pkg/actress"
	_ "modernc.org/sqlite"
)

func main() {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, "Library", "Application Support", "r19dev", "r19dev.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Query unowned movies
	rows, err := db.Query(`
		SELECT id, COALESCE(title, ''), COALESCE(original_title, ''), COALESCE(cover_url, ''), COALESCE(genres_json, '[]'), COALESCE(actresses_json, '[]')
		FROM movies
		WHERE id NOT IN (SELECT DISTINCT movie_id FROM library_files WHERE file_path != '')
		  AND id NOT IN (SELECT DISTINCT movie_id FROM organized_movies WHERE target_folder != '' OR target_video != '')
		  AND id NOT IN (SELECT DISTINCT movie_id FROM user_state WHERE is_watched = 1 OR is_favorite = 1)
	`)
	if err != nil {
		log.Fatalf("query unowned: %v", err)
	}
	defer rows.Close()

	type Movie struct {
		ID            string
		Title         string
		OriginalTitle string
		CoverURL      string
		Genres        []string
		ActCount      int
	}
	var unowned []Movie
	for rows.Next() {
		var m Movie
		var gJSON, aJSON string
		if err := rows.Scan(&m.ID, &m.Title, &m.OriginalTitle, &m.CoverURL, &gJSON, &aJSON); err != nil {
			log.Fatalf("scan: %v", err)
		}
		if gJSON != "" && gJSON != "[]" {
			_ = json.Unmarshal([]byte(gJSON), &m.Genres)
		}
		var acts []any
		if aJSON != "" && aJSON != "[]" {
			_ = json.Unmarshal([]byte(aJSON), &acts)
		}
		m.ActCount = len(acts)
		unowned = append(unowned, m)
	}

	var toDelete []string
	reasonCounts := make(map[string]int)

	for _, m := range unowned {
		skip, reason := actress.CheckFilmographyInclusion(m.ID, m.Title, m.OriginalTitle, m.CoverURL, m.Genres, m.ActCount)
		if skip {
			toDelete = append(toDelete, m.ID)
			reasonCounts[reason]++
		}
	}

	fmt.Printf("Total unowned scanned: %d\n", len(unowned))
	fmt.Printf("Found %d unowned movies to purge:\n", len(toDelete))
	for r, c := range reasonCounts {
		fmt.Printf("  - %s: %d\n", r, c)
	}

	if len(toDelete) == 0 {
		fmt.Println("Nothing to delete.")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	delStmt, err := tx.Prepare("DELETE FROM movies WHERE id = ?")
	if err != nil {
		log.Fatalf("prepare del: %v", err)
	}
	defer delStmt.Close()

	for _, id := range toDelete {
		if _, err := delStmt.Exec(id); err != nil {
			log.Printf("del error on %s: %v", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Println("Successfully purged filtered unowned compilation and variant movies from r19dev.db!")
}
