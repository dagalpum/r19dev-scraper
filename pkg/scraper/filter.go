package scraper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// FilterConfig defines user-customizable rules to exclude promotional variants,
// goods bundles, non-video items, and omnibus compilations.
type FilterConfig struct {
	Version                     int      `json:"version"`
	Enabled                     bool     `json:"enabled"`
	BlockedPrefixes             []string `json:"blocked_prefixes"`
	BlockedLabels               []string `json:"blocked_labels"`
	BlockedSeries               []string `json:"blocked_series"`
	BlockedGenres               []string `json:"blocked_genres"`
	BlockedTitleKeywords        []string `json:"blocked_title_keywords"`
	BlockedTitleRegex           []string `json:"blocked_title_regex"`
	BlockedCoverPatterns        []string `json:"blocked_cover_patterns"`
	MinActressOmnibusCount      int      `json:"min_actress_omnibus_count"`
	MaxDurationMinutesThreshold int      `json:"max_duration_minutes_threshold"`

	// In-memory compiled regexes (not serialized to JSON)
	compiledTitleRegex  []*regexp.Regexp `json:"-"`
	compiledLabelRegex  *regexp.Regexp   `json:"-"`
	compiledSeriesRegex *regexp.Regexp   `json:"-"`
}

var (
	defaultPromoSkuPrefixRegex = regexp.MustCompile(`(?i)^(?:TK[A-Z]{2,6}[-_]?\d+|[A-Z]?[4679][A-Z]{2,6}[-_]?\d+|(?:77|88)[A-Z]{3,6}[-_]?\d+|S209|C209|E209)`)
	defaultPromoSkuSuffixRegex = regexp.MustCompile(`(?i)(?:[-_]?TK\d*|[-_]?EC|-T-EC)$`)
	defaultGoodsBundleRegex    = regexp.MustCompile(`(?i)パンティ|キーホルダー|購入特典|限定特典|オンラインサイン会|参加URL|参加権|チェキセット|チェキ付き?|ポラロイドセット|ポラロイド付き?|【(?:数量限定|FANZA限定|期間限定)】.*?(?:生写真|チェキ|写真|セット|付き?)`)

	configMu     sync.RWMutex
	activeConfig *FilterConfig
)

func init() {
	cfg := DefaultFilterConfig()
	cfg.compile()
	activeConfig = cfg
	_ = LoadDefaultFilterConfig()
}

// DefaultFilterConfig returns the standard set of filter rules.
func DefaultFilterConfig() *FilterConfig {
	return &FilterConfig{
		Version: 1,
		Enabled: true,
		BlockedPrefixes: []string{
			"IPOK", "IDBD", "MIZD", "MIDD", "PBD", "OBST", "SDDE",
			"RBB", "MKCK", "MKMP", "OFJE", "SETH", "OFRF", "OFMA",
		},
		BlockedLabels: []string{
			"Idea Pocket BEST",
			"MOODYZ Best",
			"PREMIUM BEST",
			"SOD BEST",
			"Madonna BEST",
			"S1 NO.1 STYLE BEST",
			"BEST",
			"ベスト",
			"総集編",
			"セレクション",
			"Compilation",
			"Omnibus",
			"Selection",
		},
		BlockedSeries: []string{
			"Idea Pocket BEST",
			"MOODYZ Best",
			"PREMIUM BEST",
			"SOD BEST",
			"Madonna BEST",
			"BEST",
			"ベスト",
			"総集編",
			"セレクション",
			"Compilation",
			"Omnibus",
			"Selection",
		},
		BlockedGenres: []string{
			"Compilation",
			"Omnibus",
			"Photo Book",
			"Digital Photo",
			"Collection of Photographs",
			"Special Offers and Set Products",
			"Includes Event Participation Rights",
			"Anime",
			"Animation",
			"Game",
			"Comic",
			"Manga",
		},
		BlockedTitleKeywords: []string{
			"ベストセレクション", "BESTセレクション", "総集編", "オムニバス",
			"傑作選", "名場面", "全集", "メモリアル", "プレミアムベスト",
			"メモリアルベスト", "コンプリートベスト", "神BEST", "連発", "連射",
			"時間BOX", "カチコチTV", "カチコチ", "AIリマスター", "デジタルリマスター",
			"ディレクターズカット", "未公開映像収録", "復刻", "写真集",
			"デジタル写真集", "ポーズブック", "フォトブック", "電子書籍",
			"パンティ", "キーホルダー", "購入特典", "限定特典", "オンラインサイン会",
			"チェキセット", "チェキ付き", "チェキ付", "ポラロイドセット", "ポラロイド付き", "ポラロイド付",
			"グッズ付き", "グッズ付", "REbecca STARS",
		},
		BlockedTitleRegex: []string{
			`\d+連発`,
			`\d+連射`,
			`\d+時間(?:BOX|ベスト)?`,
			`[1-9]\d*本番`,
			`\d+\s*Full-Length Scenes`,
			`[1-9]\d*人`,
			`[1-9]\d*体`,
			`(?:[3-9]\d{2,}|\d{4,})分`,
			`\b(?:1st|2nd|3rd|4th|5th)?BEST\b`,
		},
		BlockedCoverPatterns: []string{
			"ebook-assets",
			"/e-book/",
		},
		MinActressOmnibusCount:      10,
		MaxDurationMinutesThreshold: 300,
	}
}

