package matcher

import (
	"testing"

	"github.com/dagalp/r19dev-scraper/pkg/scanner"
)

func TestMatcherRealWorldCases(t *testing.T) {
	m, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create matcher: %v", err)
	}

	testCases := []struct {
		filename    string
		expectedID  string
		isMultiPart bool
		partNum     int
	}{
		// 1. Noise prefix cases
		{"hhd800.com@MIDA-517.mp4", "MIDA-517", false, 0},
		{"hhd800.com@SNOS-028.mp4", "SNOS-028", false, 0},
		{"hhd800.com@WAAA-615.mp4", "WAAA-615", false, 0},
		{"hhd800.com@PRWF-010.mp4", "PRWF-010", false, 0},
		{"[4k2.com] IPX-535.mp4", "IPX-535", false, 0},

		// 2. VR 5-digit cases
		{"4k2.com@kavr00428_1_8k.mp4", "KAVR-428", true, 1},
		{"4k2.com@kavr00428_2_8k.mp4", "KAVR-428", true, 2},
		{"4k2.me@kavr00470_1_8k.mp4", "KAVR-470", true, 1},
		{"twojav.com@kavr00495_4_8k.mp4", "KAVR-495", true, 4},
		{"twojav.com@mdvr00391_1_8k.mp4", "MDVR-391", true, 1},
		{"twojav.com@mdvr00406_3_8k.mp4", "MDVR-406", true, 3},
		{"4k2.com@savr00790_1_8k.mp4", "SAVR-790", true, 1},
		{"4k2.com@sivr00394_1_hq.mp4", "SIVR-394", true, 1},
		{"twojav.com@sivr00460_2_8k.mp4", "SIVR-460", true, 2},

		// 3. FC2 & Uncensored
		{"FC2-PPV-1234567.mp4", "FC2-PPV-1234567", false, 0},
		{"020326_001-1PON.mp4", "020326_001-1PON", false, 0},
	}

	for _, tc := range testCases {
		res := m.MatchFile(scanner.FileInfo{Name: tc.filename, Path: "/dummy/" + tc.filename})
		if res == nil {
			t.Errorf("Failed to match: %s", tc.filename)
			continue
		}
		if res.ID != tc.expectedID {
			t.Errorf("For %s: expected ID %s, got %s (raw: %s)", tc.filename, tc.expectedID, res.ID, res.RawID)
		}
		if res.IsMultiPart != tc.isMultiPart {
			t.Errorf("For %s: expected IsMultiPart %v, got %v", tc.filename, tc.isMultiPart, res.IsMultiPart)
		}
		if tc.isMultiPart && res.PartNumber != tc.partNum {
			t.Errorf("For %s: expected PartNumber %d, got %d", tc.filename, tc.partNum, res.PartNumber)
		}
	}
}
