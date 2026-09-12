# 🤝 Engineering Handoff — R19DEV Scraper

## 1. Executive Summary

**Project Name**: `r19dev-scraper`  
**Current Version**: `v1.6.0`  
**Language / Runtime**: Go 1.24+ (`go 1.27.0` toolchain)  
**Primary Function**: High-performance local media library scanner, intelligent JAV filename parser, R18.dev metadata scraper, single-binary 100% offline-ready Web UI Studio with Native ES Modules, 2-Column Bento Profile & Filmography Stage, Universal Search & Quick Navigation, interactive Terminal TUI, and automated NAS Jellyfin organizer with SQLite audit trail.

The codebase is clean, thoroughly tested (100% test pass rate across all packages), modular, and fully documented.

---

## 2. Current Health & Status

| Area | Status | Notes |
|---|---|---|
| **Compilation** | ✅ Passing | Single-binary compilation via `go build -o bin/r19dev ./cmd/r19dev` |
| **Unit Tests** | ✅ Passing | 100% pass rate across `pkg/scanner`, `pkg/matcher`, `pkg/scraper`, `pkg/jellyfin`, `pkg/organizer`, `pkg/actress`, `pkg/cache`, `pkg/db`, and `pkg/web` |
| **Frontend** | ✅ Modular | Native ES Modules in `pkg/web/static/js/`, zero Node.js/npm dependencies, 100% offline-ready |
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
│   └── static/           -> Static web assets (embedded via embed.FS)
│       ├── fonts/        -> Local offline fonts (Inter, JetBrains Mono, Material Symbols)
│       ├── js/           -> Native ES modules (state, api, modal, scanner, organizer, history, actress, app)
│       ├── vendor/       -> Offline vendor bundles (Lucide, PhotoSwipe 5)
│       ├── index.html    -> Semantic dark-mode HTML shell (zero CDN links)
│       └── style.css     -> CSS Design system with local @font-face rules
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
14. **Local SQLite in Application Support & NAS Auto-Backup**:
    Database file `r19dev.db` resides in `~/Library/Application Support/r19dev/r19dev.db` (macOS) via `os.UserConfigDir()`, safe from cache cleaner sweeps, with automatic boot migration from legacy `~/Library/Caches`. Active SQLite remains on local SSD to avoid SMB (`smbfs`) WAL `.db-shm` and Darwin `fsctl` limitations, while atomic `VACUUM INTO` snapshots (`.r19dev_backup.db`) are automatically created on the NAS organized root after organize completion. A dedicated `/api/db/backup?download=1` endpoint and UI button enable instant downloads, and missing databases auto-restore from NAS backups.
15. **Git Exclusion Invariant**:
    Root `.gitignore` strictly ignores `*.db`, `*.db-shm`, and `*.db-wal`. The database is purely local, never committed to Git.
16. **Universal Search & Fast Keyboard Navigation**:
    Sticky top header includes a centered glassmorphic search input with context-aware routing (Library files/SKU, Followed Actress Directory, or Active Filmography stage). Global keyboard shortcuts `⌘K` / `Ctrl+K` and `/` instantly focus search, with `Esc` clearing or blurring.
17. **Sticky Breadcrumb & Floating Quick Navigation**:
    Top fixed navbar contains an interactive breadcrumb (`[← All Actresses] / {Actress Name}`). In long filmography views, a floating glassmorphic pill (`[← All Actresses] | [↑ Top]`) auto-reveals on scroll $> 300\text{px}$. Browser history (`history.pushState` & `popstate`) is seamlessly integrated for hardware back buttons and trackpad swipe gestures.
18. **Multi-Actress Group Work Prioritization & Zero Storage Waste**:
    When organizing group or crossover releases, `pkg/organizer` inspects followed actresses in SQLite and prioritizes placing the physical directory under the tracked performer. Only 1 physical file instance exists on the NAS (0 duplicate bytes). The title is dynamically linked across Jellyfin NFO `<actor>` tags and Web UI collections for all co-stars simultaneously.
