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

// CandidateCombinedIDs generates all possible R18.dev / DMM combined content IDs for a given JAV ID.
func CandidateCombinedIDs(id string) []string {
	clean := strings.TrimSpace(id)
	if clean == "" {
		return nil
	}

	var candidates []string
	seen := make(map[string]bool)
	add := func(c string) {
		c = strings.ToLower(strings.TrimSpace(c))
		if c != "" && !seen[c] {
			seen[c] = true
			candidates = append(candidates, c)
		}
	}

	// 1. DMM Content ID (h_1472smkcx003)
	if strings.HasPrefix(strings.ToLower(clean), "h_") || strings.HasPrefix(strings.ToLower(clean), "n_") {
		add(clean)
		return candidates
	}

	// 2. FC2
	if m := reFC2ID.FindStringSubmatch(clean); len(m) == 2 {
		add("fc2-" + m[1])
		return candidates
	}

	// 3. Standard Hyphenated JAV ID (e.g. ABP-966, DLDSS-559, PRED-224, DSVR-1866)
	if m := reHyphenID.FindStringSubmatch(clean); len(m) == 3 {
		prefix := strings.ToUpper(m[1])
		pLower := strings.ToLower(m[1])
		numStr := m[2]
		num, err := strconv.Atoi(numStr)

		if err == nil {
			n5 := fmt.Sprintf("%05d", num)
			n3 := fmt.Sprintf("%03d", num)
			if num >= 1000 {
				n3 = strconv.Itoa(num)
			}

			// Studio specific known maker prefixes:
			switch prefix {
			case "ABP", "ABW", "ABF", "ABS", "CHN", "DOC", "ONEZ", "MIA", "MBM", "HND", "EZD", "SGA", "DSA", "FVE", "KAV", "WAT", "SUK", "MGB", "SIV", "PPT", "BGN", "KAWD", "MAS", "SHKD", "INX", "BOKD", "TBD", "MBD", "DFDM", "DOKS", "UMD", "WFR", "KNMD":
				// Prestige group (prefixed by 118 or h_118)
				add(fmt.Sprintf("118%s%s", pLower, n5))
				add(fmt.Sprintf("118%s%s", pLower, n3))
				add(fmt.Sprintf("h_118%s%s", pLower, n5))
			case "DLDSS", "SDDE", "SDJS", "SDMM", "SDMU", "SDMF", "SDAB", "SDSR", "SDSI", "SDNM", "SDAM", "MIST", "KMHR", "STAR", "STARS":
				// SOD group (prefixed by 1)
				add(fmt.Sprintf("1%s%s", pLower, n5))
				add(fmt.Sprintf("1%s%s", pLower, n3))
			case "DSVR":
				add(fmt.Sprintf("13%s%s", pLower, n5))
			}

			// Standard 5-digit & 3-digit forms
			add(fmt.Sprintf("%s%s", pLower, n5))
			add(fmt.Sprintf("%s%s", pLower, n3))
			add(fmt.Sprintf("118%s%s", pLower, n5))
			add(fmt.Sprintf("1%s%s", pLower, n5))
			add(fmt.Sprintf("h_118%s%s", pLower, n5))
			add(fmt.Sprintf("h_068%s%s", pLower, n5))
			add(fmt.Sprintf("24id%s%s", pLower, n5))
			return candidates
		}
	}

	add(clean)
	return candidates
}

// NormalizeToCombinedID converts a JAV ID into R18.dev's combined ID format (e.g. MIDA-517 -> mida00517).
func NormalizeToCombinedID(id string) string {
	cands := CandidateCombinedIDs(id)
	if len(cands) > 0 {
		return cands[0]
	}
	return strings.ToLower(strings.TrimSpace(id))
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
