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

