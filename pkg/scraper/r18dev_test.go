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
