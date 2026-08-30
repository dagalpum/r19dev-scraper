# 🎬 R19DEV Scraper

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Bubble Tea](https://img.shields.io/badge/TUI-Bubble%20Tea-F25D94?style=flat&logo=charm)](https://github.com/charmbracelet/bubbletea)
[![Lip Gloss](https://img.shields.io/badge/Styling-Lip%20Gloss-7D56F4?style=flat)](https://github.com/charmbracelet/lipgloss)
[![R18.dev API](https://img.shields.io/badge/API-R18.dev-04B575?style=flat)](https://r18.dev)

A fast, lightweight, standalone JAV (Japanese Adult Video) library scanner, intelligent pattern matcher, and **R18.dev** metadata scraper built in Go. It includes an interactive, responsive **Terminal User Interface (TUI)** powered by Charm's Bubble Tea and Lip Gloss.

---

## ✨ Features

- 📁 **High-Performance Directory Scanner (`pkg/scanner`)**:
  - Recursively crawls directories with context cancellation and timeout protection.
  - Skips non-video files and filters out samples/ads based on minimum size (`MinSizeMB`) and configurable glob patterns.
  - Safe against cyclic symlinks and directory escapes (`os.ModeSymlink` detection).

- 🎯 **Intelligent JAV ID Matcher (`pkg/matcher`)**:
  - **Noise Stripping**: Automatically removes tracker / release-group prefixes (e.g. `hhd800.com@`, `4k2.com@`, `twojav.com@`, `[site]`).
  - **Standard Hyphenated IDs**: Matches `MIDA-517`, `SNOS-028`, `WAAA-615`, `PRWF-010`.
  - **VR 5-Digit Content IDs**: Converts unhyphenated VR IDs to standard display formats (e.g., `kavr00428` $\rightarrow$ `KAVR-428`, `sivr00394` $\rightarrow$ `SIVR-394`).
  - **FC2 & Uncensored**: Matches `FC2-PPV-XXXXXXX` and date-coded uncensored releases (e.g., `020326_001-1PON`).
  - **Multi-Part Heuristics**: Detects multi-part releases (`_1`, `_2_8k`, `pt1`, `-A`) with directory sibling validation.

- 🌐 **R18.dev JSON API Scraper (`pkg/scraper`)**:
  - Automatically translates raw/matched JAV IDs to R18.dev combined format (e.g., `MIDA-517` $\rightarrow$ `mida00517`, `KAVR-428` $\rightarrow$ `kavr00428`, `FC2-PPV-1234567` $\rightarrow$ `fc2-1234567`).
  - Fetches rich metadata including Title (English + Japanese), Maker, Label, Series, Director, Release Date, Runtime, Actresses, Categories/Genres, Full Jacket Cover, and Sample Screenshots.

- 🖥️ **Interactive Terminal UI (`pkg/tui`)**:
  - Split-view dashboard: File table on the left, live metadata inspector on the right.
  - Real-time status indicators (🟢 Matched, 🔴 Unmatched, 🟡 Multi-part).
  - Manual ID override modal (`e` key) for edge-case filenames with immediate live re-scraping.
  - Keyboard-driven navigation with Vim keybindings (`j`/`k`/`g`/`G`).

---

## 📂 Project Structure

```
r19dev-scraper/
├── cmd/
│   └── r19dev/
│       └── main.go           # CLI entry point (subcommands: tui, scan, scrape, version)
├── pkg/
│   ├── scanner/              # File discovery & validation
│   │   ├── config.go         # Scanner configuration
│   │   ├── scanner.go        # Recursive directory walker
│   │   └── scanner_test.go   # Unit tests
│   ├── matcher/              # Regex pattern matching & multipart detection
│   │   ├── config.go         # Matcher configuration
│   │   ├── matcher.go        # Regex engine & ID normalization
│   │   ├── multipart.go      # Multi-part detection heuristics
│   │   └── matcher_test.go   # Real-world test cases
│   ├── scraper/              # R18.dev API integration
│   │   ├── models.go         # Metadata structs (Movie, Actress)
│   │   ├── normalizer.go     # ID conversion to R18 combined format
│   │   ├── r18dev.go         # HTTP client & API parser
│   │   └── r18dev_test.go    # Normalizer unit tests
│   └── tui/                  # Bubble Tea / Lip Gloss terminal UI
│       ├── app.go            # Bubble Tea Model & Update loop
│       ├── views.go          # Layout rendering (split panels, table, detail)
│       ├── edit_modal.go     # Manual ID override modal
│       ├── keys.go           # Keybinding mappings
│       └── styles.go         # Lip Gloss theme & colors
├── CONTEXT.md                # AI Agent & Developer Architecture Context
├── OKF.md                    # Operational Knowledge Framework & Specifications
├── HANDOFF.md                # Engineering Handoff & Maintenance Guide
├── Makefile                  # Build & test automation
├── go.mod                    # Module definition
└── go.sum                    # Checksums
```

---

## 🚀 Getting Started

### Prerequisites
- **Go 1.24+** installed.

### Installation & Build

```bash
# Clone the repository
git clone https://github.com/dagalpum/r19dev-scraper.git
cd r19dev-scraper

# Build binary
make build
# Output will be located at bin/r19dev
```

---

## 💻 Usage

### 1. Interactive TUI Mode (Default)
Launch the TUI on the current directory or target video directory:
```bash
# Scan current directory
./bin/r19dev

# Scan specific folder
./bin/r19dev tui /Volumes/Media/JAV/2026
# or simply
./bin/r19dev /Volumes/Media/JAV/2026
```

### 2. CLI Scan & Match
Run a non-interactive scan and output matched files:
```bash
# Formatted console output
./bin/r19dev scan /Volumes/Media/JAV/2026

# Output as JSON for scripts & pipelines
./bin/r19dev scan /Volumes/Media/JAV/2026 --json
```

### 3. Direct R18.dev Scraper CLI
Directly query metadata for any JAV ID:
```bash
./bin/r19dev scrape MIDA-517
```

---

## ⌨️ TUI Controls

| Key | Action |
|:---:|:---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `PgUp` / `b` | Page up (10 items) |
| `PgDn` / `f` | Page down (10 items) |
| `g` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `Enter` / `Space` | Fetch R18.dev metadata for selected item |
| `e` | Open Edit Modal (Manually override JAV ID) |
| `r` | Rescan directory |
| `q` / `Ctrl+C` | Quit application |

---

## 🧪 Testing

Run all unit tests across scanner, matcher, and scraper packages:

```bash
make test
# or: go test -v ./...
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