func (cfg *FilterConfig) compile() {
	if cfg == nil {
		return
	}
	cfg.compiledTitleRegex = nil
	for _, p := range cfg.BlockedTitleRegex {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			cfg.compiledTitleRegex = append(cfg.compiledTitleRegex, re)
		}
	}

	if len(cfg.BlockedLabels) > 0 {
		var escaped []string
		for _, l := range cfg.BlockedLabels {
			l = strings.TrimSpace(l)
			if l != "" {
				escaped = append(escaped, regexp.QuoteMeta(l))
			}
		}
		if len(escaped) > 0 {
			cfg.compiledLabelRegex, _ = regexp.Compile("(?i)(?:" + strings.Join(escaped, "|") + ")")
		}
	}

	if len(cfg.BlockedSeries) > 0 {
		var escaped []string
		for _, s := range cfg.BlockedSeries {
			s = strings.TrimSpace(s)
			if s != "" {
				escaped = append(escaped, regexp.QuoteMeta(s))
			}
		}
		if len(escaped) > 0 {
			cfg.compiledSeriesRegex, _ = regexp.Compile("(?i)(?:" + strings.Join(escaped, "|") + ")")
		}
	}
}

// GetFilterConfigPath returns the absolute path to the filters.json configuration file.
func GetFilterConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, hErr := os.UserHomeDir()
		if hErr == nil && home != "" {
			configDir = filepath.Join(home, "Library", "Application Support")
		} else {
			configDir = "."
		}
	}
	appDir := filepath.Join(configDir, "r19dev")
	_ = os.MkdirAll(appDir, 0755)
	return filepath.Join(appDir, "filters.json")
}

// LoadDefaultFilterConfig loads the filters.json file from the default location,
// creating it with defaults if it does not exist.
func LoadDefaultFilterConfig() error {
	path := GetFilterConfigPath()
	_, err := LoadFilterConfig(path)
	return err
}

// LoadFilterConfig loads FilterConfig from the specified file path.
// If the file doesn't exist, it creates a default configuration file.
func LoadFilterConfig(filePath string) (*FilterConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultFilterConfig()
			_ = SaveFilterConfig(cfg, filePath)
			return cfg, nil
		}
		return DefaultFilterConfig(), err
	}

	var cfg FilterConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultFilterConfig(), err
	}

	cfg.compile()
	configMu.Lock()
	activeConfig = &cfg
	configMu.Unlock()
	return &cfg, nil
}

// SaveFilterConfig writes the FilterConfig to the specified file path (or default path if omitted)
// and updates the active configuration in memory.
func SaveFilterConfig(cfg *FilterConfig, customPath ...string) error {
	if cfg == nil {
		cfg = DefaultFilterConfig()
	}
	cfg.compile()

	targetPath := GetFilterConfigPath()
	if len(customPath) > 0 && customPath[0] != "" {
		targetPath = customPath[0]
	}

	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return err
	}

	configMu.Lock()
	activeConfig = cfg
	configMu.Unlock()
	return nil
}

