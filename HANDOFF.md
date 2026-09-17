# 🤝 Engineering Handoff — R19DEV Scraper

## 1. Executive Summary

**Project Name**: `r19dev-scraper`  
**Current Version**: `v2.0.0`  
**Language / Runtime**: Go 1.24+ (`go 1.27.0` toolchain)  
**Primary Function**: High-performance local media library scanner, intelligent JAV filename parser, R18.dev metadata scraper with Tier-1 sub-millisecond offline dump store, single-binary 100% offline-ready Web UI Studio with Native ES Modules, 3-Tier Navigation Architecture (Incoming, Actresses, Library), 2-Column Bento Profile & Filmography Stage, Universal Search & Quick Navigation, interactive Terminal TUI, high-speed Batch Migrator with Bubble Tea TUI, atomic crash-consistent SQLite backup pipeline, 16-worker concurrent HTML upgrader, automated NAS Jellyfin organizer with SQLite audit trail, and Cinematic Offline-First `movie.html` Interactive Viewer.

The codebase is clean, thoroughly tested (100% test pass rate across all packages), modular, and fully documented.

---

## 2. Current Health & Status

| Area | Status | Notes |
|---|---|---|
| **Compilation** | ✅ Passing | Single-binary compilation via `go build -o bin/r19dev ./cmd/r19dev` |
| **Unit Tests** | ✅ Passing | 100% pass rate across `pkg/scanner`, `pkg/matcher`, `pkg/scraper`, `pkg/jellyfin`, `pkg/organizer`, `pkg/migrator`, `pkg/actress`, `pkg/cache`, `pkg/db`, and `pkg/web` |
| **Migrator & Reorganizer**| ✅ Passing | 5-phase pipeline, Charm Bubble Tea TUI, smart collision co-location (4K/uncensored/multi-part), 16-worker HTML upgrader |
| **Library Milestone** | ✅ 6.5 TB | **868 movies / 966 video files (~6.5 TB)** organized on NAS across 3 migration phases (Sorted, Root loose, Misc) |
| **Frontend** | ✅ Modular | Native ES Modules in `pkg/web/static/js/`, zero Node.js/npm dependencies, 100% offline-ready |
| **Offline Scraper** | ✅ Sub-ms | Tier-1 `r18_dump.db` (1.9M+ movies, 101k+ actresses, 540k+ translations) resolving in `< 1ms` |
| **Dependencies** | ✅ Stable | Using standard library + `modernc.org/sqlite` (pure Go, zero CGO) + Charm packages (`bubbletea`, `lipgloss`) |
| **Performance** | ✅ Fast | Zero UI lag; asynchronous IO for disk traversal, HTTP connection pooling, and live SSE streaming |
| **Documentation** | ✅ Complete | Updated `README.md`, `OKF.md`, `CONTEXT.md`, and `HANDOFF.md` |

---

## 3. Key Components & File Map

