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
	"github.com/dagalp/r19dev-scraper/pkg/cache"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
	"github.com/dagalp/r19dev-scraper/pkg/tui"
)

const version = "1.0.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runTUI(".", "en", tui.ProtocolAuto)
		return
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
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
			runTUI(args[0], "en", tui.ProtocolAuto)
			return
		}
		printHelp()
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

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(movie)
}

func printHelp() {
	fmt.Println(`🎬 R19DEV Scraper - JAV Scanner, Matcher & R18.dev Scraper

Usage:
  r19dev [path]               Launch Interactive TUI (default: .)
  r19dev tui [path] [flags]   Launch Interactive TUI
                              Flags:
                                --lang <en|ja>          Language preference (default: en)
                                --proto <auto|halfblock>  Graphics protocol

  r19dev scan [path] [--json] Run directory scan & ID matching
  r19dev scrape <JAV-ID>      Fetch metadata directly from R18.dev
  r19dev clear-cache          Clear local metadata and image cache
  r19dev --version            Show version
  r19dev --help               Show this help message`)
}
