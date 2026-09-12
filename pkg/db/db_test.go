package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

func TestDBOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_db_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	// 1. Test Follow / Unfollow Actress
	if err := d.FollowActress("Kanna Seto", "瀬戸環奈", "https://example.com/kanna.jpg"); err != nil {
		t.Fatalf("FollowActress failed: %v", err)
	}

	followed, err := d.IsActressFollowed("kanna seto") // Case-insensitive
	if err != nil || !followed {
		t.Errorf("IsActressFollowed expected true, got %v (err: %v)", followed, err)
	}

	actresses, err := d.ListFollowedActresses()
	if err != nil || len(actresses) != 1 || actresses[0].Name != "Kanna Seto" {
		t.Errorf("ListFollowedActresses mismatch: %+v", actresses)
	}

	// 2. Test Save & Get Movie
	movie := &scraper.Movie{
		ID:             "SNOS-038",
		CombinedID:     "snos00038",
		Title:          "Sample Movie",
		Maker:          "S1",
		ReleaseDate:    "2026-01-09",
		RuntimeMinutes: 120,
		Actresses: []scraper.Actress{
			{Name: "Kanna Seto", JaName: "瀬戸環奈"},
		},
		Genres:            []string{"Beautiful Girl", "Hi-Def"},
		SampleScreenshots: []string{"https://example.com/1.jpg"},
		ScrapedAt:         time.Now(),
	}

	if err := d.SaveMovie(movie); err != nil {
		t.Fatalf("SaveMovie failed: %v", err)
	}

	saved, err := d.GetMovie("snos-038")
	if err != nil || saved == nil || saved.Title != movie.Title {
		t.Errorf("GetMovie failed: %+v (err: %v)", saved, err)
	}

	// 3. Test User State: Watched, Rating, Favorite
	if _, err := d.ToggleWatched("SNOS-038"); err != nil {
		t.Fatalf("ToggleWatched failed: %v", err)
	}
	if err := d.SetRating("SNOS-038", 5); err != nil {
		t.Fatalf("SetRating failed: %v", err)
	}
	if _, err := d.ToggleFavorite("SNOS-038"); err != nil {
		t.Fatalf("ToggleFavorite failed: %v", err)
	}

	st, err := d.GetUserState("SNOS-038")
	if err != nil || st == nil || !st.IsWatched || st.UserRating != 5 || !st.IsFavorite {
		t.Errorf("UserState mismatch: %+v", st)
	}

	// 4. Test Library Files
	rec := LibraryFileRecord{
		FilePath:  "/nas/SNOS-038.mp4",
		MovieID:   "SNOS-038",
		SizeBytes: 1024 * 1024 * 1024,
	}
	if err := d.UpsertLibraryFile(rec); err != nil {
		t.Fatalf("UpsertLibraryFile failed: %v", err)
	}

	inLib, err := d.HasMovieInLibrary("SNOS-038")
	if err != nil || !inLib {
		t.Errorf("HasMovieInLibrary expected true, got %v", inLib)
	}

	// 5. Test Organized
	if err := d.SetOrganized("SNOS-038", "/nas/target/SNOS-038", "/nas/target/SNOS-038/SNOS-038.mp4"); err != nil {
		t.Fatalf("SetOrganized failed: %v", err)
	}
	orgMap, err := d.GetOrganizedMap()
	if err != nil || !orgMap["SNOS-038"] {
		t.Errorf("GetOrganizedMap expected SNOS-038 true, got %v", orgMap)
	}
	isOrg, err := d.IsOrganized("SNOS-038")
	if err != nil || !isOrg {
		t.Errorf("IsOrganized expected true, got %v", isOrg)
	}

	// 6. Test Operation History
	histID, err := d.AddOperationHistory("organize", "/nas/target", 10, 8, 2, false, "Organize finished.\n[MOVED] SNOS-038")
	if err != nil || histID == 0 {
		t.Fatalf("AddOperationHistory failed: %v", err)
	}

	histList, err := d.GetOperationHistory(10, false)
	if err != nil || len(histList) == 0 {
		t.Fatalf("GetOperationHistory failed: %v", err)
	}
	if histList[0].Operation != "organize" || histList[0].SuccessCount != 8 {
		t.Errorf("OperationHistory mismatch: %+v", histList[0])
	}

	detail, err := d.GetOperationDetail(histID)
	if err != nil || detail == nil || detail.LogText == "" {
		t.Errorf("GetOperationDetail failed: %v, detail: %+v", err, detail)
	}

	if err := d.ClearOperationHistory(); err != nil {
		t.Fatalf("ClearOperationHistory failed: %v", err)
	}
	emptyList, _ := d.GetOperationHistory(10, false)
	if len(emptyList) != 0 {
		t.Errorf("Expected 0 history records after clear, got %d", len(emptyList))
	}

	// 7. Test DB Path and BackupTo
	if d.Path() != dbPath {
		t.Errorf("Expected db.Path() to be %s, got %s", dbPath, d.Path())
	}

	backupFile := filepath.Join(tempDir, "backup_test.db")
	if err := d.BackupTo(backupFile); err != nil {
		t.Fatalf("BackupTo failed: %v", err)
	}

	if _, err := os.Stat(backupFile); err != nil {
		t.Fatalf("Expected backup file to exist at %s: %v", backupFile, err)
	}

	// Verify backup can be opened and contains data
	backupDB, err := Open(backupFile)
	if err != nil {
		t.Fatalf("Failed to open backup database: %v", err)
	}
	defer backupDB.Close()

	backupMovie, err := backupDB.GetMovie("snos-038")
	if err != nil || backupMovie == nil || backupMovie.Title != "Sample Movie" {
		t.Errorf("Backup database corrupted or missing data: %+v (err: %v)", backupMovie, err)
	}

	// 8. Test Latest Activity Time
	latestTime, err := d.GetLatestActivityTime()
	if err != nil {
		t.Fatalf("GetLatestActivityTime failed: %v", err)
	}
	if latestTime.IsZero() {
		t.Errorf("Expected non-zero latest activity time")
	}

	inspectedTime, err := InspectLatestActivityTime(backupFile)
	if err != nil {
		t.Fatalf("InspectLatestActivityTime failed: %v", err)
	}
	if inspectedTime.IsZero() {
		t.Errorf("Expected non-zero inspected activity time")
	}
}

