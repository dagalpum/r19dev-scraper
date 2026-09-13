package scraper

import (
	"testing"
)

func TestDumpStoreLookup(t *testing.T) {
	ds := DefaultDumpStore()
	if ds == nil {
		t.Skip("r18_dump.db not found, skipping offline dump test")
	}

	// Test with snos00115 / SNOS-115
	movie, found := ds.GetMovie("SNOS-115", "en")
	if !found || movie == nil {
		t.Fatalf("Expected to find SNOS-115 in dump store, but not found")
	}

	if movie.CombinedID != "snos00115" {
		t.Errorf("Expected combinedID snos00115, got %s", movie.CombinedID)
	}

	if movie.Title == "" || movie.Title == movie.OriginalTitle {
		t.Errorf("Expected English title, got empty or Japanese: %s", movie.Title)
	}

	if movie.Maker != "S1 NO.1 STYLE" {
		t.Errorf("Expected maker S1 NO.1 STYLE, got %s", movie.Maker)
	}

	if len(movie.Actresses) == 0 {
		t.Errorf("Expected at least 1 actress for SNOS-115, got 0")
	} else {
		t.Logf("Found movie: %s | %s | %s | Actresses: %v", movie.ID, movie.Maker, movie.Title, movie.Actresses[0].JaName)
	}
}
