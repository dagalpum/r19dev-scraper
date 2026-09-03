# 🤝 Engineering Handoff — R19DEV Scraper

## 1. Executive Summary

**Project Name**: `r19dev-scraper`  
**Current Version**: `v1.2.0`  
**Language / Runtime**: Go 1.24+ (`go 1.27.0` toolchain)  
**Primary Function**: High-performance local media library scanner, intelligent JAV filename parser, R18.dev metadata scraper, single-binary Web UI Studio, interactive Terminal TUI, and automated NAS Jellyfin organizer with SQLite audit trail.

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
./bin/r19dev web /path/to/jav/library

# 2. Interactive TUI Mode
./bin/r19dev tui /path/to/jav/library

# 3. CLI Scan Mode (Standard / JSON)
./bin/r19dev scan /path/to/jav/library --json

# 4. CLI Organize Mode (Dry-Run / Live)
./bin/r19dev organize /path/to/source /path/to/destination --dry-run
./bin/r19dev organize /path/to/source /path/to/destination

# 5. Direct Scraper Query
./bin/r19dev scrape MIDA-517
```

---

## 5. Key Architecture & Design Decisions

1. **Boundary-Safe Regular Expressions**:
   The matcher uses `(?:^|[^a-zA-Z0-9])` boundary assertions instead of standard `\b` to avoid splitting on underscores in filenames.
2. **English Naming Priority with Japanese Fallback**:
   Destination folders follow `{Destination}/{Actress_English_Name}/{JAV-ID} {English_Title}/`. Both actress names and movie titles prioritize English metadata, gracefully falling back to Japanese only if English is missing.
3. **Filesystem Safety & ENAMETOOLONG Prevention**:
   Filenames and folder names are sanitized and strictly capped at $\le 180$ bytes along UTF-8 rune boundaries, preventing OS filesystem crashes (`ENAMETOOLONG`) on APFS, ext4, NTFS, and SMB shares (where max component length is 255 bytes).
4. **SSE Streaming without WriteTimeout**:
   Global `http.Server.WriteTimeout` is removed to avoid cutting off long-running streams. Streaming handlers invoke `rc.SetWriteDeadline(time.Time{})` and employ 30-minute context timeouts for reliable multi-gigabyte organizing tasks.
5. **SQLite Audit Trail with Auto-Pruning**:
   All batch/single operations are recorded in SQLite (`operation_history`) with complete console logs, counts, and status badges. The table auto-prunes entries older than 30 days or beyond 100 runs, guaranteeing zero disk clutter.
6. **Smart Console Log Auto-Scroll & Copy**:
   The web console log pauses auto-scrolling when the user scrolls up, displays a floating resume button, and includes a one-click clipboard copy button.

---

## 6. Recommended Future Roadmap

The following major roadmap milestones from previous versions are now **fully completed**:
- ✅ **Kodi / Jellyfin NFO & Media Asset Exporter**: Generated automatically in standardized folders.
- ✅ **Persistent SQLite Database**: Stores user states, ratings, favorites, and organized status.
- ✅ **NAS Organizer Pipeline**: Automatic atomic move/copy with multi-part consolidation.
- ✅ **Operation History & Audit Trail**: SQLite-backed with auto-retention and Web UI viewer.

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