func TestSyncWithBackupCandidates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "r19dev_sync_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	localPath := filepath.Join(tempDir, "local.db")
	backupPath := filepath.Join(tempDir, "nas_backup.db")

	// 1. Create a mock NAS backup DB with an operation record
	nasDB, err := Open(backupPath)
	if err != nil {
		t.Fatalf("failed to create nasDB: %v", err)
	}
	_, err = nasDB.AddOperationHistory("organize", "/dest", 5, 5, 0, false, "Initial NAS organize")
	if err != nil {
		t.Fatalf("failed to add operation to nasDB: %v", err)
	}
	_ = nasDB.Close()

	// Scenario 1: Local DB does not exist -> Should restore from NAS backup
	res := SyncWithBackupCandidates(localPath, []string{backupPath})
	if res.Action != SyncActionRestored {
		t.Errorf("Expected SyncActionRestored, got %v (%s)", res.Action, res.Message)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("Expected restored local DB to exist: %v", err)
	}

	// Verify restored DB content
	localDB, err := Open(localPath)
	if err != nil {
		t.Fatalf("failed to open restored localDB: %v", err)
	}
	hist, _ := localDB.GetOperationHistory(10, false)
	if len(hist) != 1 || hist[0].Operation != "organize" {
		t.Errorf("Restored DB missing expected history: %+v", hist)
	}

	// Scenario 2: Local DB has newer activity -> Should NOT sync from older NAS backup
	time.Sleep(10 * time.Millisecond) // Ensure timestamp strictly advances
	_, err = localDB.ToggleWatched("NEW-001")
	if err != nil {
		t.Fatalf("failed to update localDB user state: %v", err)
	}
	_ = localDB.Close()

	res2 := SyncWithBackupCandidates(localPath, []string{backupPath})
	if res2.Action != SyncActionLocalNewer {
		t.Errorf("Expected SyncActionLocalNewer, got %v (%s)", res2.Action, res2.Message)
	}

	// Scenario 3: NAS backup gets newer activity -> Should sync to local and create .bak
	time.Sleep(10 * time.Millisecond)
	nasDB2, err := Open(backupPath)
	if err != nil {
		t.Fatalf("failed to open nasDB2: %v", err)
	}
	_, err = nasDB2.AddOperationHistory("organize", "/dest2", 10, 10, 0, false, "Newer NAS run")
	if err != nil {
		t.Fatalf("failed to add operation to nasDB2: %v", err)
	}
	_ = nasDB2.Close()

	res3 := SyncWithBackupCandidates(localPath, []string{backupPath})
	if res3.Action != SyncActionSynced {
		t.Errorf("Expected SyncActionSynced, got %v (%s)", res3.Action, res3.Message)
	}

	// Verify local.db.bak was created
	if _, err := os.Stat(localPath + ".bak"); err != nil {
		t.Errorf("Expected local backup %s.bak to exist: %v", localPath, err)
	}

	// Verify local DB now has newer NAS history (2 operations)
	localDB2, err := Open(localPath)
	if err != nil {
		t.Fatalf("failed to open synced localDB: %v", err)
	}
	defer localDB2.Close()
	hist2, _ := localDB2.GetOperationHistory(10, false)
	if len(hist2) != 2 {
		t.Errorf("Expected 2 history records after sync, got %d", len(hist2))
	}
}



