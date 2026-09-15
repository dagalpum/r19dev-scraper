package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reHyphenID     = regexp.MustCompile(`(?i)^([a-z]{2,6})-?(\d{1,5})$`)
	reFC2ID        = regexp.MustCompile(`(?i)^FC2(?:-PPV)?-(\d+)$`)
	reRawContentID = regexp.MustCompile(`(?i)^(?:[a-z]{1,2}_\d{1,4}|\d{1,3})?([a-z]{2,6})(\d{3,5})(?:tk\d*|ec)?$`)
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

// NormalizeToCanonicalID converts any raw content ID or JAV ID into standard uppercase canonical format (e.g. 1dldss00559 -> DLDSS-559, n_1544prian048 -> PRIAN-048).
func NormalizeToCanonicalID(id string) string {
	clean := strings.TrimSpace(id)
	if clean == "" {
		return ""
	}

	// 1. FC2
	if m := reFC2ID.FindStringSubmatch(clean); len(m) == 2 {
		return "FC2-PPV-" + m[1]
	}

	// 2. Already hyphenated Standard ID (e.g. MIDA-517, SNOS-028)
	if m := reHyphenID.FindStringSubmatch(clean); len(m) == 3 {
		prefix := strings.ToUpper(m[1])
		num, err := strconv.Atoi(m[2])
		if err == nil {
			if num < 1000 {
				return fmt.Sprintf("%s-%03d", prefix, num)
			}
			return fmt.Sprintf("%s-%d", prefix, num)
		}
		return prefix + "-" + m[2]
	}

	// 3. Raw DMM Content ID with prefix numbers or letters (e.g. 1dldss00559, n_1544prian048, 13dsvr01866, sone00682)
	if m := reRawContentID.FindStringSubmatch(clean); len(m) == 3 {
		prefix := strings.ToUpper(m[1])
		num, err := strconv.Atoi(m[2])
		if err == nil {
			if num < 1000 {
				return fmt.Sprintf("%s-%03d", prefix, num)
			}
			return fmt.Sprintf("%s-%d", prefix, num)
		}
	}

	return strings.ToUpper(clean)
}
