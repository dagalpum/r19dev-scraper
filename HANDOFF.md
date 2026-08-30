# 🤝 Engineering Handoff — R19DEV Scraper

## 1. Executive Summary

**Project Name**: `r19dev-scraper`  
**Current Version**: `v1.0.0`  
**Language / Runtime**: Go 1.24+ (`go 1.27.0`)  
**Primary Function**: High-performance local media library scanner, intelligent JAV filename parser, and interactive R18.dev metadata scraper.

The codebase is clean, well-tested, modular, and ready for production usage or integration into larger media management ecosystems.

---

## 2. Current Health & Status

| Area | Status | Notes |
|---|---|---|
| **Compilation** | ✅ Passing | Clean build via `go build -o bin/r19dev ./cmd/r19dev` |
| **Unit Tests** | ✅ Passing | 100% pass rate across `pkg/scanner`, `pkg/matcher`, and `pkg/scraper` |
| **Dependencies** | ✅ Stable | Using standard library + official Charm packages (`bubbletea`, `lipgloss`, `bubbles`) |
| **Performance** | ✅ Fast | Zero UI lag; asynchronous IO for disk traversal and HTTP requests |
| **Documentation** | ✅ Complete | Includes `README.md`, `OKF.md`, `CONTEXT.md`, `HANDOFF.md` |

---

## 3. Key Components & File Map

```
pkg/
├── scanner/
│   ├── config.go         -> Configuration (extensions, min size, exclusion globs)
│   ├── scanner.go        -> Safe WalkDir engine with symlink detection & timeout guards
│   └── scanner_test.go   -> Verifies filtering, exclusion, and symlink handling
├── matcher/
│   ├── config.go         -> Matcher settings (custom regex toggle, noise filter)
│   ├── matcher.go        -> Regex engine with boundary checks, JAV ID normalizer
│   ├── multipart.go      -> Part detection (_1, pt1, -A) & directory sibling validator
│   └── matcher_test.go   -> Real-world filename test fixtures
├── scraper/
│   ├── models.go         -> Domain models (Movie, Actress)
│   ├── normalizer.go     -> JAV ID to R18 combined ID converter (e.g. MIDA-517 -> mida00517)
│   ├── r18dev.go         -> HTTP client communicating with R18.dev JSON API
│   └── r18dev_test.go    -> Unit tests for combined ID normalization
└── tui/
    ├── app.go            -> Elm Architecture Model & Update loop
    ├── views.go          -> Responsive split-view layout rendering (Table + Detail)
    ├── edit_modal.go     -> Textinput overlay for manual ID correction
    ├── keys.go           -> Default keymap (Vim + standard arrow navigation)
    └── styles.go         -> Color palette, borders, badges, Lip Gloss styles
```

---

## 4. How to Build, Test, and Run

### 4.1 Build
```bash
make build
# Binary created at: bin/r19dev
```

### 4.2 Test
```bash
make test
# or: go test -v ./...
```

### 4.3 Running the Application
```bash
# 1. Interactive TUI Mode
./bin/r19dev /path/to/jav/library

# 2. CLI Scan Mode (Standard / JSON)
./bin/r19dev scan /path/to/jav/library
./bin/r19dev scan /path/to/jav/library --json

# 3. Direct Scraper Query
./bin/r19dev scrape MIDA-517
```

---

## 5. Key Architecture & Design Decisions

1. **Boundary-Safe Regular Expressions**:
   Instead of standard `\b` (which breaks on underscores in filenames like `kavr00428_1_8k`), the matcher uses `(?:^|[^a-zA-Z0-9])` boundary assertions.
2. **Directory Sibling Multipart Confirmation**:
   Single-letter suffixes like `-A` or `-B` are ambiguous. The engine validates whether sibling files with matching IDs exist in the same folder before marking them as multi-part.
3. **Reactive UI State Management**:
   The TUI uses asynchronous `tea.Cmd` tasks so the UI never blocks while crawling network disks or waiting for R18.dev API responses.

---

## 6. Recommended Future Roadmap & Enhancements

If extending this project in future iterations, consider the following prioritized tasks:

1. **NFO File & Poster Exporter**:
   - Add a command/keybinding (`s` or `w`) to generate Kodi/Plex compatible `.nfo` files and download `poster.jpg` / `fanart.jpg` directly alongside video files.
2. **Multi-Provider Scraper Architecture**:
   - Abstract `scraper.Client` into an interface to support fallback scrapers (e.g. JavLibrary, DMM, JavDB, FC2Club) when R18.dev returns a 404.
3. **Persistent SQLite Caching**:
   - Persist scanned libraries and fetched metadata to a local SQLite database (`~/.r19dev/cache.db`) for instant restarts on large collections.
4. **File Renamer / Organizer**:
   - Implement template-based renaming (e.g. `<MAKER>/<ID> [<ACTRESS>] - <TITLE>/<ID>.<EXT>`).

---

## 7. Operational Contact & Maintenance

* **Maintainer**: `dagalpum`
* **Repository**: `https://github.com/dagalpum/r19dev-scraper`
