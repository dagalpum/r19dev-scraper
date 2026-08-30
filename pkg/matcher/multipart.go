package matcher

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Matches pt1, PT2, part1, PART2
	reNumericPart = regexp.MustCompile(`(?i)(?:^|[-_.\s])(?:(pt|part))[-_.\s]?(\d{1,2})(?:$|[-_.\s])`)
	// Matches plain numbers at end: -1, _2, .1, _1_8k, _2_hq
	rePlainNumberWithQuality = regexp.MustCompile(`(?i)[-_](\d{1,2})(?:[-_](?:8k|4k|2k|1080p|720p|hq|hd|fhd|uhd|vr|hevc|x264|x265|60fps))?$`)
	reLetterOnlyRemainder    = regexp.MustCompile(`(?i)^\s*[-_.\s]?([a-z])\s*$`)
)

// DetectPart parses the remainder of the filename and returns (partNumber, partSuffix, isMultipart).
func DetectPart(filename, matchedID string) (int, string, bool) {
	lowerName := strings.ToLower(filename)
	lowerID := strings.ToLower(matchedID)

	idx := strings.Index(lowerName, lowerID)
	remainder := filename
	if idx >= 0 {
		remainder = filename[idx+len(matchedID):]
	}

	trimmed := strings.TrimSpace(remainder)
	if trimmed == "" {
		return 0, "", false
	}

	// 1) Explicit pt1/part2
	if m := reNumericPart.FindStringSubmatch(trimmed); len(m) == 3 {
		token := strings.ToLower(m[1])
		if n, err := strconv.Atoi(m[2]); err == nil && n > 0 {
			return n, "-" + token + m[2], true
		}
	}

	// 2) Number with optional quality tag (_1, _2_8k, -1_hq, _4_8k)
	if m := rePlainNumberWithQuality.FindStringSubmatch(trimmed); len(m) >= 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n, "-pt" + m[1], true
		}
	}

	// 3) Single letter part (A/B/C)
	if m := reLetterOnlyRemainder.FindStringSubmatch(trimmed); len(m) == 2 {
		r := strings.ToUpper(m[1])[0]
		if r >= 'A' && r <= 'Z' {
			return int(r - 'A' + 1), "-" + string(r), false
		}
	}

	return 0, "", false
}

// ValidateMultipartInDirectory confirms ambiguous single-letter parts by checking sibling files in the same directory.
func ValidateMultipartInDirectory(results []MatchResult) []MatchResult {
	if len(results) == 0 {
		return results
	}

	type dirIDKey struct {
		dir string
		id  string
	}
	groups := make(map[dirIDKey][]int)
	for i, r := range results {
		key := dirIDKey{dir: filepath.Dir(r.File.Path), id: r.ID}
		groups[key] = append(groups[key], i)
	}

	validated := make([]MatchResult, len(results))
	copy(validated, results)

	for _, indices := range groups {
		if len(indices) >= 2 {
			seenParts := make(map[int]bool)
			for _, idx := range indices {
				if validated[idx].PartNumber > 0 {
					seenParts[validated[idx].PartNumber] = true
				}
			}
			if len(seenParts) >= 2 {
				for _, idx := range indices {
					if validated[idx].PartNumber > 0 {
						validated[idx].IsMultiPart = true
					}
				}
			}
		}
	}

	return validated
}