```
pkg/
├── scanner/              -> Safe WalkDir engine with symlink detection & timeout guards
├── matcher/              -> Regex engine with boundary checks, JAV ID normalizer, multipart detector
├── scraper/              -> Domain models, Tier-1 offline DumpStore (r18_dump.db), R18.dev REST API client
├── db/                   -> Pure Go SQLite storage, migrations, auto-pruning, operation history audit trail, atomic backup
├── organizer/            -> NAS Jellyfin organization planner, multi-part merger, live progress reporter
├── migrator/             -> High-speed batch migration engine, pre-flight safety gate, Bubble Tea TUI, 16-worker HTML upgrader
├── jellyfin/             -> Kodi/Jellyfin NFO XML generator, 180-byte safe filename sanitizer, HTML viewer, asset downloader
├── actress/              -> Actress tracking service, filmography tracker, and local release comparator
├── audit/                -> Movie completeness auditor & doctor engine, Bubble Tea split-pane TUI, interactive quick-fix
├── cache/                -> Persistent disk cache for API payloads and images (~/.cache or ~/Library/Caches)
├── web/                  -> Single-binary Web UI Studio server, SSE streaming, REST API, embedded SPA frontend, self-healing avatar cache
│   └── static/           -> Static web assets (embedded via embed.FS)
│       ├── fonts/        -> Local offline fonts (Inter, JetBrains Mono, Material Symbols)
│       ├── js/           -> Native ES modules (state, api, modal, scanner, organizer, history, actress, graph, app)
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

# 6. High-Speed Batch Migrator (Interactive TUI / Headless CLI)
./bin/r19dev migrate /Volumes/home/BT/Sorted /Volumes/home/BT/organized
./bin/r19dev migrate /Volumes/home/BT/Misc /Volumes/home/BT/organized --dry-run
./bin/r19dev migrate /Volumes/home/BT/Sorted /Volumes/home/BT/organized --yes --no-tui

# 7. Atomic Database Backup Snapshot
./bin/r19dev backup /Volumes/home/BT/organized

# 8. Standalone Concurrent HTML Upgrader (16 workers)
./bin/r19dev upgrade-html /Volumes/home/BT/organized
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
    Frontend refactored from a monolithic script into 9 single-responsibility ES modules under `pkg/web/static/js/` (`state.js`, `api.js`, `modal.js`, `scanner.js`, `organizer.js`, `history.js`, `actress.js`, `graph.js`, `app.js`). Runs natively via `<script type="module" src="/js/app.js"></script>` without Node.js or npm.
23. **Balanced 2-Element Directory Card & Skipped Filmography Audit**:
    Redesigned the actress directory cards with a compact status pill + monospace release date to prevent text clipping. Added a dedicated `Skipped` sub-filter tab on the actress filmography stage showing excluded titles with reasons.
24. **Tier-1 Sub-Millisecond Offline Dump Store (`r18_dump.db`)**:
    Integrated high-performance SQLite read-only query engine (`DumpStore` in `pkg/scraper/dump.go`) parsed from official weekly R18.dev PostgreSQL dumps. Indexes 1,902,762 movies, 101,906 actresses, 2,480,384 video-actress links, and 540,500 DeepL English translations. Queries resolve in `< 1ms` completely offline, eliminating Cloudflare HTTP 429 rate limits and error 1015 IP blocks while migrating over 97% of the library to authentic English titles.
25. **3-Tier Navigation Architecture (User Journey Segregation)**:
    Segregated the application into three intuitive user journeys:
    - **📥 Incoming** (Tab 1): Ingest new downloads, scan directories, inspect SKUs, and launch the Jellyfin organizer.
    - **👤 Actresses** (Tab 2): Followed (18) vs Unfollowed (7) performer tracking with completion % and backlog.
    - **🎬 Library** (Tab 3): Standalone movie catalog ready to watch on NAS, featuring status filter pills (`All Works`, `In Library`, `Missing`, `Watched`, `Favorites`), multi-criteria sorting (`Release Date`, `User Rating`, `Studio/Maker`, `JAV ID`, `Title`), and dynamic density toggles.
26. **Strict Git Safety & Database Isolation**:
    All databases (`r19dev.db`, `r18_dump.db`), write-ahead logs, and dump archives are stored outside the Git workspace in `~/Library/Application Support/r19dev/`. Root `.gitignore` explicitly excludes `*.db`, `*.db-shm`, `*.db-wal`, `*.sql`, `*.sql.gz`, and `dumps/`. Verified 0 database files tracked or unstaged in Git.
27. **Enhanced Filmography Gatekeeping (TK- Tokuten SKUs & RBB- Omnibus)**:
    Filters out Tokuten promotional goods duplicate SKUs (`TKCJOD-510`, `TKMFYD-123`, `TKCAWB-040`) bundling Cheki photos/raw prints, Rookie Best Box (`RBB-`) omnibus series, and multi-actress compilation regexes (`\d+連発`, `\d+連射`, `\d+時間BOX`). Evaluates English and Japanese original titles simultaneously. Canonical base ID grouping penalizes promo SKUs (`-100`) so standard releases always win.
28. **100% Deterministic Offline R18 Outbound Navigation & Verified IDs**:
    Populated verified `r18_id` for all 37 followed actresses in SQLite (`r19dev.db.actresses`), guaranteeing 100% accurate profile links (e.g. Nao Satsuki ID `1089946`, Sayaka Nakamura ID `1094001`) without homonymous search collisions. Migrated `combined_id` across all 5,436 movies in `r19dev.db.movies` to authentic DMM `content_id` from `r18_dump.db` (e.g. `1start223`, `1fsdss685`, `cjod510`, `mfyd123`). Fixed `DetailURL` syntax in `pkg/scraper/dump.go` line 190 from `detail/-/combined=%s/` to `detail/-/id=%s/`.
29. **Adaptive Poster Card Actions & Missing Movie UX**:
    Poster card hover actions dynamically show `[📂 Finder]` for locally stored media, and automatically replace it with `[🌐 R18 ↗]` for unowned/missing titles. Movie detail modal replaces dysfunctional "Organize for Jellyfin" button on missing releases with primary `[🌐 View on R18.dev ↗]` button and `[📋 Copy ID]`.
30. **High-Speed Batch Migrator & Interactive Bubble Tea TUI (`pkg/migrator`)**:
    Provides an interactive live terminal interface with percentage progress bars, real-time speed calculation (`1.1/s`), live activity cards, and milestone notifications for batch library reorganizations.
31. **Zero-Destructive Smart Collision & Quality Coexistence**:
    Automatically detects existing titles in target directories and merges complementary assets without throwing destructive errors or overwriting existing media. Co-locates 4K editions (`-4k.mp4`), uncensored releases (`-uncensored.mp4`), and standard editions (`.mp4`) in the same movie directory, while preserving multi-part CDs (`-cd1.mp4` through `-cd5.mp4`).
32. **Pre-Flight Safety Gate & Zero-Lag SQLite Discovery**:
    Scans the entire source tree and maps all IDs before moving files, pausing for explicit operator confirmation (`[Enter]` / `[q]`). Leverages local SQLite `organized_movies` table lookups (`< 0.01s`) to detect existing library movies instantly without recursive SMB network scans.
33. **16-Worker Concurrent HTML Template Upgrader**:
    Refreshes `movie.html` templates across library destinations using a 16-worker goroutine pool, reducing re-rendering times across 800+ movies on NAS from several minutes down to seconds.
34. **Atomic Crash-Consistent Database Backup (`VACUUM INTO`)**:
    Solves SQLite Darwin `smbfs` file locking and lack of POSIX shared-memory support by executing `VACUUM INTO` targeting an isolated SSD temp file first, defragmenting database pages before streaming cleanly to the NAS destination as `.r19dev_backup.db`.
35. **Self-Healing Unfollowed Performer Avatar Caching**:
    On-demand avatar cache handler in `pkg/web/server.go` checks local disk cache, dynamically queries performer profile image URLs from `r18_dump.db`, and downloads authentic HD headshots from DMM CloudFront CDN upon discovery.
36. **Configurable Filter Engine & Compilation Purge Pipeline (`filters.json`)**:
    User-customizable JSON rules schema (`~/Library/Application Support/r19dev/filters.json`) allowing instant addition/removal of blocked prefixes (`IPOK`, `IDBD`, `MIZD`, `MIDD`, `PBD`, `OBST`, `SDDE`, etc.), studio labels/series (`Idea Pocket BEST`, `MOODYZ Best`, `PREMIUM BEST`), and title patterns (`100本番`, `\d+連発`). Dynamic database sweep purges unowned duplicates while strictly protecting on-disk media. Accessible via CLI (`r19dev filters [show|path|purge|reset]`) and REST API (`/api/filters`).
37. **Step-by-Step User Journeys & Extended Documentation**:
    Comprehensive user journey guide in `README.md` with visual diagrams detailing Ingest & Organize, Actress Hub & Backlog Wishlist, Library & Universal Search (`⌘K`), Custom Filter Configuration, High-Speed Batch Migration, and Database Backup.
38. **Interactive Terminal Doctor & Movie Auditor (`pkg/audit`)**:
    Split-pane Bubble Tea TUI (`r19dev audit`), comprehensive 6-point completeness checklist (video, NFO, HTML, poster, fanart, extrafanart), fast image header decoding, aspect ratio validation, 0-byte file detection, and interactive/batch auto-healing (`f` / `F`).
39. **Post-Migration Verification & Deep Audit Pipeline (`r19dev migrate --audit`)**:
    Integrates lightweight zero-overhead sanity checks (file size equality, non-empty NFO/HTML) by default, alongside opt-in deep asset auditing and automatic healing (`auditor.FixMovie`) to ensure 100% complete libraries after migration.
40. **Multi-Studio Numerical Prefix Normalizer (`CandidateCombinedIDs`)**:
    Solves non-standard DMM numerical prefixes for Prestige (`118abp00966`), SOD (`1dldss00077`), and VR (`13kavr00403`), ensuring seamless metadata lookup across all studio brands.
41. **High-Res Local Image Serving Priority & Stale Thumbnail Purge**:
    Eliminates low-resolution cover display on Web UI modals (`MFYD-123` 147x200px vs 800x538px) by prioritizing local Full HD `poster.jpg` files on disk over RAM/disk cache and enforcing a $>20\text{KB}$ quality threshold.

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
- ✅ **Tier-1 Sub-Millisecond Offline Dump Store (`r18_dump.db`)**: 1.9M+ movies, 101k+ actresses, 540k+ translations.
- ✅ **3-Tier User Journey Navigation (Incoming, Actresses, Library)**: Reorganized navigation architecture.
- ✅ **Library Catalog Hub**: Status pills (Watched, Favorites), multi-criteria sorting (Rating, Studio).
- ✅ **Cinematic Standalone `movie.html` Interactive Viewer**: Ambient backdrop hero with blur, direct `Play Movie` CTA, in-browser HTML5 `<video>` modal player, smart multi-part play buttons, in-page Lightbox gallery with keyboard navigation, one-click JAV ID copy with toast, and local asset auto-discovery.
- ✅ **High-Speed SMB Network Scanner Traversal**: Immediate directory pruning of `.actors`, `extrafanart`, `@eaDir`, and hidden directories at `d.IsDir()`, reducing scan times across large NAS archives from timeouts to seconds.
- ✅ **Web Studio Library Tab & Favicon Stability**: Fixed `TypeError` in `actress.js` and added native SVG favicon handler.
- ✅ **Batch Migrator & Interactive Bubble Tea TUI (`pkg/migrator`)**: Pre-flight safety confirmation, speed tracking, live activity cards.
- ✅ **Zero-Destructive Smart Collision & Quality Coexistence**: 4K, 1080p, uncensored, and multi-part files co-located safely.
- ✅ **16-Worker Concurrent HTML Template Upgrader**: Instant re-rendering with fast database lookup bypassing SMB walks.
- ✅ **Atomic Crash-Consistent Database Backup (`VACUUM INTO`)**: Defragmented SQLite snapshots streamed safely to NAS share.
- ✅ **Self-Healing Unfollowed Actress Avatar Caching**: Automatic DMM CDN fetch and local caching on discovery.
- ✅ **Configurable Filter Rules Engine (`filters.json`)**: User-editable exclusion rules, Label/Series omnibus purging, CLI & REST API.
- ✅ **Step-by-Step User Journeys Guide**: Complete visual guide in README.md.
- ✅ **Quality Auditor & Doctor Engine (`pkg/audit`)**: 6-point completeness verification and batch auto-healing.
- ✅ **Post-Migration Verification & Deep Auto-Heal**: Integrated in `r19dev migrate --audit`.
- ✅ **Multi-Studio Numerical Prefix Generator**: Maps Prestige (`118`), SOD (`1`), VR (`13`), and standard prefixes.
- ✅ **High-Res Local Image Serving Priority**: Prioritizes local Full HD poster files over cached thumbnails.

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


