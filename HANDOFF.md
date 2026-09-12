# 🤝 Engineering Handoff — R19DEV Scraper

## 1. Executive Summary

**Project Name**: `r19dev-scraper`  
**Current Version**: `v1.4.0`  
**Language / Runtime**: Go 1.24+ (`go 1.27.0` toolchain)  
**Primary Function**: High-performance local media library scanner, intelligent JAV filename parser, R18.dev metadata scraper, single-binary Web UI Studio with 2-Column Bento Profile & Filmography Stage, interactive Terminal TUI, and automated NAS Jellyfin organizer with SQLite audit trail.

The codebase is clean, thoroughly tested (100% test pass rate across all packages), modular, and fully documented.

---

## 2. Current Health & Status

| Area | Status | Notes |
|---|---|---|
| **Compilation** | ✅ Passing | Single-binary compilation via `go build -o bin/r19dev ./cmd/r19dev` |
| **Unit Tests** | ✅ Passing | 100% pass rate across `pkg/scanner`, `pkg/matcher`, `pkg/scraper`, `pkg/jellyfin`, `pkg/organizer`, `pkg/actress`, `pkg/cache`, `pkg/db`, and `pkg/web` |
| **Dependencies** | ✅ Stable | Using standard library + `modernc.org/sqlite` (pure Go, zero CGO) + Charm packages (`bubbletea`, `lipgloss`) |
| **Performance** | ✅ Fast | Zero UI lag; asynchronous IO for disk traversal, HTTP connection pooling, and live SSE streaming |
| **Documentation** | ✅ Complete | Updated `README.md`, `OKF.md`, `CONTEXT.md`, and `HANDOFF.md` |

---

## 3. Key Components & File Map

```
pkg/
├── scanner/              -> Safe WalkDir engine with symlink detection & timeout guards
├── matcher/              -> Regex engine with boundary checks, JAV ID normalizer, multipart detector
├── scraper/              -> Domain models, R18.dev REST API client, combined ID normalizer
├── db/                   -> Pure Go SQLite storage, migrations, auto-pruning, operation history audit trail
├── organizer/            -> NAS Jellyfin organization planner, multi-part merger, live progress reporter
├── jellyfin/             -> Kodi/Jellyfin NFO XML generator, 180-byte safe filename sanitizer, HTML viewer, asset downloader
├── actress/              -> Actress tracking service, filmography tracker, and local release comparator
├── cache/                -> Persistent disk cache for API payloads and images (~/.cache or ~/Library/Caches)
├── web/                  -> Single-binary Web UI Studio server, SSE streaming, REST API, embedded SPA frontend
└── tui/                  -> Elm Architecture terminal dashboard with native GPU bitmap protocols (Kitty, iTerm2, Sixel)
```

---

## 4. How to Build, Test, and Run

### 4.1 Build
```bash
make build
# Standalone binary output: bin/r19dev
```

### 4.2 Test
```bash
make test
# or: go test -v ./...
```

### 4.3 Running the Application
```bash
# 1. Web UI Studio (Recommended)
./bin/r19dev web /Volumes/home/BT/2026

# 2. Interactive TUI Mode
./bin/r19dev tui /Volumes/home/BT/2026

# 3. CLI Scan Mode (Standard / JSON)
./bin/r19dev scan /Volumes/home/BT/2026 --json

# 4. CLI Organize Mode (Dry-Run / Live)
./bin/r19dev organize /Volumes/home/BT/2026 /Volumes/home/BT/organized --dry-run
./bin/r19dev organize /Volumes/home/BT/2026 /Volumes/home/BT/organized

# 5. Direct Scraper Query
./bin/r19dev scrape SNOS-038
```

---

## 5. Key Architecture & Design Decisions

1. **Boundary-Safe Regular Expressions**:
   The matcher uses `(?:^|[^a-zA-Z0-9])` boundary assertions instead of standard `\b` to avoid splitting on underscores in filenames.
2. **English Naming Priority with Japanese Fallback**:
   Destination folders follow `/Volumes/home/BT/organized/{Actress_English_Name}/{JAV-ID} {English_Title}/`. Both actress names and movie titles prioritize English metadata, gracefully falling back to Japanese only if English is missing.
3. **Filesystem Safety & ENAMETOOLONG Prevention**:
   Filenames and folder names are sanitized and strictly capped at $\le 180$ bytes along UTF-8 rune boundaries, preventing OS filesystem crashes (`ENAMETOOLONG`) on APFS, ext4, NTFS, and SMB shares (where max component length is 255 bytes).
