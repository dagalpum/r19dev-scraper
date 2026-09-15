package scraper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPromotionalOrOmnibusVariant(t *testing.T) {
	// Ensure default configuration
	_ = SaveFilterConfig(DefaultFilterConfig())

	testCases := []struct {
		id       string
		title    string
		orig     string
		expected bool
	}{
		{"1DLDSS559TK", "Title", "【数量限定】... パンティと写真付き", true},
		{"1DLDSS545TK", "Title", "【数量限定】... パンティと写真付き", true},
		{"MKMP-765", "中出し懇願 白濁マ○コBEST 淫乱30人種付け240分", "中出し懇願 白濁マ○コBEST 淫乱30人種付け240分", true},
		{"MIZD-550", "Hinako Mori: MOODYZ Exclusive 1st BEST", "森日向子 MOODYZ専属1stBEST", true},
		{"9OFJE-548", "Compilation", "オムニバス", true},
		{"1SETH00008", "Compilation", "オムニバス", true},
		{"TKCJOD-510", "Title", "チェキ付き", true},
		{"FSDSS-559TK1", "Title", "生写真付き", true},
		{"IPOK-035", "AIが導き出した...", "オナニー特化アングル100本番", true},
		// Genuine releases should NOT be skipped
		{"DLDSS-559", "Teacher Romance", "’多分、娘が好きであろう’娘の家庭●師と週3日、自宅不貞行為に及んでいますー。 角奈保", false},
		{"DLDSS-545", "Teacher Romance", "不良生徒から優等生を守るため…人妻女●師 放課後肉便器 角奈保/花守夏歩", false},
		{"SNOS-038", "Sample Movie", "Sample", false},
		{"SONE-682", "Potential Eros Explosion Of The Strongest Heroine Kanna Seto", "最強ヒロインの潜在的エロス爆発 瀬戸環奈", false},
	}

	for _, tc := range testCases {
		skip, reason := IsPromotionalOrOmnibusVariant(tc.id, tc.title, tc.orig, "", nil)
		if skip != tc.expected {
			t.Errorf("IsPromotionalOrOmnibusVariant(%s) = (%v, %s); expected skip = %v", tc.id, skip, reason, tc.expected)
		}
	}

	// Test Label and Series checks specifically
	if skip, _ := IsPromotionalOrOmnibusVariantWithDetails("CUSTOM-001", "Title", "Orig", "Idea Pocket BEST", "", "", nil); !skip {
		t.Errorf("Expected Idea Pocket BEST label to be skipped")
	}
	if skip, _ := IsPromotionalOrOmnibusVariantWithDetails("CUSTOM-002", "Title", "Orig", "", "Idea Pocket BEST", "", nil); !skip {
		t.Errorf("Expected Idea Pocket BEST series to be skipped")
	}
}

func TestDynamicFilterConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filter_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "filters.json")

	// 1. Load non-existent file should create default
	cfg, err := LoadFilterConfig(configPath)
	if err != nil {
		t.Fatalf("LoadFilterConfig error: %v", err)
	}
	if len(cfg.BlockedPrefixes) == 0 {
		t.Fatalf("Expected default blocked prefixes")
	}

	// 2. Add custom prefix and test
	cfg.BlockedPrefixes = append(cfg.BlockedPrefixes, "MYCUSTOM")
	cfg.BlockedLabels = append(cfg.BlockedLabels, "My Custom Super BEST")
	cfg.BlockedTitleKeywords = append(cfg.BlockedTitleKeywords, "พิเศษสุดคุ้ม")
	if err := SaveFilterConfig(cfg, configPath); err != nil {
		t.Fatalf("SaveFilterConfig error: %v", err)
	}

	// Reload config
	reloadedCfg, err := LoadFilterConfig(configPath)
	if err != nil {
		t.Fatalf("LoadFilterConfig reloaded error: %v", err)
	}

	// Check custom prefix
	if skip, reason := reloadedCfg.IsExcluded("MYCUSTOM-123", "Title", "Orig", "", "", "", nil); !skip {
		t.Errorf("Expected MYCUSTOM-123 to be excluded, got (%v, %s)", skip, reason)
	}

	// Check custom label
	if skip, reason := reloadedCfg.IsExcluded("GENUINE-001", "Title", "Orig", "My Custom Super BEST Label", "", "", nil); !skip {
		t.Errorf("Expected custom label to be excluded, got (%v, %s)", skip, reason)
	}

	// Check custom keyword
	if skip, reason := reloadedCfg.IsExcluded("GENUINE-002", "Title พิเศษสุดคุ้ม", "Orig", "", "", "", nil); !skip {
		t.Errorf("Expected custom keyword to be excluded, got (%v, %s)", skip, reason)
	}

	// 3. Reset config
	resetCfg, err := ResetFilterConfig(configPath)
	if err != nil {
		t.Fatalf("ResetFilterConfig error: %v", err)
	}
	if skip, _ := resetCfg.IsExcluded("MYCUSTOM-123", "Title", "Orig", "", "", "", nil); skip {
		t.Errorf("Expected MYCUSTOM-123 to NOT be excluded after reset")
	}
}
