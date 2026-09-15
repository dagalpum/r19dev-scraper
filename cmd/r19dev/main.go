package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagalp/r19dev-scraper/pkg/actress"
	"github.com/dagalp/r19dev-scraper/pkg/cache"
	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/migrator"
	"github.com/dagalp/r19dev-scraper/pkg/organizer"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
	"github.com/dagalp/r19dev-scraper/pkg/tui"
	"github.com/dagalp/r19dev-scraper/pkg/web"
)

const version = "2.0.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runWeb(".", 8080, true)
		return
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "web", "serve", "ui":
		target := "."
		port := 8080
		openBrowser := true
		for i := 1; i < len(args); i++ {
			if (args[i] == "--port" || args[i] == "-p") && i+1 < len(args) {
				var p int
				if _, err := fmt.Sscanf(args[i+1], "%d", &p); err == nil && p > 0 {
					port = p
				}
				i++
			} else if args[i] == "--no-open" {
				openBrowser = false
			} else if !strings.HasPrefix(args[i], "-") {
				target = args[i]
			}
		}
		runWeb(target, port, openBrowser)

	case "tui":
		target := "."
		lang := "en"
		proto := tui.ProtocolAuto
		for i := 1; i < len(args); i++ {
			if args[i] == "--lang" && i+1 < len(args) {
				lang = args[i+1]
				i++
			} else if (args[i] == "--proto" || args[i] == "-p") && i+1 < len(args) {
				proto = tui.GraphicProtocol(strings.ToLower(args[i+1]))
				i++
			} else {
				target = args[i]
			}
		}
		runTUI(target, lang, proto)

	case "scan":
		target := "."
		jsonOutput := false
		for _, a := range args[1:] {
			if a == "--json" || a == "-j" {
				jsonOutput = true
			} else {
				target = a
			}
		}
		runScan(target, jsonOutput)

	case "scrape":
		if len(args) < 2 {
			fmt.Println("Usage: r19dev scrape <JAV-ID> [--lang en|ja]")
			os.Exit(1)
		}
		id := args[1]
		lang := "en"
		for i := 2; i < len(args); i++ {
			if args[i] == "--lang" && i+1 < len(args) {
				lang = args[i+1]
				i++
			}
		}
		runScrape(id, lang)

	case "actress":
		runActress(args[1:])

	case "organize":
		runOrganize(args[1:])

	case "migrate":
		runMigrate(args[1:])

	case "upgrade-html":
		runUpgradeHTML(args[1:])

	case "backup":
		runBackup(args[1:])

	case "filters", "filter":
		runFilters(args[1:])

	case "cache-clear", "clear-cache":
		if err := cache.Default().Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to clear cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🧹 Cache cleared successfully (%s)\n", cache.Default().RootDir())

	case "-v", "--version", "version":
		fmt.Printf("r19dev-scraper version %s\n", version)

	case "-h", "--help", "help":
		printHelp()

	default:
		if stat, err := os.Stat(args[0]); err == nil && stat.IsDir() {
			runWeb(args[0], 8080, true)
			return
		}
		printHelp()
	}
}