4. **SSE Streaming without WriteTimeout**:
   Global `http.Server.WriteTimeout` is removed to avoid cutting off long-running streams. Streaming handlers invoke `rc.SetWriteDeadline(time.Time{})` and employ 30-minute context timeouts for reliable multi-gigabyte organizing tasks.
5. **SQLite Audit Trail with Auto-Pruning**:
   All batch/single operations are recorded in SQLite (`operation_history`) with complete console logs, counts, and status badges. The table auto-prunes entries older than 30 days or beyond 100 runs, guaranteeing zero disk clutter.
6. **Smart Console Log Auto-Scroll & Copy**:
   The web console log pauses auto-scrolling when the user scrolls up, displays a floating resume button, and includes a one-click clipboard copy button.
7. **Actress Hub Dual-Mode (Chat vs. Collection Showcase)**:
   Toggle between **Chat Mode 💬** (dialogue stream with unacquired grayscale effects and status response bubbles) and **Movie Collection Mode 🎬** (full-bleed poster grid with status ribbons for `In Library`, `Staging`, and `Missing`).
8. **R18 ID Auto-Backfill & Profile Drawer**:
   Stores `r18_id` in SQLite, auto-migrated and backfilled from `movies.actresses_json`. Generates verified R18 actress links (`?id={r18_id}&type=actress`) and powers the slide-over profile drawer with collection completeness statistics.
9. **Native File Manager Integration**:
   One-click `[📂 Open in Finder]` buttons in header, chat cards, collection cards, and detail modal via `/api/open-folder`.
10. **Organized Library Consolidation**:
    All 23 actress folders consolidated under `/Volumes/home/BT/organized`, set as the permanent default across backend and frontend.
11. **Safe Database Updates & Test Isolation**:
    Guarded `SaveMovie` SQL upsert prevents partial records from wiping existing metadata, and unit tests use isolated temporary databases via `organizer.SetDB()` and `web.Config{DB}`.
12. **Multi-Tier Image Endpoint (`/api/images/{id}`)**:
    Combines in-memory cache, local organized disk poster lookup, and remote DMM fetch fallback to guarantee reliable poster display under all network and offline conditions.
13. **2-Column Bento Profile & Filmography Stage Layout**:
    Individual actress view features a 340px sticky Bento sidebar with real-time NAS storage metrics in GB (`TotalSizeBytes`), average file size, completion progress bar, career span, and clickable top genre pills (`TopGenres`). The right stage provides instant in-page search, sub-filters, active genre tags, and multi-key sorting.
14. **Local SQLite Database & Git Exclusion**:
    Database file `r19dev.db` resides in the OS user cache directory (`~/Library/Caches/r19dev/r19dev.db` on macOS) outside the repository and is ignored in `.gitignore` (`*.db`), guaranteeing it remains local and is never committed or pushed to Git.

---

## 6. Recommended Future Roadmap

The following major roadmap milestones from previous versions are now **fully completed**:
- ✅ **Kodi / Jellyfin NFO & Media Asset Exporter**: Generated automatically in standardized folders.
- ✅ **Persistent SQLite Database**: Stores user states, ratings, favorites, and organized status.
- ✅ **NAS Organizer Pipeline**: Automatic atomic move/copy with multi-part consolidation.
- ✅ **Operation History & Audit Trail**: SQLite-backed with auto-retention and Web UI viewer.
- ✅ **Interactive Actress Chat UI & Dual View**: Chat timeline mode and Movie Collection showcase.
- ✅ **Slide-Over Profile Drawer**: Stats grid, collection progress bar, and verified R18.dev links.
- ✅ **Native Finder Integration**: Instant reveal in macOS Finder / OS file manager.
- ✅ **Organized Library Default**: Standardized destination at `/Volumes/home/BT/organized`.
- ✅ **Safe Metadata Upserts & Test DB Isolation**: Guarantees zero production database corruption.
- ✅ **Multi-Tier Image Serving Pipeline**: Complete resilience with disk fallback.

Recommended future enhancements:
1. **Multi-Provider Scraper Fallbacks**:
   - Add secondary providers (e.g. JavLibrary, DMM/Fanza, JavBus) when R18.dev returns 404 for obscure or legacy titles.
2. **Batch Torrent/Download Webhook Trigger**:
   - Add a webhook endpoint (`/api/webhook/download-complete`) to trigger automatic scanning and organizing upon download completion from qBittorrent/Transmission.
3. **Subtitle Matcher & Relocation**:
   - Automatically detect matching external `.srt` / `.ass` subtitle files and copy them alongside the video as `<JAV-ID>.th.srt` or `<JAV-ID>.en.srt`.

---

## 7. Operational Contact & Maintenance

* **Maintainer**: `dagalpum`
* **Repository**: `https://github.com/dagalpum/r19dev-scraper`
