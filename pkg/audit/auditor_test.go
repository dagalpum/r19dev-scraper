package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectFolder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev-audit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	movieDir := filepath.Join(tempDir, "Asuka Aida", "PPPD-710 Beautiful Soapland Girl")
	if err := os.MkdirAll(movieDir, 0o755); err != nil {
		t.Fatalf("failed to create movie dir: %v", err)
	}

	// 1. Initially empty folder
	auditor, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create auditor: %v", err)
	}

	audit, err := auditor.InspectFolder(context.Background(), movieDir, tempDir)
	if err != nil {
		t.Fatalf("unexpected error inspecting folder: %v", err)
	}
	if audit.MovieID != "PPPD-710" {
		t.Errorf("expected MovieID PPPD-710, got %s", audit.MovieID)
	}
	if audit.ActressName != "Asuka Aida" {
		t.Errorf("expected ActressName Asuka Aida, got %s", audit.ActressName)
	}
	if audit.Status != StatusNoVideo {
		t.Errorf("expected StatusNoVideo, got %s", audit.Status)
	}

	// 2. Add video file
	videoFile := filepath.Join(movieDir, "PPPD-710.mp4")
	if err := os.WriteFile(videoFile, []byte("fake-video-bytes"), 0o644); err != nil {
		t.Fatalf("failed to write video file: %v", err)
	}

	audit, err = auditor.InspectFolder(context.Background(), movieDir, tempDir)
	if err != nil {
		t.Fatalf("unexpected error inspecting folder: %v", err)
	}
	if !audit.HasVideo {
		t.Errorf("expected HasVideo to be true")
	}
	if audit.Status != StatusIncomplete {
		t.Errorf("expected StatusIncomplete, got %s", audit.Status)
	}
	if len(audit.MissingItems) != 5 { // Missing NFO, HTML, Poster, Fanart, Screenshots
		t.Errorf("expected 5 missing items, got %v", audit.MissingItems)
	}

	// 3. Add all required standard assets
	_ = os.WriteFile(filepath.Join(movieDir, "PPPD-710.nfo"), []byte("<movie></movie>"), 0o644)
	_ = os.WriteFile(filepath.Join(movieDir, "movie.html"), []byte("<html></html>"), 0o644)
	_ = os.WriteFile(filepath.Join(movieDir, "poster.jpg"), []byte("jpg"), 0o644)
	_ = os.WriteFile(filepath.Join(movieDir, "fanart.jpg"), []byte("jpg"), 0o644)
	extraDir := filepath.Join(movieDir, "extrafanart")
	_ = os.MkdirAll(extraDir, 0o755)
	_ = os.WriteFile(filepath.Join(extraDir, "fanart1.jpg"), []byte("jpg"), 0o644)

	audit, err = auditor.InspectFolder(context.Background(), movieDir, tempDir)
	if err != nil {
		t.Fatalf("unexpected error inspecting folder: %v", err)
	}
	if audit.Status != StatusComplete {
		t.Errorf("expected StatusComplete, got %s (missing: %v)", audit.Status, audit.MissingItems)
	}
	if !audit.HasNFO || !audit.HasHTML || !audit.HasPoster || !audit.HasFanart || !audit.HasScreenshots {
		t.Errorf("expected all asset flags to be true")
	}
	if audit.ScreenshotsCount != 1 {
		t.Errorf("expected ScreenshotsCount 1, got %d", audit.ScreenshotsCount)
	}
}

func TestAuditDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev-audit-dir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create 2 movie folders
	m1 := filepath.Join(tempDir, "Actress1", "MIDA-517 Title One")
	m2 := filepath.Join(tempDir, "Actress2", "IPX-123 Title Two")
	_ = os.MkdirAll(m1, 0o755)
	_ = os.MkdirAll(m2, 0o755)
	_ = os.WriteFile(filepath.Join(m1, "MIDA-517.mp4"), []byte("v1"), 0o644)
	_ = os.WriteFile(filepath.Join(m2, "IPX-123.mp4"), []byte("v2"), 0o644)

	auditor, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create auditor: %v", err)
	}

	results, err := auditor.AuditDirectory(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("unexpected error auditing directory: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 audited movies, got %d", len(results))
	}
	if results[0].MovieID != "MIDA-517" || results[1].MovieID != "IPX-123" {
		t.Errorf("expected sorted Actress1 (MIDA-517) and Actress2 (IPX-123), got %s and %s", results[0].MovieID, results[1].MovieID)
	}
}

func TestZeroByteAndCorruptDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev-audit-corrupt-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	movieDir := filepath.Join(tempDir, "Actress", "PRED-123 Broken Assets")
	_ = os.MkdirAll(movieDir, 0o755)

	// Create valid video file
	_ = os.WriteFile(filepath.Join(movieDir, "PRED-123.mp4"), []byte("video data"), 0o644)
	// Create 0-byte poster and fanart
	_ = os.WriteFile(filepath.Join(movieDir, "poster.jpg"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(movieDir, "fanart.jpg"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(movieDir, "PRED-123.nfo"), []byte(""), 0o644)

	auditor, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create auditor: %v", err)
	}

	audit, err := auditor.InspectFolder(context.Background(), movieDir, tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 0-byte files should NOT be marked as HasPoster / HasFanart / HasNFO
	if audit.HasPoster {
		t.Errorf("expected HasPoster=false for 0-byte file")
	}
	if audit.HasFanart {
		t.Errorf("expected HasFanart=false for 0-byte file")
	}
	if audit.HasNFO {
		t.Errorf("expected HasNFO=false for 0-byte file")
	}
	if len(audit.CorruptedFiles) < 3 {
		t.Errorf("expected at least 3 corrupted files recorded, got %d", len(audit.CorruptedFiles))
	}
}

func TestMultipartSequenceCheck(t *testing.T) {
	// Case 1: Continuous sequence CD1, CD2, CD3
	pathsOK := []string{
		"/path/KAVR-403-cd1.mp4",
		"/path/KAVR-403-cd2.mp4",
		"/path/KAVR-403-cd3.mp4",
	}
	isMp, missing, warns := checkMultipartSequence(pathsOK)
	if !isMp || len(missing) > 0 || len(warns) > 0 {
		t.Errorf("expected continuous sequence without missing parts, got isMp=%v missing=%v warns=%v", isMp, missing, warns)
	}

	// Case 2: Gap in sequence CD1, CD3 (missing CD2)
	pathsGap := []string{
		"/path/KAVR-403-cd1.mp4",
		"/path/KAVR-403-cd3.mp4",
	}
	isMp2, missing2, warns2 := checkMultipartSequence(pathsGap)
	if !isMp2 || len(missing2) != 1 || missing2[0] != 2 {
		t.Errorf("expected missing part 2, got %v (warns: %v)", missing2, warns2)
	}
}

func TestFolderNamingAnomaly(t *testing.T) {
	// Unbalanced bracket: KAVR-403 VR] ...
	warns := checkFolderNaming("KAVR-403 VR] Two Lonely Seniors")
	if len(warns) == 0 {
		t.Errorf("expected warning for unbalanced brackets 'VR]', got none")
	}

	// Balanced brackets: KAVR-403 [VR] ...
	warnsClean := checkFolderNaming("KAVR-403 [VR] Two Lonely Seniors")
	if len(warnsClean) > 0 {
		t.Errorf("expected no warnings for balanced brackets, got %v", warnsClean)
	}
}