func runWeb(targetDir string, port int, openBrowser bool) {
	srv, err := web.NewServer(web.Config{
		TargetDir: targetDir,
		Port:      port,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize Web server: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Start(openBrowser); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Web server error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(targetDir, lang string, proto tui.GraphicProtocol) {
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid path: %v\n", err)
		os.Exit(1)
	}

	model, err := tui.New(absPath, lang, proto)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize TUI: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runScan(targetDir string, jsonOutput bool) {
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid path: %v\n", err)
		os.Exit(1)
	}

	sc := scanner.New(scanner.DefaultConfig())
	res, err := sc.Scan(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
		os.Exit(1)
	}

	mc, err := matcher.New(matcher.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Matcher error: %v\n", err)
		os.Exit(1)
	}

	matches := mc.Match(res.Files)

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(matches)
		return
	}

	fmt.Printf("=== Scan & Match Results for: %s ===\n", absPath)
	fmt.Printf("Total Files: %d | Matched: %d\n\n", len(res.Files), len(matches))
	for _, m := range matches {
		partInfo := "Single"
		if m.IsMultiPart {
			partInfo = fmt.Sprintf("Pt %d (%s)", m.PartNumber, m.PartSuffix)
		}
		fmt.Printf("▶ [%s] %-28s -> JAV ID: %-12s (%s, %.1f MB)\n",
			m.MatchedBy, filepath.Base(m.File.Name), m.ID, partInfo, m.File.SizeMB())
	}
}

func runScrape(id, lang string) {
	client := scraper.NewClient(15 * time.Second)
	client.SetLanguage(lang)
	client.SetCache(cache.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("🔍 Fetching metadata for '%s' (with local disk cache)...\n", id)
	movie, err := client.Scrape(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	// Also persist to SQLite DB
	if defaultDB, dErr := db.Default(); dErr == nil && defaultDB != nil {
		_ = defaultDB.SaveMovie(movie)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(movie)
}

func runActress(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: r19dev actress [list | follow <name> | unfollow <name> | check]")
		return
	}

	d, err := db.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database error: %v\n", err)
		os.Exit(1)
	}
	svc := actress.New(d, nil)

	subCmd := strings.ToLower(args[0])
	switch subCmd {
	case "list":
		actresses, err := svc.ListFollowed()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error listing actresses: %v\n", err)
			os.Exit(1)
		}
		if len(actresses) == 0 {
			fmt.Println("No actresses followed yet. Use: r19dev actress follow \"<name>\"")
			return
		}
		fmt.Printf("=== Followed Actresses (%d) ===\n", len(actresses))
		for _, a := range actresses {
			ja := ""
			if a.JaName != "" {
				ja = fmt.Sprintf("(%s)", a.JaName)
			}
			fmt.Printf("⭐ %-25s %-20s [Followed: %s]\n", a.Name, ja, a.FollowedAt.Format("2006-01-02"))
		}

	case "follow":
		if len(args) < 2 {
			fmt.Println("Usage: r19dev actress follow \"<Actress Name>\"")
			return
		}
		name := args[1]
		if err := svc.Follow(name, "", ""); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to follow %s: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("✅ Successfully followed '%s'\n", name)

	case "unfollow":
		if len(args) < 2 {
			fmt.Println("Usage: r19dev actress unfollow \"<Actress Name>\"")
			return
		}
		name := args[1]
		if err := svc.Unfollow(name); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to unfollow %s: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("🗑️ Unfollowed '%s'\n", name)

	case "check":
		var summaries []actress.ActressSummary
		if len(args) > 1 {
			targetName := strings.TrimSpace(args[1])
			summary, err := svc.GetActressSummary(context.Background(), targetName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error checking releases for %s: %v\n", targetName, err)
				os.Exit(1)
			}
			if summary != nil {
				if followed, _ := d.ListFollowedActresses(); len(followed) > 0 {
					for _, a := range followed {
						if strings.EqualFold(a.Name, targetName) {
							summary.Actress = a
							break
						}
					}
				}
				summaries = append(summaries, *summary)
			}
		} else {
			var err error
			summaries, err = svc.CheckAllFollowed(context.Background())
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error checking releases: %v\n", err)
				os.Exit(1)
			}
		}
		if len(summaries) == 0 {
			fmt.Println("No releases found or no actresses followed yet.")
			return
		}
		fmt.Printf("=== Actress Release Tracker & Filmography ===\n\n")
		for _, s := range summaries {
			ja := ""
			if s.Actress.JaName != "" {
				ja = fmt.Sprintf("(%s)", s.Actress.JaName)
			}
			fmt.Printf("⭐ %s %s | Total: %d | 🟢 Downloaded: %d | 🔴 Missing: %d | 👁️ Watched: %d\n",
				s.Actress.Name, ja, s.Total, s.Downloaded, s.Missing, s.Watched)
			for _, r := range s.Releases {
				statusIcon := "🔴 Missing"
				if r.IsDownloaded {
					statusIcon = "🟢 Downloaded"
				}
				watchedIcon := ""
				if r.IsWatched {
					watchedIcon = " [👁️ Watched]"
				}
				stars := ""
				if r.UserRating > 0 {
					stars = fmt.Sprintf(" [%s]", strings.Repeat("⭐", r.UserRating))
				}
				fmt.Printf("   ├─ [%-10s] %s | %-12s %s%s%s\n",
					r.ReleaseDate, r.MovieID, statusIcon, r.Title, watchedIcon, stars)
			}
			fmt.Println()
		}
	}
}

