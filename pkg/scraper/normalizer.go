package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reHyphenID = regexp.MustCompile(`(?i)^([a-z]{2,6})-?(\d{1,5})$`)
	reFC2ID    = regexp.MustCompile(`(?i)^FC2(?:-PPV)?-(\d+)$`)
)

// NormalizeToCombinedID converts a JAV ID into R18.dev's combined ID format (e.g. MIDA-517 -> mida00517).
func NormalizeToCombinedID(id string) string {
	clean := strings.TrimSpace(id)
	if clean == "" {
		return ""
	}

	// 1. DMM Content ID (h_1472smkcx003)
	if strings.HasPrefix(strings.ToLower(clean), "h_") {
		return strings.ToLower(clean)
	}

	// 2. FC2 (FC2-PPV-1234567 -> fc2-1234567)
	if m := reFC2ID.FindStringSubmatch(clean); len(m) == 2 {
		return "fc2-" + m[1]
	}

	// 3. Hyphenated / Standard JAV ID (MIDA-517, kavr00428, SNOS-028)
	if m := reHyphenID.FindStringSubmatch(clean); len(m) == 3 {
		prefix := strings.ToLower(m[1])
		numStr := m[2]

		num, err := strconv.Atoi(numStr)
		if err == nil {
			// Pad number to 5 digits (R18.dev standard)
			return fmt.Sprintf("%s%05d", prefix, num)
		}
		return prefix + numStr
	}

	return strings.ToLower(clean)
}
