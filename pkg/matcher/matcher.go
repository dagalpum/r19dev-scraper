package matcher

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagalp/r19dev-scraper/pkg/scanner"
)

// Common web tracker & domain noise prefixes to strip before matching.
var domainNoiseRegex = regexp.MustCompile(`(?i)^(?:(?:https?://)?(?:www\.)?[\w\.-]+\.(?:com|me|net|org|cc|to|xyz|tv|vip|guru|fun|top|site|link|pw|club|vip)[@_]?|\s*\[[^\]]+\]|\s*\([^\)]+\)|[a-z0-9\._-]+@)\s*`)

// MatchResult represents a matched file and its extracted JAV ID.
type MatchResult struct {
	File        scanner.FileInfo `json:"file"`
	ID          string           `json:"id"`           // Normalized ID (e.g., "MIDA-517", "KAVR-428")
	RawID       string           `json:"raw_id"`       // Raw extracted token (e.g., "kavr00428")
	PartNumber  int              `json:"part_number"`  // 0 = single-part, 1..N = part index
	PartSuffix  string           `json:"part_suffix"`  // "-pt1", "-pt2", "-A"
	IsMultiPart bool             `json:"is_multipart"` // True if identified as multi-part
	MatchedBy   string           `json:"matched_by"`   // Pattern type that matched
}

// Matcher identifies and normalizes JAV IDs from filenames.
type Matcher struct {
	config      *Config
	customRegex *regexp.Regexp

	// Core regex patterns (using boundary-safe groups instead of standard \b to handle underscores)
	reFC2        *regexp.Regexp
	reUncensored *regexp.Regexp
	reHyphenated *regexp.Regexp
	reVR5Digit   *regexp.Regexp
	reNoHyphen   *regexp.Regexp
	reDMMContent *regexp.Regexp
}

// New creates a new Matcher instance with boundary-safe regex patterns.
func New(cfg *Config) (*Matcher, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	m := &Matcher{
		config:       cfg,
		reFC2:        regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])FC2(?:-PPV)?-(\d{5,8})(?:$|[^a-zA-Z0-9])`),
		reUncensored: regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])(\d{6}[-_]\d{2,3}-(?:1PON|10MU|CARIB))(?:$|[^a-zA-Z0-9])`),
		reHyphenated: regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])([A-Za-z]{2,6})-(\d{2,5})(?:[ZE])?(?:$|[^a-zA-Z0-9])`),
		reVR5Digit:   regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])([A-Za-z]{3,5})(\d{5})(?:$|[^a-zA-Z0-9])`),
		reNoHyphen:   regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])([A-Za-z]{3,6})(\d{3,4})(?:$|[^a-zA-Z0-9])`),
		reDMMContent: regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])(h_\d+[a-z]+\d+)(?:$|[^a-zA-Z0-9])`),
	}

	if cfg.CustomRegexEnabled && cfg.CustomRegexPattern != "" {
		re, err := regexp.Compile(cfg.CustomRegexPattern)
		if err != nil {
			return nil, err
		}
		m.customRegex = re
	}

	return m, nil
}

// Match processes a slice of files and returns match results.
func (m *Matcher) Match(files []scanner.FileInfo) []MatchResult {
	results := make([]MatchResult, 0, len(files))
	for _, file := range files {
		if res := m.MatchFile(file); res != nil {
			results = append(results, *res)
		}
	}
	return ValidateMultipartInDirectory(results)
}