func runOrganize(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: r19dev organize <source_dir> <destination_root> [--dry-run]")
		return
	}

	srcDir := args[0]
	destRoot := args[1]
	dryRun := false
	for _, a := range args[2:] {
		if a == "--dry-run" || a == "-n" {
			dryRun = true
		}
	}

	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid source path: %v\n", err)
		os.Exit(1)
	}
	absDest, err := filepath.Abs(destRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid destination path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🚀 Scanning source directory: %s\n", absSrc)
	sc := scanner.New(scanner.DefaultConfig())
	res, err := sc.Scan(absSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
		os.Exit(1)
	}

	mc, err := matcher.New(matcher.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Matcher error: %v\n", err)
		os.Exit(1)
	}
	matches := mc.Match(res.Files)

	fmt.Printf("📂 Found %d video files (%d matched with JAV ID)\n", len(res.Files), len(matches))
	if dryRun {
		fmt.Println("⚠️  DRY-RUN MODE ENABLED (No files will be moved or downloaded)")
	}
	fmt.Println()

	client := scraper.NewClient(15 * time.Second)
	client.SetCache(cache.Default())
	d, _ := db.Default()

	successCount := 0
	ctx := context.Background()

	for _, match := range matches {
		if match.ID == "" {
			continue
		}

		// Fetch movie metadata
		movie, err := client.Scrape(ctx, match.ID)
		if err != nil {
			fmt.Printf("⚠️  [SKIPPED] %s: Metadata not found (%v)\n", match.ID, err)
			continue
		}

		var userState *db.UserState
		if d != nil {
			userState, _ = d.GetUserState(match.ID)
		}

		orgRes, err := organizer.OrganizeMatch(ctx, &match, movie, userState, absDest, dryRun)
		if err != nil {
			fmt.Printf("❌ [FAILED] %s: %v\n", match.ID, err)
			continue
		}

		actionLabel := "MOVED"
		if dryRun {
			actionLabel = "PLAN"
		}

		fmt.Printf("✅ [%s] %s -> %s\n", actionLabel, match.ID, orgRes.TargetFolder)
		fmt.Printf("    Video: %s\n", filepath.Base(orgRes.TargetVideo))
		fmt.Printf("    Assets: NFO, HTML, poster.jpg, fanart.jpg, extrafanart/ (%d screenshots)\n", orgRes.ScreenshotsNum)
		successCount++
	}

	if !dryRun && successCount > 0 && d != nil {
		backupPath := filepath.Join(absDest, ".r19dev_backup.db")
		if err := d.BackupTo(backupPath); err == nil {
			fmt.Printf("💾 [Auto-Backup] Saved database snapshot to %s\n", backupPath)
		}
	}

	fmt.Printf("\n✨ Finished! Successfully organized %d/%d movies into %s\n", successCount, len(matches), absDest)
}

