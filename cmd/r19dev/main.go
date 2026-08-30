package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
	"github.com/dagalp/r19dev-scraper/pkg/tui"
)

const version = "1.0.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runTUI(".")
		return
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "tui":
		target := "."
		if len(args) > 1 {
			target = args[1]
		}
		runTUI(target)

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
			fmt.Println("Usage: r19dev scrape <JAV-ID>")
			os.Exit(1)
		}
		id := args[1]
		runScrape(id)

	case "-v", "--version", "version":
		fmt.Printf("r19dev-scraper version %s\n", version)

	case "-h", "--help", "help":
		printHelp()

	default:
		// If argument is a directory, launch TUI on that directory
		if stat, err := os.Stat(args[0]); err == nil && stat.IsDir() {
			runTUI(args[0])
			return
		}
		printHelp()
	}
}

func runTUI(targetDir string) {
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid path: %v\n", err)
		os.Exit(1)
	}

	model, err := tui.New(absPath)
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

func runScrape(id string) {
	client := scraper.NewClient(15 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("🔍 Fetching metadata for '%s' from R18.dev...\n", id)
	movie, err := client.Scrape(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(movie)
}

func printHelp() {
	fmt.Println(`🎬 R19DEV Scraper - JAV Scanner, Matcher & R18.dev Scraper

Usage:
  r19dev [path]               Launch Interactive TUI on specified directory (default: .)
  r19dev tui [path]           Launch Interactive TUI
  r19dev scan [path] [--json] Run directory scan & ID matching
  r19dev scrape <JAV-ID>      Fetch metadata directly from R18.dev for a JAV ID
  r19dev --version            Show version
  r19dev --help               Show this help message`)
}