// MatchFile attempts to extract a JAV ID from a single file.
func (m *Matcher) MatchFile(file scanner.FileInfo) *MatchResult {
	baseName := filepath.Base(file.Name)
	ext := file.Extension
	if ext == "" {
		ext = filepath.Ext(baseName)
	}
	cleanName := strings.TrimSuffix(baseName, ext)

	// Pre-strip web tracker/domain noise if enabled
	if m.config.StripNoisePrefixes {
		cleanName = domainNoiseRegex.ReplaceAllString(cleanName, "")
	}

	// 1. Custom Regex
	if m.customRegex != nil {
		if sm := m.customRegex.FindStringSubmatch(cleanName); len(sm) > 1 {
			id := strings.ToUpper(strings.TrimSpace(sm[1]))
			partNum, partSuf, isMulti := DetectPart(cleanName, sm[1])
			return &MatchResult{
				File:        file,
				ID:          id,
				RawID:       sm[1],
				PartNumber:  partNum,
				PartSuffix:  partSuf,
				IsMultiPart: isMulti,
				MatchedBy:   "custom_regex",
			}
		}
	}

	// 2. FC2 Pattern: FC2-PPV-1234567 -> FC2-PPV-1234567
	if sm := m.reFC2.FindStringSubmatch(cleanName); len(sm) >= 2 {
		id := "FC2-PPV-" + sm[1]
		partNum, partSuf, isMulti := DetectPart(cleanName, sm[1])
		return &MatchResult{
			File:        file,
			ID:          id,
			RawID:       "FC2-" + sm[1],
			PartNumber:  partNum,
			PartSuffix:  partSuf,
			IsMultiPart: isMulti,
			MatchedBy:   "fc2",
		}
	}

	// 3. Uncensored Date-based Pattern: 020326_001-1PON
	if sm := m.reUncensored.FindStringSubmatch(cleanName); len(sm) >= 2 {
		id := strings.ToUpper(sm[1])
		partNum, partSuf, isMulti := DetectPart(cleanName, sm[1])
		return &MatchResult{
			File:        file,
			ID:          id,
			RawID:       sm[1],
			PartNumber:  partNum,
			PartSuffix:  partSuf,
			IsMultiPart: isMulti,
			MatchedBy:   "uncensored",
		}
	}

	// 4. Standard Hyphenated JAV ID: MIDA-517, SNOS-028, WAAA-615, KAVR-428
	if sm := m.reHyphenated.FindStringSubmatch(cleanName); len(sm) >= 3 {
		prefix := strings.ToUpper(sm[1])
		num := sm[2]
		id := prefix + "-" + num
		rawMatched := sm[1] + "-" + sm[2]
		partNum, partSuf, isMulti := DetectPart(cleanName, rawMatched)
		return &MatchResult{
			File:        file,
			ID:          id,
			RawID:       rawMatched,
			PartNumber:  partNum,
			PartSuffix:  partSuf,
			IsMultiPart: isMulti,
			MatchedBy:   "standard_hyphen",
		}
	}

	// 5. VR 5-digit Content ID: kavr00428 -> KAVR-428, sivr00394 -> SIVR-394
	if sm := m.reVR5Digit.FindStringSubmatch(cleanName); len(sm) >= 3 {
		prefix := strings.ToUpper(sm[1])
		rawNum := sm[2]
		rawMatched := sm[1] + sm[2]
		numInt, err := strconv.Atoi(rawNum)
		displayNum := rawNum
		if err == nil {
			displayNum = strconv.Itoa(numInt)
			if len(displayNum) < 3 {
				displayNum = fmtSprintf3Digits(numInt)
			}
		}
		id := prefix + "-" + displayNum
		partNum, partSuf, isMulti := DetectPart(cleanName, rawMatched)
		return &MatchResult{
			File:        file,
			ID:          id,
			RawID:       rawMatched,
			PartNumber:  partNum,
			PartSuffix:  partSuf,
			IsMultiPart: isMulti,
			MatchedBy:   "vr_5digit",
		}
	}

	// 6. Standard No-Hyphen: SNOS028 -> SNOS-028
	if sm := m.reNoHyphen.FindStringSubmatch(cleanName); len(sm) >= 3 {
		prefix := strings.ToUpper(sm[1])
		num := sm[2]
		rawMatched := sm[1] + sm[2]
		id := prefix + "-" + num
		partNum, partSuf, isMulti := DetectPart(cleanName, rawMatched)
		return &MatchResult{
			File:        file,
			ID:          id,
			RawID:       rawMatched,
			PartNumber:  partNum,
			PartSuffix:  partSuf,
			IsMultiPart: isMulti,
			MatchedBy:   "no_hyphen",
		}
	}

	// 7. DMM Content ID: h_1472smkcx003
	if sm := m.reDMMContent.FindStringSubmatch(cleanName); len(sm) >= 2 {
		id := sm[1]
		partNum, partSuf, isMulti := DetectPart(cleanName, sm[1])
		return &MatchResult{
			File:        file,
			ID:          id,
			RawID:       sm[1],
			PartNumber:  partNum,
			PartSuffix:  partSuf,
			IsMultiPart: isMulti,
			MatchedBy:   "dmm_content_id",
		}
	}

	return nil
}

func fmtSprintf3Digits(n int) string {
	if n < 10 {
		return "00" + strconv.Itoa(n)
	}
	if n < 100 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