func runMigrate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: r19dev migrate <source_dir> [destination_root] [--dry-run] [--yes] [--no-tui] [--upgrade-all-html]")
		os.Exit(1)
	}

	srcDir := args[0]
	destRoot := "/Volumes/home/BT/organized"
	dryRun := false
	autoConfirm := false
	noTUI := false
	updateExisting := false

	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--dry-run" || a == "-n" {
			dryRun = true
		} else if a == "--yes" || a == "-y" {
			autoConfirm = true
		} else if a == "--no-tui" {
			noTUI = true
		} else if a == "--upgrade-all-html" || a == "--update-all-html" || a == "-u" {
			updateExisting = true
		} else if a == "--no-update-html" {
			updateExisting = false
		} else if !strings.HasPrefix(a, "-") && destRoot == "/Volumes/home/BT/organized" && i == 1 {
			destRoot = a
		}
	}

	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid source path: %v\n", err)
		os.Exit(1)
	}
	absDest, err := filepath.Abs(destRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid destination path: %v\n", err)
		os.Exit(1)
	}

	cfg := migrator.Config{
		SourceDir:      absSrc,
		DestRoot:       absDest,
		DryRun:         dryRun,
		AutoConfirm:    autoConfirm,
		UpdateExisting: updateExisting,
		NoTUI:          noTUI,
	}

	ctx := context.Background()
	var summary *migrator.Summary
	if noTUI {
		summary, err = migrator.RunCLI(ctx, cfg)
	} else {
		summary, err = migrator.RunTUI(ctx, cfg)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Migration error: %v\n", err)
		os.Exit(1)
	}

	if summary != nil {
		fmt.Println("\n==================================================")
		if dryRun {
			fmt.Println("🏁 MIGRATION DRY-RUN SUMMARY:")
			fmt.Printf("   🔍 Would Organize: %d movies\n", summary.OrganizedCount)
		} else {
			fmt.Println("🏁 MIGRATION COMPLETE SUMMARY:")
			fmt.Printf("   ✅ Successfully Organized: %d movies\n", summary.OrganizedCount)
			fmt.Printf("   🔄 Updated Existing HTML: %d files\n", summary.UpdatedHTMLNum)
		}
		fmt.Printf("   ℹ️  Duplicates Handled: %d\n", summary.DuplicateCount)
		fmt.Printf("   ⚠️  Skipped Files: %d\n", summary.SkippedCount)
		fmt.Printf("   ❌ Errors: %d\n", summary.ErrorCount)
		fmt.Printf("   ⏱  Duration: %s\n", summary.Duration.Round(time.Second))
		fmt.Println("==================================================")
	}
}

func runUpgradeHTML(args []string) {
	destRoot := "/Volumes/home/BT/organized"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		destRoot = args[0]
	}
	absDest, err := filepath.Abs(destRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid destination path: %v\n", err)
		os.Exit(1)
	}

	homeDir, _ := os.UserHomeDir()
	dumpDBPath := filepath.Join(homeDir, "Library", "Application Support", "r19dev", "r18_dump.db")
	dumpDB, err := sql.Open("sqlite", dumpDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open dump database: %v\n", err)
		os.Exit(1)
	}
	defer dumpDB.Close()

	appDBPath := filepath.Join(homeDir, "Library", "Application Support", "r19dev", "r19dev.db")
	appDB, err := sql.Open("sqlite", appDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open app database: %v\n", err)
		os.Exit(1)
	}
	defer appDB.Close()

	fmt.Printf("🎬 Upgrading movie.html files in %s to Cinematic template (16 concurrent workers)...\n", absDest)
	t0 := time.Now()
	count := migrator.UpgradeHTMLFiles(context.Background(), absDest, appDB, dumpDB, func(e migrator.ProgressEvent) {
		if e.Total > 0 && (e.Current == 1 || e.Current%25 == 0 || e.Current == e.Total) {
			fmt.Printf("[%d/%d] 🔄 Upgraded %s movie.html\n", e.Current, e.Total, e.MovieID)
		}
	})
	dur := time.Since(t0)
	var speed float64
	if dur.Seconds() > 0 {
		speed = float64(count) / dur.Seconds()
	}
	fmt.Printf("✨ Finished! Upgraded %d movie.html files in %s (speed: %.1f files/sec)\n", 
		count, dur.Round(time.Millisecond), speed)
}

func runBackup(args []string) {
	destPath := "/Volumes/home/BT/organized/.r19dev_backup.db"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		destPath = args[0]
		if stat, err := os.Stat(destPath); err == nil && stat.IsDir() {
			destPath = filepath.Join(destPath, ".r19dev_backup.db")
		}
	}
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid destination path: %v\n", err)
		os.Exit(1)
	}

	d, err := db.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	fmt.Printf("💾 Creating atomic, crash-consistent backup of r19dev.db at %s...\n", absDest)
	t0 := time.Now()
	if err := d.BackupTo(absDest); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Backup failed: %v\n", err)
		os.Exit(1)
	}
	fi, _ := os.Stat(absDest)
	var sizeMB float64
	if fi != nil {
		sizeMB = float64(fi.Size()) / 1024 / 1024
	}
	fmt.Printf("✨ Backup successful! Saved to %s (%.2f MB) in %s\n", absDest, sizeMB, time.Since(t0).Round(time.Millisecond))
}