19. **Extended Filmography Gatekeeping & Canonical SKU Preference**:
    Automated gatekeeper excludes AI Remasters (`JQRE-`, `AIリマスター`, `復刻`), variety talk shows (`KCKC-`, `MLTN-`), and omnibus clip compilations (`BMW-`, `REbecca STARS`, $\ge 10$ performers). Smart deduplication prioritizes canonical maker disc codes over streaming re-releases (e.g. `PPPD-485` over `PPP-485`, `BOMN-169` over `BOM-169`).
20. **Database Null-Safety & DMM Content ID Fallback**:
    `GetMovie` in `pkg/db/db.go` uses SQL `COALESCE(dvd_id, id)` and `sql.NullTime` scanning. This prevents database driver scan errors for physical releases on DMM/R18.dev where `dvd_id` is null (such as `EBDB-998`), gracefully falling back to `combined_id`.
21. **100% Offline-Ready Architecture (Zero CDN Reliance)**:
    Bundled local `.woff2` font files in `pkg/web/static/fonts/` (`inter-variable.woff2`, `jetbrains-mono-latin.woff2`, `material-symbols-outlined.woff2`) with local `@font-face` definitions in `style.css`. All external Google Fonts CDN links are removed from `index.html`.
22. **Native Browser ES Modules (Zero Build Step)**:
    Frontend refactored from a monolithic 3,500-line script into 8 single-responsibility ES modules under `pkg/web/static/js/` (`state.js`, `api.js`, `modal.js`, `scanner.js`, `organizer.js`, `history.js`, `actress.js`, `app.js`). Runs natively via `<script type="module" src="/js/app.js"></script>` without Node.js or npm.
23. **Balanced 2-Element Directory Card & Skipped Filmography Audit**:
    Redesigned the actress directory cards with a compact status pill + monospace release date to prevent text clipping. Added a dedicated `Skipped` sub-filter tab on the actress filmography stage showing excluded titles with reasons.

---

## 6. Recommended Future Roadmap

The following major roadmap milestones from previous versions are now **fully completed**:
- ✅ **Kodi / Jellyfin NFO & Media Asset Exporter**: Generated automatically in standardized folders.
- ✅ **Persistent SQLite Database**: Stores user states, ratings, favorites, and organized status.
- ✅ **Application Support Migration & NAS Auto-Backup**: Resilient local storage with automated NAS snapshots.
- ✅ **NAS Organizer Pipeline**: Automatic atomic move/copy with multi-part consolidation.
- ✅ **Operation History & Audit Trail**: SQLite-backed with auto-retention and Web UI viewer.
- ✅ **Interactive Actress Chat UI & Dual View**: Chat timeline mode and Movie Collection showcase.
- ✅ **Slide-Over Profile Drawer**: Stats grid, collection progress bar, and verified R18.dev links.
- ✅ **Native Finder Integration**: Instant reveal in macOS Finder / OS file manager.
- ✅ **Organized Library Default**: Standardized destination at `/Volumes/home/BT/organized`.
- ✅ **Safe Metadata Upserts & Test DB Isolation**: Guarantees zero production database corruption.
- ✅ **Multi-Tier Image Serving Pipeline**: Complete resilience with disk fallback.
- ✅ **Universal Search & Keyboard Shortcuts**: Header search bar with `⌘K` / `Ctrl+K` and `/`.
- ✅ **Sticky Breadcrumbs & Floating Quick Navigation**: Breadcrumb in navbar and floating navigation pill.
- ✅ **Multi-Actress Group Work Prioritization**: Followed performer folder priority with zero storage waste.
- ✅ **Extended Clean Filmography Gatekeeping**: Excludes AI remasters, talk shows, and omnibus compilations.
- ✅ **DMM Content ID Fallback & DB Null-Safety**: Robust scanning handling null `dvd_id` without errors.
- ✅ **100% Offline-Ready UI (Zero CDN Reliance)**: Bundled local WOFF2 fonts and icons.
- ✅ **Native ES Modules Frontend**: Modular JavaScript with zero Node.js/npm dependencies.
- ✅ **Actress Directory Card Redesign & Skipped Filmography Audit**: 2-element layout and auditable exclusions.

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


