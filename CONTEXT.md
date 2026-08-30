# 🧠 Engineering Context — R19DEV Scraper

## 1. Project Background & Identity

`r19dev-scraper` is a Go-based scanner, pattern matching engine, and metadata scraper tailored for Japanese Adult Video (JAV) media libraries. It features an interactive TUI built with Bubble Tea (`charmbracelet/bubbletea`) and Lip Gloss (`charmbracelet/lipgloss`), alongside a non-interactive CLI for scripting.

### Primary Problem Solved:
JAV media files downloaded from various torrents, Usenet groups, or DMM web rips often contain inconsistent prefixes (e.g. `hhd800.com@`, `[4k2.com]`), non-standard hyphenation (e.g. `kavr00428`, `SNOS028`), or multi-part tags (`_1_8k`, `pt2`). `r19dev-scraper` parses these filenames safely, normalizes the ID, queries the official/semi-official R18.dev REST API, and presents the enriched metadata in an interactive terminal dashboard.

---

## 2. Tech Stack & Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| **Go** | `>= 1.24` (`go 1.27.0` toolchain) | Core language |
| `github.com/charmbracelet/bubbletea` | `v1.3.4` | TUI runtime (Elm Architecture event loop) |
| `github.com/charmbracelet/lipgloss` | `v1.0.0` | Declarative terminal styling, borders, colors |
| `github.com/charmbracelet/bubbles` | `v0.20.0` | UI widgets (`spinner`, `textinput`, `key`) |
| `net/http` (Standard Library) | — | HTTP client for R18.dev JSON API queries |
| `io/fs`, `path/filepath` | — | Filesystem traversal and path manipulation |

---

## 3. Repository Directory Structure & Module Boundaries

```
r19dev-scraper/
├── cmd/
│   └── r19dev/
│       └── main.go           # CLI argument parsing, entrypoints for TUI / Scan / Scrape
├── pkg/
│   ├── scanner/              # Module 1: Filesystem Discovery & Validation
│   │   ├── config.go         # Config struct & defaults (extensions, min size, exclusions)
│   │   ├── scanner.go        # Directory crawler (WalkDir, symlink safety, cancellation)
│   │   └── scanner_test.go   # Scanner unit tests
│   ├── matcher/              # Module 2: Pattern Recognition & Multi-part Parsing
│   │   ├── config.go         # Regex and noise configuration
│   │   ├── matcher.go        # Boundary-safe regexes, ID extractors, noise stripper
│   │   ├── multipart.go      # Part parsing (pt1, _1, etc.) & sibling directory validator
│   │   └── matcher_test.go   # Test cases covering real-world filenames
│   ├── scraper/              # Module 3: Metadata Retrieval & Normalization
│   │   ├── models.go         # Data structures: Movie, Actress
│   │   ├── normalizer.go     # Conversion: JAV ID -> R18 combined format
│   │   ├── r18dev.go         # HTTP client communicating with R18.dev API
│   │   └── r18dev_test.go    # Unit tests for ID normalizer
│   └── tui/                  # Module 4: Interactive Terminal Dashboard
│       ├── app.go            # Bubble Tea Model (Init, Update, async Cmd handlers)
│       ├── views.go          # View layout: Split screen (file table + metadata inspector)
│       ├── edit_modal.go     # Textinput modal for manual ID override
│       ├── keys.go           # Key bindings definition
│       └── styles.go         # Lip Gloss theme tokens and palette
├── .gitignore                # Go build artifact ignore rules
├── Makefile                  # Build and test orchestration
├── README.md                 # User-facing manual and quick start
├── OKF.md                    # Operational Knowledge Framework & Specs
├── CONTEXT.md                # Developer & Agent architectural context (this file)
└── HANDOFF.md                # Maintenance & Handover guide
```

---

## 4. Key Architectural Invariants & Patterns

### 4.1 Non-Blocking IO in TUI
* Never perform network requests or disk scans synchronously inside `Update()` or `View()`.
* Wrap long-running IO in `tea.Cmd` returning dedicated message structs (`scanDoneMsg`, `scrapeDoneMsg`).
* The UI retains interactivity at 60 FPS while background workers execute.

### 4.2 Safe Regex Word Boundaries
* Standard `\b` boundaries in Go's `regexp` package treat underscores (`_`) as word characters.
* To properly match IDs embedded in filenames like `4k2.com@kavr00428_1_8k.mp4`, regexes use boundary assertions:
  ```regex
  (?:^|[^a-zA-Z0-9])([A-Za-z]{2,6})-(\d{2,5})(?:$|[^a-zA-Z0-9])
  ```

### 4.3 In-Memory Cache
* `tui.Model` maintains a `metadataCache map[string]*scraper.Movie` and `scrapeErrors map[string]string`.
* When the user scrolls through previously inspected items, metadata renders instantly from cache without re-hitting R18.dev.

### 4.4 Symlink Protection
* The scanner executes `os.Lstat()` on every discovered node.
* Symlinks are filtered out (`lstat.Mode() & os.ModeSymlink != 0`) to guarantee immunity from circular links or traversal escapes.

---

## 5. Domain Knowledge: JAV ID Conventions

1. **Standard Hyphenated**: `[Letters 2-6]-[Numbers 2-5]` (e.g. `MIDA-517`, `SNOS-028`, `WAAA-615`).
2. **VR 5-Digit**: `[Letters 3-5][Numbers 5]` (e.g. `kavr00428`, `sivr00394`). Display format: `KAVR-428`, `SIVR-394`.
3. **FC2**: `FC2-PPV-[Numbers 5-8]` (e.g. `FC2-PPV-1234567`). R18 combined format: `fc2-1234567`.
4. **Uncensored Date-Based**: `[YYMMDD]_[NNN]-[LABEL]` (e.g. `020326_001-1PON`, `100122_001-CARIB`).
5. **DMM Content ID**: `h_[number][letters][number]` (e.g. `h_1472smkcx003`).

---

## 6. Developer Commands

```bash
# Build the binary into bin/r19dev
make build

# Run all test suites with verbose output
make test

# Launch TUI on a test directory
./bin/r19dev tui /path/to/videos

# CLI scan with JSON output
./bin/r19dev scan /path/to/videos --json

# Direct API scrape check
./bin/r19dev scrape MIDA-517
```