func runFilters(args []string) {
	subCmd := "show"
	if len(args) > 0 {
		subCmd = strings.ToLower(args[0])
	}

	switch subCmd {
	case "path":
		fmt.Println(scraper.GetFilterConfigPath())

	case "reset":
		cfg, err := scraper.ResetFilterConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to reset filter config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Filter configuration reset to default (%s)\n", scraper.GetFilterConfigPath())
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(cfg)

	case "purge":
		d, err := db.Default()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to open database: %v\n", err)
			os.Exit(1)
		}
		defer d.Close()

		fmt.Printf("🧹 Scanning and purging unowned compilations & promotional variants matching filter rules...\n")
		t0 := time.Now()
		purged, err := d.PurgePromotionalVariants()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Purge failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✨ Purged %d unowned duplicate/compilation movies in %s\n", purged, time.Since(t0).Round(time.Millisecond))

	case "show", "list", "view":
		fallthrough
	default:
		cfg := scraper.GetActiveFilterConfig()
		cfgPath := scraper.GetFilterConfigPath()
		fmt.Printf("📋 Active Filter Configuration (%s):\n\n", cfgPath)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(cfg)
		fmt.Printf("\n💡 To customize rules, edit: %s\n", cfgPath)
		fmt.Println("💡 To apply changes and purge matching titles: r19dev filters purge")
	}
}

func printHelp() {
	fmt.Println(`🎬 R19DEV Scraper - JAV Scanner, Matcher, Actress Tracker & NAS Jellyfin Organizer

Usage:
  r19dev [path]               Launch Modern Web UI Studio (default: .)
  r19dev web [path] [flags]   Launch Modern Web UI Studio
                              Flags:
                                --port <8080>           HTTP server port (default: 8080)
                                --no-open               Do not auto-open default browser

  r19dev tui [path] [flags]   Launch Interactive Terminal TUI
                              Flags:
                                --lang <en|ja>          Language preference (default: en)
                                --proto <auto|halfblock>  Graphics protocol

  r19dev scan [path] [--json] Run directory scan & ID matching
  r19dev scrape <JAV-ID>      Fetch metadata directly from R18.dev
  r19dev actress <command>    Track favorite actresses & check new releases
                              Commands:
                                list                    List followed actresses
                                follow "<name>"         Follow an actress
                                unfollow "<name>"       Unfollow an actress
                                check                   Check new releases & status

  r19dev organize <src> <dest> [--dry-run]
                              Organize video library into NAS Jellyfin structure:
                              {dest}/{Actress}/{JAV-ID} {Title}/
                              - Moves video (with multi-part support)
                              - Generates Jellyfin .nfo
                              - Generates Standalone movie.html
                              - Downloads High-Res poster.jpg & fanart.jpg
                              - Downloads Sample screenshots into extrafanart/

  r19dev migrate <src> [dest] [--dry-run] [--yes] [--no-tui] [--upgrade-all-html]
                              Fast batch migration & reorganization using local dump DB
                              - Interactive TUI progress bar with live stats & activity log
                              - Reuses local assets (poster, fanart, extrafanart/) instantly
                              - Generates Jellyfin NFO and Cinematic movie.html for new items
                              - Use --upgrade-all-html to also upgrade existing library items

  r19dev filters [cmd]        Manage promotional & omnibus exclusion rules:
                              Commands:
                                show                    View active filter configuration (default)
                                path                    Print path to filters.json
                                purge                   Purge unowned titles matching filter rules
                                reset                   Reset filters.json to factory defaults

  r19dev upgrade-html [path]  Concurrently upgrade all movie.html to Cinematic template
  r19dev backup [path]        Create crash-consistent SQLite backup snapshot (default: NAS)
  r19dev clear-cache          Clear local metadata and image cache
  r19dev --version            Show version
  r19dev --help               Show this help message`)
}
