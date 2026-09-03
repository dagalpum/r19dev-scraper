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
│       └── main.go           # CLI argument parsing, entrypoints for Web / TUI / Scan / Scrape / Actress / Organize
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
│   ├── db/                   # Module 4: SQLite Database & Audit Trail
│   │   ├── db.go             # SQLite engine (pure Go modernc.org/sqlite), schema, migrations, auto-pruning
│   │   └── db_test.go        # Tests for user states, movies, library files, and operation history
│   ├── organizer/            # Module 5: NAS Directory & Jellyfin Organizer
│   │   ├── organizer.go      # Move/Copy planning, multi-part handling, granular step progress reporter
│   │   └── organizer_test.go # Plan & execution tests with dry-run verification
│   ├── jellyfin/             # Module 6: Jellyfin Metadata & Media Assets
│   │   ├── nfo.go            # Kodi/Jellyfin NFO XML generator & 180-byte safe filename sanitizer
│   │   ├── html.go           # Standalone offline dark-mode HTML viewer generator
│   │   ├── assets.go         # Asset downloader (poster, fanart, extrafanart) with granular progress
│   │   └── jellyfin_test.go  # Tests for NFO XML, HTML, and DMM URL upgrader
│   ├── actress/              # Module 7: Actress Tracking Service
│   │   ├── service.go        # Follow/Unfollow, filmography cross-reference against local library
│   │   └── service_test.go   # Actress service tests
│   ├── cache/                # Module 8: Persistent Disk Cache
│   │   ├── cache.go          # LRU disk cache for API payloads and image assets
│   │   └── cache_test.go     # Cache persistence tests
│   ├── web/                  # Module 9: Single-Binary Web UI Studio
│   │   ├── server.go         # HTTP router, SSE stream handlers (scan, scrape, organize), timeout guards
│   │   ├── server_test.go    # REST and streaming endpoint test suite
│   │   └── static/           # Embedded SPA assets (index.html, style.css, app.js, vendor/lucide.min.js)
│   └── tui/                  # Module 10: Interactive Terminal Dashboard
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

### 4.1 Non-Blocking IO & SSE Streaming
* Long-running operations (organize, scan, scrape) stream real-time events over Server-Sent Events (SSE) with granular steps (`step`, `item`, `done`).
* Global `http.Server.WriteTimeout` is kept disabled for streaming endpoints, with write deadlines reset via `rc := http.NewResponseController(w); rc.SetWriteDeadline(time.Time{})` to allow continuous processing for up to 30 minutes without disconnection.

### 4.2 English Metadata Hierarchy & 180-Byte Filesystem Limits
* **Naming Convention**: Folder structure follows `<Dest>/<Actress_Name>/<JAV-ID Title>/`. Actress name and movie title prioritize English metadata, falling back to Japanese only when English is absent.
* **ENAMETOOLONG Prevention**: Single directory components are capped at $\le 180$ bytes along UTF-8 rune boundaries, preventing OS filesystem `ENAMETOOLONG` errors (255-byte limit on APFS, ext4, NTFS, and SMB shares).

### 4.3 SQLite Audit Trail with Auto-Pruning
* All organize and scrape operations are logged into the `operation_history` table in SQLite (`~/Library/Caches/r19dev/r19dev.db` on macOS).
* Automated pruning keeps only the last 100 entries and purges logs older than 30 days, guaranteeing zero disk clutter.

### 4.4 Symlink Protection & Boundary-Safe Regexes
* The scanner executes `os.Lstat()` on every node; symlinks are filtered out to guarantee immunity from circular loops.
* Regex matching uses boundary assertions `(?:^|[^a-zA-Z0-9])` instead of standard `\b` to avoid splitting on underscores.

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
# Build standalone binary into bin/r19dev
make build

# Run all test suites
make test

# Launch Web UI Studio
./bin/r19dev web /Volumes/home/BT/2026

# Launch TUI
./bin/r19dev tui /Volumes/home/BT/2026

# CLI Scan
./bin/r19dev scan /Volumes/home/BT/2026 --json
```