// ResetFilterConfig resets the filters.json to factory defaults and reloads it.
func ResetFilterConfig(customPath ...string) (*FilterConfig, error) {
	cfg := DefaultFilterConfig()
	err := SaveFilterConfig(cfg, customPath...)
	return cfg, err
}

// GetActiveFilterConfig returns the current thread-safe active FilterConfig.
func GetActiveFilterConfig() *FilterConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	if activeConfig == nil {
		return DefaultFilterConfig()
	}
	return activeConfig
}

// IsExcluded checks if a given movie metadata matches any of the filter exclusion rules.
func (cfg *FilterConfig) IsExcluded(movieID, title, originalTitle, label, series, coverURL string, genres []string, actressCount ...int) (bool, string) {
	if cfg == nil || !cfg.Enabled {
		return false, ""
	}

	upperID := strings.ToUpper(strings.TrimSpace(movieID))
	allText := strings.TrimSpace(title + " " + originalTitle)
	allLower := strings.ToLower(allText)

	// 1. Photobooks / Digital books / Cover URL patterns
	if strings.HasPrefix(upperID, "B600") || strings.HasPrefix(upperID, "D600") || strings.HasPrefix(upperID, "DG") {
		return true, "Photobook / Digital Book"
	}
	for _, pat := range cfg.BlockedCoverPatterns {
		if pat != "" && strings.Contains(coverURL, pat) {
			return true, "Photobook / Digital Book"
		}
	}
	for _, kw := range []string{"写真集", "デジタル写真集", "ポーズブック", "フォトブック", "電子書籍", "photobook", "photo book"} {
		if strings.Contains(allLower, kw) {
			return true, "Photobook / Digital Book"
		}
	}

	// 2. Promotional SKU prefixes & suffixes
	if defaultPromoSkuPrefixRegex.MatchString(upperID) || defaultPromoSkuSuffixRegex.MatchString(upperID) ||
		strings.HasSuffix(upperID, "-EC") || strings.HasSuffix(upperID, "-T-EC") || strings.HasSuffix(upperID, "EC") {
		return true, "Promotional SKU Variant"
	}

	// 3. Goods bundle markers
	if defaultGoodsBundleRegex.MatchString(allText) || strings.Contains(allLower, "cheki set") || strings.Contains(allLower, "polaroid set") {
		return true, "Promotional Bundle Variant"
	}

	// 4. Variety talk shows
	if strings.HasPrefix(upperID, "KCKC") || strings.HasPrefix(upperID, "MLTN") || strings.HasPrefix(upperID, "BMW") ||
		strings.Contains(allText, "カチコチTV") || strings.Contains(allText, "カチコチ") {
		return true, "Variety / Talk Show"
	}

	// 5. Re-issue Remaster / Director's Cut
	if strings.HasPrefix(upperID, "JQRE") ||
		strings.Contains(allText, "未公開映像収録") ||
		strings.Contains(allText, "ディレクターズカット") ||
		strings.Contains(allText, "AIリマスター") ||
		strings.Contains(allLower, "ai remaster") ||
		strings.Contains(allText, "デジタルリマスター") ||
		strings.Contains(allLower, "digital remaster") ||
		strings.Contains(allText, "リマスター") ||
		strings.Contains(allLower, "remaster") ||
		strings.Contains(allText, "復刻") {
		return true, "Remaster / Director's Cut"
	}

	// 6. Blocked Labels
	if label != "" {
		if cfg.compiledLabelRegex != nil && cfg.compiledLabelRegex.MatchString(label) {
			return true, "Omnibus Compilation"
		}
		for _, l := range cfg.BlockedLabels {
			if l != "" && strings.Contains(strings.ToLower(label), strings.ToLower(l)) {
				return true, "Omnibus Compilation"
			}
		}
	}

	// 7. Blocked Series
	if series != "" {
		if cfg.compiledSeriesRegex != nil && cfg.compiledSeriesRegex.MatchString(series) {
			return true, "Omnibus Compilation"
		}
		for _, s := range cfg.BlockedSeries {
			if s != "" && strings.Contains(strings.ToLower(series), strings.ToLower(s)) {
				return true, "Omnibus Compilation"
			}
		}
	}

	// 8. Blocked Prefixes
	for _, p := range cfg.BlockedPrefixes {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" && (strings.HasPrefix(upperID, p) || strings.Contains(upperID, p)) {
			return true, "Omnibus Compilation"
		}
	}

	// 9. Blocked Genres
	for _, g := range genres {
		gNorm := strings.ToLower(strings.TrimSpace(g))
		if gNorm == "anime" || gNorm == "animation" || gNorm == "game" || gNorm == "comic" || gNorm == "manga" {
			return true, "Non-Video Media"
		}
		if gNorm == "photo book" || gNorm == "digital photo" || gNorm == "collection of photographs" {
			return true, "Photobook / Digital Book"
		}
		if gNorm == "special offers and set products" || strings.Contains(gNorm, "set products") {
			return true, "Promotional Bundle Variant"
		}
		if gNorm == "includes event participation rights" || strings.Contains(gNorm, "event participation") {
			return true, "Event / Autograph Session"
		}
		if gNorm == "compilation" || gNorm == "omnibus" {
			return true, "Omnibus Compilation"
		}
		for _, bg := range cfg.BlockedGenres {
			if strings.EqualFold(gNorm, strings.ToLower(strings.TrimSpace(bg))) || strings.Contains(gNorm, strings.ToLower(strings.TrimSpace(bg))) {
				return true, "Omnibus Compilation"
			}
		}
	}

	// 10. Blocked Title Keywords
	for _, kw := range cfg.BlockedTitleKeywords {
		kw = strings.TrimSpace(kw)
		if kw != "" && (strings.Contains(allText, kw) || strings.Contains(allLower, strings.ToLower(kw))) {
			return true, "Omnibus Compilation"
		}
	}

	// 11. Blocked Title Regex Patterns & Multi-Actress
	// Exemption: Official anniversary crossover works
	isAnniversary := strings.Contains(allText, "周年") ||
		strings.Contains(allLower, "anniversary") ||
		strings.Contains(allText, "記念作品") ||
		strings.Contains(allText, "創立")

	if !isAnniversary {
		for _, re := range cfg.compiledTitleRegex {
			if re.MatchString(allText) {
				return true, "Omnibus Compilation"
			}
		}

		actCount := 0
		if len(actressCount) > 0 {
			actCount = actressCount[0]
		}
		if cfg.MinActressOmnibusCount > 0 && actCount >= cfg.MinActressOmnibusCount {
			return true, "Omnibus Compilation"
		}

		if actCount >= 5 {
			for _, g := range genres {
				if strings.EqualFold(strings.TrimSpace(g), "over 4 hours") {
					return true, "Omnibus Compilation"
				}
			}
			if strings.Contains(allLower, "600min") || strings.Contains(allLower, "480分") || strings.Contains(allLower, "300分") {
				return true, "Omnibus Compilation"
			}
		}
	}

	return false, ""
}

// IsPromotionalOrOmnibusVariant checks if a movie represents a promotional variant, goods bundle,
// non-video item, or multi-actress omnibus compilation using active filter configuration.
// Returns (shouldSkip bool, reason string).
func IsPromotionalOrOmnibusVariant(movieID, title, originalTitle, coverURL string, genres []string, actressCount ...int) (bool, string) {
	return IsPromotionalOrOmnibusVariantWithDetails(movieID, title, originalTitle, "", "", coverURL, genres, actressCount...)
}

// IsPromotionalOrOmnibusVariantWithDetails checks if a movie represents a promotional variant, goods bundle,
// non-video item, or multi-actress omnibus compilation, also checking studio Label and Series names.
// Returns (shouldSkip bool, reason string).
func IsPromotionalOrOmnibusVariantWithDetails(movieID, title, originalTitle, label, series, coverURL string, genres []string, actressCount ...int) (bool, string) {
	cfg := GetActiveFilterConfig()
	return cfg.IsExcluded(movieID, title, originalTitle, label, series, coverURL, genres, actressCount...)
}
