package scraper

import (
	"testing"
)

func TestNormalizeToCombinedID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"MIDA-517", "mida00517"},
		{"SNOS-028", "snos00028"},
		{"WAAA-615", "waaa00615"},
		{"kavr00428", "kavr00428"},
		{"KAVR-428", "kavr00428"},
		{"SIVR-394", "sivr00394"},
		{"FC2-PPV-1234567", "fc2-1234567"},
		{"h_1472smkcx003", "h_1472smkcx003"},
	}

	for _, tc := range testCases {
		res := NormalizeToCombinedID(tc.input)
		if res != tc.expected {
			t.Errorf("NormalizeToCombinedID(%s) = %s; expected %s", tc.input, res, tc.expected)
		}
	}
}

func TestNormalizeToCanonicalID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"1dldss00559", "DLDSS-559"},
		{"1dldss00545", "DLDSS-545"},
		{"n_1544prian048", "PRIAN-048"},
		{"13dsvr01866", "DSVR-1866"},
		{"sone00682", "SONE-682"},
		{"MIDA-517", "MIDA-517"},
		{"snos-38", "SNOS-038"},
		{"FC2-PPV-1234567", "FC2-PPV-1234567"},
	}

	for _, tc := range testCases {
		res := NormalizeToCanonicalID(tc.input)
		if res != tc.expected {
			t.Errorf("NormalizeToCanonicalID(%s) = %s; expected %s", tc.input, res, tc.expected)
		}
	}
}

func TestCandidateCombinedIDs(t *testing.T) {
	candsABP := CandidateCombinedIDs("ABP-966")
	found118 := false
	for _, c := range candsABP {
		if c == "118abp00966" {
			found118 = true
			break
		}
	}
	if !found118 {
		t.Errorf("expected 118abp00966 in candidates for ABP-966, got %v", candsABP)
	}

	candsDLDSS := CandidateCombinedIDs("DLDSS-559")
	found1 := false
	for _, c := range candsDLDSS {
		if c == "1dldss00559" {
			found1 = true
			break
		}
	}
	if !found1 {
		t.Errorf("expected 1dldss00559 in candidates for DLDSS-559, got %v", candsDLDSS)
	}
}

func TestGenerateDMMSampleURLs(t *testing.T) {
	// 1. From coverURL
	cover := "https://pics.dmm.co.jp/digital/video/fway00103/fway00103pl.jpg"
	urls := GenerateDMMSampleURLs("FWAY-103", cover, 5)
	if len(urls) != 5 {
		t.Fatalf("expected 5 URLs, got %d", len(urls))
	}
	expectedFirst := "https://pics.dmm.co.jp/digital/video/fway00103/fway00103jp-1.jpg"
	if urls[0] != expectedFirst {
		t.Errorf("expected %s, got %s", expectedFirst, urls[0])
	}
	expectedFifth := "https://pics.dmm.co.jp/digital/video/fway00103/fway00103jp-5.jpg"
	if urls[4] != expectedFifth {
		t.Errorf("expected %s, got %s", expectedFifth, urls[4])
	}

	// 2. From contentID alone
	urls2 := GenerateDMMSampleURLs("mida00688", "", 10)
	if len(urls2) != 10 {
		t.Fatalf("expected 10 URLs, got %d", len(urls2))
	}
	if urls2[0] != "https://pics.dmm.co.jp/digital/video/mida00688/mida00688jp-1.jpg" {
		t.Errorf("unexpected URL: %s", urls2[0])
	}

	// 3. From standard hyphen JAV ID
	urls3 := GenerateDMMSampleURLs("MIDA-688", "", 3)
	if len(urls3) != 3 {
		t.Fatalf("expected 3 URLs, got %d", len(urls3))
	}
	if urls3[0] != "https://pics.dmm.co.jp/digital/video/mida00688/mida00688jp-1.jpg" {
		t.Errorf("unexpected URL: %s", urls3[0])
	}
}

