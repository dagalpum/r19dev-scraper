package torrent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"6.5 GiB", int64(6.5 * 1024 * 1024 * 1024)},
		{"800.0 MiB", int64(800 * 1024 * 1024)},
		{"1.2 TiB", 1319413953331},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseSize(tt.input)
		if got != tt.expected {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestSukebeiSearchMock(t *testing.T) {
	sampleRSS := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:nyaa="https://sukebei.nyaa.si/xmlns/nyaa">
  <channel>
    <title>Sukebei - "SNOS-250"</title>
    <item>
      <title>[FHD] SNOS-250 miru Accounting Senior</title>
      <link>https://sukebei.nyaa.si/download/1001.torrent</link>
      <guid>https://sukebei.nyaa.si/view/1001</guid>
      <pubDate>Wed, 26 Aug 2026 13:23:39 -0000</pubDate>
      <nyaa:seeders>12</nyaa:seeders>
      <nyaa:leechers>2</nyaa:leechers>
      <nyaa:downloads>150</nyaa:downloads>
      <nyaa:infoHash>42bf9453a53ab13267953ffbb25392c931add6d1</nyaa:infoHash>
      <nyaa:category>Real Life - Videos</nyaa:category>
      <nyaa:size>6.5 GiB</nyaa:size>
      <nyaa:trusted>Yes</nyaa:trusted>
    </item>
    <item>
      <title>SNOS-250 SD low res</title>
      <link>https://sukebei.nyaa.si/download/1002.torrent</link>
      <guid>https://sukebei.nyaa.si/view/1002</guid>
      <pubDate>Mon, 24 Aug 2026 10:00:00 -0000</pubDate>
      <nyaa:seeders>0</nyaa:seeders>
      <nyaa:leechers>0</nyaa:leechers>
      <nyaa:downloads>10</nyaa:downloads>
      <nyaa:infoHash>abcdef1234567890abcdef1234567890abcdef12</nyaa:infoHash>
      <nyaa:category>Real Life - Videos</nyaa:category>
      <nyaa:size>1.1 GiB</nyaa:size>
      <nyaa:trusted>No</nyaa:trusted>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer ts.Close()

	client := NewSukebeiClient(ts.URL)
	res, err := client.Search(context.Background(), "SNOS-250")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if res.Total != 2 {
		t.Fatalf("Expected 2 items, got %d", res.Total)
	}

	if res.Recommended == nil {
		t.Fatalf("Expected a recommended item, got nil")
	}

	if res.Recommended.Resolution != "1080p" {
		t.Errorf("Expected 1080p, got %s", res.Recommended.Resolution)
	}

	if res.Recommended.Score <= res.Items[1].Score {
		t.Errorf("Recommended score %d should be higher than item 1 score %d", res.Recommended.Score, res.Items[1].Score)
	}
}
