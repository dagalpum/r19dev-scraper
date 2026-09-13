package scraper

import (
	"context"
	"testing"
	"time"
)

func TestClientScrapeOffline(t *testing.T) {
	client := NewClient(5 * time.Second)
	if client.DumpStore() == nil {
		t.Skip("DumpStore not loaded, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	movie, err := client.Scrape(ctx, "SNOS-115")
	if err != nil {
		t.Fatalf("Scrape error: %v", err)
	}

	if movie == nil {
		t.Fatalf("Expected non-nil movie")
	}

	if movie.ID != "SNOS-115" {
		t.Errorf("Expected ID SNOS-115, got %s", movie.ID)
	}

	if movie.Title == "" {
		t.Errorf("Expected non-empty title")
	}

	t.Logf("Offline Scrape SUCCESS: %s - %s (%s)", movie.ID, movie.Title, movie.Maker)
}
