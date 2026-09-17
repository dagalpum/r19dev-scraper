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
│   │   ├── dump.go           # Tier-1 Offline Dump Store (sub-millisecond SQLite queries against r18_dump.db)
│   │   ├── dump_test.go      # DumpStore unit tests
│   │   ├── client_offline_test.go # End-to-end offline scraping tests
│   │   ├── r18dev.go         # HTTP client & Tier-1 dump fallback communicating with R18.dev API
│   │   └── r18dev_test.go    # Unit tests for ID normalizer
│   ├── db/                   # Module 4: SQLite Database & Audit Trail
│   │   ├── db.go             # SQLite engine (pure Go modernc.org/sqlite), schema, migrations, auto-pruning
│   │   └── db_test.go        # Tests for user states, movies, library files, and operation history
│   ├── organizer/            # Module 5: NAS Directory & Jellyfin Organizer
│   │   ├── organizer.go      # Move/Copy planning, multi-part handling, granular step progress reporter
│   │   └── organizer_test.go # Plan & execution tests with dry-run verification
│   ├── jellyfin/             # Module 6: Jellyfin Metadata & Media Assets
│   │   ├── nfo.go            # Kodi/Jellyfin NFO XML generator & 180-byte safe filename sanitizer
│   │   ├── html.go           # Cinematic offline-first HTML viewer (backdrop hero, embedded player, lightbox, multi-part)
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
│   │   └── static/           # Embedded SPA assets
│   │       ├── fonts/        # Local offline WOFF2 fonts (Inter, JetBrains Mono, Material Symbols)
│   │       ├── js/           # Native ES Modules (state, api, modal, scanner, organizer, history, actress, graph, app)
│   │       ├── vendor/       # Offline vendor bundles (Lucide icons, PhotoSwipe 5)
│   │       ├── index.html    # Semantic dark-mode HTML shell (zero external CDN links)
│   │       └── style.css     # CSS Design system with local @font-face rules
│   ├── migrator/             # Module 11: High-Speed Batch Migrator & Safe Reorganizer
│   │   ├── engine.go         # 5-phase migration pipeline, smart collision protection, concurrent HTML upgrader
│   │   ├── engine_test.go    # Tests for cleanEmptyTree and concurrent UpgradeHTMLFiles
│   │   ├── tui.go            # Interactive Charm Bubble Tea TUI with real-time progress bar & log pane
│   │   ├── cli.go            # Headless non-interactive CLI runner for scripts and servers
│   │   └── types.go          # Event definitions, config, and migration summary types
│   ├── audit/                # Module 13: Quality Auditor & Doctor
│   │   ├── auditor.go        # Fast image header decoder, aspect ratio validation, multi-part checker, and auto-healer
│   │   ├── auditor_test.go   # Unit tests for inspection rules and asset repair
│   │   └── tui.go            # Interactive Terminal TUI Doctor (filtering, live status, batch fix)
│   └── tui/                  # Module 12: Interactive Terminal Dashboard
│       ├── app.go            # Bubble Tea Model (Init, Update, async Cmd handlers)
│       ├── views.go          # View layout: Split screen (file table + metadata inspector)
│       ├── edit_modal.go     # Textinput modal for manual ID override
│       ├── keys.go           # Key bindings definition
│       └── styles.go         # Lip Gloss theme tokens and palette
├── .gitignore                # Go build artifact & database ignore rules
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

### 4.2 Tier-1 Offline Dump Store & Cloudflare Immunity
* **Problem Solved**: High-volume metadata queries against the public R18.dev REST API frequently trigger Cloudflare HTTP 429 rate limits and error 1015 IP bans.
* **Architecture**: The `pkg/scraper/dump.go` module implements `DumpStore`, opening a local read-only SQLite database `r18_dump.db` parsed from the official R18.dev weekly PostgreSQL dumps (`https://r18.dev/dumps`).
* **Indexed Coverage**:
  - `r18_movies`: 1,902,762 videos indexed on `content_id`, `dvd_id`, and `clean_id`.
  - `actresses`: 101,906 performers with Romaji and Kanji names.
  - `video_actresses`: 2,480,384 video-actress links.
  - `translations`: 540,500 DeepL translations (`source_ja` $\rightarrow$ `target_en`). Over 97% of the library is converted to English completely offline.
* **Latency**: Resolves queries in **`< 1ms`** (0.00s in Go tests), populating the Tier 0 disk cache without initiating any external network connections.
* **Tiered Fallback**:
  $$\text{Cache (Tier 0)} \longrightarrow \text{DumpStore (Tier 1)} \longrightarrow \text{Live HTTP API (Tier 2)}$$

### 4.3 3-Tier Navigation Architecture (User Journey Segregation)
* **Tab 1: 📥 Incoming**: Focused entirely on ingestion. Scans unorganized directories, groups multi-part files, checks SKUs, and launches the Jellyfin organizer drawer.
* **Tab 2: 👤 Actresses**: Dedicated to performer collection tracking. Sub-tabs cleanly separate `Followed` (collection completion %, career span, backlog wishlist) from `Unfollowed` (performers detected in local files available for 1-click follow).
* **Tab 3: 🎬 Library**: Complete media catalog of all titles organized and ready to watch on NAS. Features status filter pills (`All Works`, `In Library`, `Missing`, `Watched`, `Favorites`), multi-criteria sorting (`Release Date`, `User Rating`, `Studio/Maker`, `JAV ID`, `Title`), and dropdown filters (`Genre`, `Studio`, `Actress`).

### 4.4 English Metadata Hierarchy & 180-Byte Filesystem Limits
* **Naming Convention**: Folder structure follows `<Dest>/<Actress_Name>/<JAV-ID Title>/`. Actress name and movie title prioritize English metadata, falling back to Japanese only when English is absent.
* **ENAMETOOLONG Prevention**: Single directory components are capped at $\le 180$ bytes along UTF-8 rune boundaries, preventing OS filesystem `ENAMETOOLONG` errors (255-byte limit on APFS, ext4, NTFS, and SMB shares).

### 4.5 SQLite Storage Invariant, Application Support & NAS Auto-Backup
* **Engine & Path**: All application data is managed via pure Go SQLite (`modernc.org/sqlite` with WAL mode & busy timeout) stored in `~/Library/Application Support/r19dev/r19dev.db` (macOS) or `~/.config/r19dev/r19dev.db` (Linux) via `os.UserConfigDir()`, protected from OS cache-cleaners.
* **Seamless Migration**: On startup, legacy databases from `~/Library/Caches/r19dev/r19dev.db` are automatically migrated to `Application Support`.
* **SMB Invariant & NAS Auto-Backup**: SQLite directly on SMB network shares (`smbfs`) suffers from lack of `.db-shm` `mmap` and missing Darwin `fsctl` support. Therefore, active SQLite is strictly kept on the local SSD, while atomic, defragmented snapshots (`.r19dev_backup.db`) are created on the target NAS directory using `VACUUM INTO` (via local temp file copy) after organize operations.
* **Strict Git Exclusion Invariant**: All databases (`r19dev.db`, `r18_dump.db`), write-ahead logs, and dump files live outside the workspace and are ignored via `.gitignore` (`*.db`, `*.db-shm`, `*.db-wal`, `*.sql`, `*.sql.gz`, `dumps/`), guaranteeing zero database leaks into Git.
* **Audit Trail**: All organize and scrape operations are logged into the `operation_history` table in SQLite. Automated pruning keeps only the last 100 entries and purges logs older than 30 days, guaranteeing zero disk clutter.

### 4.6 Symlink Protection & Boundary-Safe Regexes
* The scanner executes `os.Lstat()` on every node; symlinks are filtered out to guarantee immunity from circular loops.
* Regex matching uses boundary assertions `(?:^|[^a-zA-Z0-9])` instead of standard `\b` to avoid splitting on underscores.

### 4.5 Actress Hub: 2-Column Bento UI & Dedicated Filmography Stage
* **2-Column Bento Architecture**: Individual actress view splits into:
  - **Left Sticky Bento Sidebar (~340px)**: Large HD avatar, Romaji/Kanji names, R18 ID badge, career span (`YYYY – YYYY`), real-time NAS storage highlight in GB (`TotalSizeBytes`), completion progress bar (`x/total (pct%)`), clickable top genre tags (`TopGenres`), and native Finder/R18.dev quick action buttons.
  - **Right Filmography Main Stage**: In-page live search input (filter instantly by ID/title), sub-filter pills (`All Works`, `In Library`, `Missing`), active genre tag indicator with 1-click removal, multi-key sort dropdown (Date Newest/Oldest, Size, ID), and uniform responsive poster grid.
  - **Responsive Layout**: Media query `@media (max-width: 960px)` stacks the sidebar on top of the stage for tablets/mobile screens.
* **Multi-Layer Gatekeeper for Genuine Solo Releases**: Automated filtering removes compilation titles (総集編, BEST, BOX), photobooks, duplicate SKU formats (BOD, 9SNOS, K9SNOS), talk shows (`KCKC-`, `MLTN-`), AI Remaster re-issues (`JQRE-`, `AIリマスター`, `復刻`), and omnibus clip compilations (`BMW-`, `REbecca STARS`, $\ge 10$ performers), retaining 100% clean solo works.
* **Official R18 Actress URLs**: Links to `https://r18.dev/videos/vod/movies/list/?id={r18_id}&type=actress` avoiding the non-existent `/search/` route on R18.dev.
* **Database Backfill**: `initSchema` executes `ALTER TABLE actresses ADD COLUMN r18_id INTEGER DEFAULT 0;` and automatically triggers `backfillActressR18IDs()` on startup, extracting R18 actress IDs from cached `movies.actresses_json`.

### 4.6 Followed Actresses Directory & Native Finder Integration
* **Directory Grid**: Clean actress cards with minimalist 6px progress track `${dl}/${total} (${pct}%)`, search input, and multi-sort dropdown (`% Completed`, `Most Missing`, `Name A-Z`, `Total Works`).
* **Native Finder Controls**: Direct endpoints (`POST /api/open-folder`) trigger native OS file managers (`open` on macOS Finder, `explorer` on Windows, `xdg-open` on Linux) to reveal exact actress or title folders under `/Volumes/home/BT/organized`.

### 4.7 Multi-Tier Image Serving Architecture (`/api/images/{id}`)
* **Tier 1 (RAM Cache)**: Instant lookup in `cache.Default().GetImage(id)`.
* **Tier 2 (Local Disk)**: Reads `poster.jpg`, `fanart.jpg`, or `cover.jpg` from the organized directory (via `organized_movies` or directory glob `/Volumes/home/BT/organized/*/*{id}*`), caching in RAM.
* **Tier 3 (Network Scrape Fallback)**: Automatically downloads the jacket cover via upgraded DMM URLs using proper browser referrers, caches in RAM, and serves seamlessly.

### 4.8 Safe DB Updates & Test Environment Isolation
* **Safe Upserts**: `SaveMovie` employs SQL `CASE WHEN excluded.<field> != '' THEN excluded.<field> ELSE movies.<field> END` to ensure partial movie records never erase existing cover URLs, titles, or metadata arrays.
* **Test Isolation**: `organizer.SetDB(testDB)` and `web.Config{DB: testDB}` allow test suites to run against temporary SQLite databases without polluting `~/Library/Application Support/r19dev/r19dev.db`.

### 4.9 Universal Search & Keyboard Navigation
* **Sticky Navbar Search**: Centered glassmorphic search input in the fixed header with context-aware routing:
  - Library tab: Searches library files and SKUs.
  - Actresses directory: Filters followed performers in real-time.
  - Actress stage: Filters filmography titles and IDs.
* **Global Keyboard Shortcuts**: `⌘K` (macOS) / `Ctrl+K` (Windows/Linux) and `/` (when browsing) focus the search input; `Esc` clears or unfocuses.
* **Deep Navigation & History**: Sticky breadcrumbs (`[← All Actresses] / {Actress Name}`), floating quick navigation pill (`[← All Actresses] | [↑ Top]` appearing after scrolling $> 300\text{px}$), and full `history.pushState` integration supporting trackpad gestures and hardware back buttons.

### 4.10 Multi-Actress Group Work Prioritization & Zero Storage Duplication
* **Physical Directory Placement Priority**: When organizing group or crossover releases with multiple co-stars, `pkg/organizer` queries SQLite to determine which actresses are followed. The physical folder is placed under the followed performer (avoiding placing works under untracked co-stars). If multiple co-stars are followed, the first alphabetically or primary tracked performer is selected.
* **Zero Storage Waste (Single Physical Instance)**: Files reside strictly in a single physical location on the NAS (0 duplicate bytes).
* **Multi-Library Discovery**: Jellyfin NFO contains all `<actor>` tags, and R19DEV Studio records the path in `organized_movies`, allowing the title to be discovered and linked under all participating actresses across both platforms simultaneously.

### 4.11 Extended Filmography Gatekeeping & Canonical SKU Preference
* **Canonical Disc Code Preference**: Deduplication engine favors standard maker disc codes over streaming outlet re-releases (e.g. `PPPD-485` preferred over `PPP-485`, `BOMN-169` over `BOM-169`).
* **AI Remaster & Re-issue Filter**: Excludes duplicate re-issues bearing `JQRE-` prefixes or keywords like `AIリマスター`, `デジタルリマスター`, `復刻`, or `名作`.
* **Omnibus Series Filter**: Excludes clip compilations (e.g. `BMW-` series from Wanz Factory, `REbecca STARS`, and works featuring $\ge 10$ performers) from solo performer filmographies.

### 4.12 DMM Content ID Fallback & Database Null-Safety
* **Content ID vs DVD ID Distinction**: On DMM/R18.dev, physical goods (such as REbecca Blu-rays, e.g. `EBDB-998` $\rightarrow$ `h_346rebdb998`) have `content_id` set but `dvd_id = null`.
* **Null-Safe Scanning**: `GetMovie` in `pkg/db/db.go` scans using SQL `COALESCE(dvd_id, id)` and `sql.NullTime` for timestamps, preventing driver conversion errors.
* **Graceful Scraper Fallback**: If standard `dvd_id` lookup yields a 404 on R18.dev, the engine falls back to `combined_id` resolution without application errors.

### 4.13 100% Offline-Ready Architecture (Zero CDN Reliance)
* **Embedded WOFF2 Assets**: All web typography and iconography are bundled locally in `pkg/web/static/fonts/`:
  - `inter-variable.woff2` (Inter variable 100–900)
  - `jetbrains-mono-latin.woff2` (JetBrains Mono Latin)
  - `material-symbols-outlined.woff2` (Google Material Symbols Outlined full glyph set)
* **Local `@font-face` Invariant**: `style.css` resolves all fonts from local absolute paths (`/fonts/...`). All external Google Fonts CDN links, preconnect directives, and remote stylesheets are eliminated from `index.html`.
* **Isolated Environment Resilience**: UI functions 100% identically with zero broken glyphs or raw font fallbacks on completely air-gapped home labs or offline NAS devices.

### 4.14 Native ES Modules Architecture (Zero Build Step)
* **Browser-Native `import` / `export`**: Frontend is cleanly divided into 8 single-responsibility ES modules under `pkg/web/static/js/`:
  - `state.js`: Global reactive application state, DOM cache, sanitization, and toast notifications.
  - `api.js`: REST/SSE clients, scraping, user state updates, folder opening, actress operations.
  - `modal.js`: Full-width hero modal, PhotoSwipe 5 dynamic loader, ratings, and lightbox.
  - `scanner.js`: Media discovery stream, multi-part grouping, grid density, sorting, and directory rescan.
  - `organizer.js`: Jellyfin organizer stream, terminal log console, and auto-scroll controller.
  - `history.js`: Operation history modal and SQLite audit log inspection.
  - `actress.js`: Actress Hub (Followed/Discovered directory, Bento profile, dedicated filmography stage, and Chat mode).
  - `app.js`: Application bootstrap, tab routing, and `window.app` public interface binding.
* **Single-Binary Zero-Node Philosophy**: No Node.js runtime, npm dependencies, or Webpack/Vite bundler steps required. Browsers execute ES modules natively via `<script type="module" src="/js/app.js"></script>`, maintaining the pure `go build` single-binary distribution.

### 4.15 Cinematic Offline-First `movie.html` Architecture
* **Ambient Backdrop Hero**: Features full-width `fanart.jpg` backdrop with 30px CSS blur and dark linear gradient overlay. Falls back smoothly to blurred `poster.jpg` if `fanart.jpg` is absent.
* **Direct Play Action Bar**: Prominent `▶ Play Movie` button launches the file in the OS default video player (VLC, IINA, QuickTime). The `🖥️ Watch in Browser` button toggles a pop-up HTML5 `<video controls>` player modal directly within the web browser.
* **Smart Multi-Part Video Detection**: Automatically identifies multi-part video sets (e.g. `-pt1.mp4`, `-pt2.mp4` or `-cd1`, `-cd2`) and renders dedicated `▶ Play Part 1` and `▶ Play Part 2` buttons.
* **In-Page Lightbox Gallery**: In-page modal with zero external dependencies; replaces jarring new-tab image links with a full-screen carousel supporting `←` / `→` arrow keys, `Esc` to close, and an image counter.
* **Local Asset Auto-Discovery**: Automatically inspects the local `extrafanart/` folder and video files on disk, ensuring 100% complete rich media galleries and multi-part play buttons even when using offline dump records that lack online screenshot URLs.

### 4.16 High-Speed SMB Network Traversal Optimization
* **Direct Directory Pruning**: The directory crawler in `pkg/scanner/scanner.go` evaluates `d.IsDir()` immediately upon entering directory traversal and executes `filepath.SkipDir` for `.actors`, `extrafanart`, `@eaDir`, and hidden dot folders (`.`) without executing redundant `os.Lstat` syscalls.
* **NAS Performance Impact**: Completely eliminates network SMB latency bottlenecks, speeding up scans of archives with thousands of asset images from timeouts down to seconds.

### 4.17 Tokuten Promotional SKU Deduplication & Omnibus Gatekeeping
* **Tokuten Goods Bundle SKUs (`TK-` Prefixes)**: DMM/FANZA prefixes `TK` to studio disc codes (e.g. `TKCJOD-510`, `TKMFYD-123`, `TKCAWB-040`) for limited bundle packages that include Cheki photos (チェキセット) or raw photographic prints (生写真). The video content is 100% identical to the primary catalog release.
* **Dual-Language Filter Inspection**: `CheckFilmographyInclusion` in `pkg/actress/service.go` inspects both English machine-translated `Title` and Japanese `OriginalTitle` simultaneously, detecting Japanese bundle keywords (`チェキセット`, `生写真`, `【FANZA限定】`) and compilation markers (`\d+連発`, `\d+連射`, `\d+時間BOX`, `ベストセレクション`).
* **Omnibus Clip Compilations (`RBB-` Series)**: Added explicit detection for Rookie Best Box / REbecca Best Box (`RBB-`) and multi-actress clip compilation series.
* **Canonical Base ID Grouping**: `deduplicateReleases` strips promotional prefixes (`TK`) and suffixes (`-EC`), penalizing promotional SKUs with a `-100` score so standard canonical releases (`CJOD-510`, `MFYD-123`) always emerge as master titles.

### 4.18 100% Deterministic Offline R18 Outbound Navigation & Verified IDs
* **Authentic DMM Content IDs**: Rather than relying on naive algorithmic padding (which produces 404s for SOD/Faleno/Dahlia titles prepended with `1`, e.g. `1start223`, `1fsdss685`), `r19dev.db.movies.combined_id` is matched against `r18_dump.db.r18_movies.content_id`. 100.0% of library releases match verified records.
* **Direct Detail URL Syntax**: Direct links use `https://r18.dev/videos/vod/movies/detail/-/id={content_id}/` (HTTP 200 OK), replacing legacy `combined={content_id}` syntax (which yielded HTTP 404).
* **Performer Profile Disambiguation**: 100% of followed actresses in `r19dev.db.actresses` have their verified DMM `r18_id` populated (e.g. Nao Satsuki ID `1089946`, Sayaka Nakamura ID `1094001`), eliminating homonymous collisions on older performers.
* **Adaptive Card Actions**: On filmography poster cards, hover action buttons adapt dynamically: local library items show `[📂 Finder]`, while unowned/missing releases present `[🌐 R18 ↗]` for 1-click preview.
* **100% Offline-First Invariant**: All outbound links are client-side `<a target="_blank">` hyperlinks, requiring zero external server-side requests and immune to Cloudflare HTTP 429 rate limits.

### 4.19 Batch Migration Engine & Smart Collision Protection (`pkg/migrator`)
* **Problem Solved**: Reorganizing and migrating hundreds of gigabytes (or terabytes) of existing downloads into standardized Jellyfin structures across network SMB storage without destructive overwrites, data loss, or long UI freezes.
* **5-Phase Pipeline**:
  1. *Discovery & Pre-flight*: Crawls source directory, resolves metadata via `r18_dump.db`, aggregates multi-part CDs (`-cd1..-cd5`), and computes target paths.
  2. *Pre-flight Safety Gate*: Pauses execution in TUI/CLI mode, presenting a verified plan card (`✔ Pre-Flight Complete: X movies ready`) for operator confirmation before touching files.
  3. *Live Migration*: Renames/moves video files, merges folder assets (`poster.jpg`, `fanart.jpg`, `extrafanart/`), writes Jellyfin `.nfo` and Cinematic `movie.html`, and registers entries into SQLite `movies`, `organized_movies`, and `library_files`.
  4. *Concurrent HTML Upgrader*: Employs a 16-worker goroutine pool to refresh `movie.html` templates across destination folders in seconds, with fast database discovery bypassing slow SMB directory walks.
  5. *Safe Source Pruning*: Bottom-up tree cleanup removes only empty directories via `isDirEmpty`, strictly preserving non-empty folders containing skipped or foreign media.
* **Smart Collision Protection**:
  - Automatically identifies existing target movies and prevents destructive overwrites.
  - Distinguishes and co-locates 4K editions (`-4k.mp4`), uncensored editions (`-uncensored.mp4`), and standard editions (`.mp4`) side-by-side in the same movie folder.
  - Generates HTML player action buttons supporting multiple resolutions and parts.

### 4.20 Atomic Crash-Consistent Database Backup Pipeline (`VACUUM INTO`)
* **Problem Solved**: SQLite databases located on SMB/NFS network mounts suffer from file-locking latency and corruption risks if backed up via naive file copies while write transactions are active.
* **Architecture**: `DB.BackupTo` executes SQLite's atomic `VACUUM INTO` command into a local temporary file first on fast local NVMe storage, defragmenting database pages and ensuring transaction consistency without holding long database locks.
* **Cross-Filesystem Safe Copy**: The clean, defragmented backup file is copied atomically to the destination path (e.g. `/Volumes/home/BT/organized/.r19dev_backup.db`). Accessible via CLI command `./bin/r19dev backup [path]` or Web UI history modal.

### 4.21 Self-Healing Avatar Caching for Discovered / Unfollowed Actresses
* **Problem Solved**: Unfollowed performers appearing in library titles lacked pre-cached avatar images in `~/Library/Application Support/r19dev/actress_images/`, causing generic SVG initials to render on the Unfollowed tab.
### 4.22 Configurable Filter Engine & Compilation Purge Pipeline (`pkg/scraper/filter.go`)
* **Problem Solved**: Omnibus releases, promotional variants (`TK-`, `-EC`), goods bundles, and studio compilations (e.g. `IPOK-`, `IDBD-`, `MIZD-`, `MIDD-`, `PBD-`, `OBST-`, `SDDE-`, `RBB-`, `MKCK-`, `MKMP-`, `OFJE-`, `SETH-`, `OFRF-`, `OFMA-`, or labels like `Idea Pocket BEST`, `MOODYZ Best`, `PREMIUM BEST`, `SOD BEST`, `Madonna BEST`) previously polluted solo performer filmographies and library listings if hardcoded rules missed new variations or titles like `100本番`.
* **Dynamic Configuration (`filters.json`)**:
  - Filter rules are managed via a JSON schema stored at `~/Library/Application Support/r19dev/filters.json` (macOS) or `~/.config/r19dev/filters.json` (Linux).
  - Configurable arrays: `blocked_prefixes`, `blocked_labels`, `blocked_series`, `blocked_genres`, `blocked_title_keywords`, `blocked_title_regex`, `blocked_cover_patterns`, `min_actress_omnibus_count`, `max_duration_minutes_threshold`.
  - Automatically created with robust defaults on first startup. Hot-reloaded without requiring Go recompilation.
* **Database Integration & Series Schema**:
  - `movies` table schema expanded with `series TEXT` column and automated startup migration (`ALTER TABLE movies ADD COLUMN series TEXT;`).
  - `SaveMovie` checks `IsPromotionalOrOmnibusVariantWithDetails(m.ID, m.Title, m.OriginalTitle, m.Label, m.Series, m.CoverURL, m.Genres, len(m.Actresses))`.
  - `PurgePromotionalVariants` executes both direct SQL elimination of known compilation patterns and a dynamic sweep against the active `FilterConfig`, safely deleting unowned excluded titles while strictly preserving any media present on the user's disk or marked as watched/favorite.
* **CLI & Web REST API**:
  - CLI: `r19dev filters show`, `r19dev filters path`, `r19dev filters purge`, `r19dev filters reset`.
  - REST API: `GET /api/filters`, `POST /api/filters`, `POST /api/filters/reset`, `POST /api/filters/purge`.

### 4.23 Post-Migration Verification & Deep Audit Pipeline (`pkg/migrator`, `pkg/audit`)
* **Two-Tier Verification Philosophy**:
  - **Tier 1 (Lightweight Assertion - Default)**: Inline zero-overhead check asserting destination video file exists, is non-zero, matches source byte size, and `.nfo`/`movie.html` sizes $> 0$.
  - **Tier 2 (Deep Asset Audit & Auto-Heal - Opt-in via `--audit` / `-a` / `--heal`)**: Following migration, `auditor.InspectFolder` concurrently inspects all organized targets for genuine Full HD `poster.jpg` (> 25KB), `fanart.jpg`, and `extrafanart/` sample screenshots. Missing/corrupted assets are auto-healed via `auditor.FixMovie` by scraping provider CDNs directly.
* **TUI & CLI Event Pipeline**: Emits `EventAuditStart`, `EventAuditProgress`, `EventAuditHealed`, and `EventAuditDone`, rendering an updated `✨ Healed: X` badge counter in TUI.

### 4.24 Multi-Studio Combined ID Candidate Generator & Online Fallback (`pkg/scraper/normalizer.go`)
* **Studio Prefix Mapping Matrix**: DMM uses non-standard numerical prefixes for specific labels:
  - Prestige (`ABP`, `ABW`, `ONEZ`, `EZD`): Prefix `118` (e.g. `ABP-966` $\rightarrow$ `118abp00966`, `118abp0966`, `abp00966`).
  - SOD Create (`DLDSS`, `MIST`, `KMHR`): Prefix `1` (e.g. `DLDSS-077` $\rightarrow$ `1dldss00077`, `dldss00077`).
  - VR Studios (`KAVR`, `SIVR`, `VRTM`): Prefix `13` (e.g. `KAVR-403` $\rightarrow$ `13kavr00403`, `kavr00403`).
  - Wanz Factory / Premium (`PPPD`, `PRED`, `EBOD`): Suffix `so` or standard zero-padding (e.g. `PRED-224` $\rightarrow$ `pred00224`, `h_068pred00224`).
* **Candidate Resolution Pipeline**: `CandidateCombinedIDs(id)` generates all plausible DMM content IDs in priority order. `Scrape` in `pkg/scraper/r18dev.go` tries candidate queries sequentially against both `r18_dump.db` and live R18.dev REST APIs before declaring a title unmatched.

### 4.25 High-Res Image Cache Priority Invariant (`pkg/web/server.go`)
* **Priority Invariant**: `/api/images/{id}` prioritizes genuine local disk `poster.jpg` / `fanart.jpg` from the organized directory over stale disk cache (`~/Library/Caches/r19dev/images/`).
* **Thumbnail Purge & High-Res Threshold**: Rejects cached thumbnails $< 20\text{KB}$ or $< 350\text{px}$ wide, upgrading remote DMM URLs automatically via `jellyfin.UpgradeDMMImageURL` (`ps.jpg` $\rightarrow$ `pl.jpg`, `pt.jpg` $\rightarrow$ `pl.jpg`) to ensure full-width hero covers render in Full HD (800×538) without blur.

### 4.26 Cross-User NAS Migration Architecture (Multi-User SMB Mounts)
* **Problem Solved**: Handling scenarios where source media belongs to one user (e.g. User D) and the target destination library belongs to another user (e.g. User P) on Synology/Linux NAS.
* **Architectural Solutions**:
  1. *Admin `homes` SMB Share*: Mounting `smb://<nas>/homes` allows direct pathing between `/Volumes/homes/User_D/...` and `/Volumes/homes/User_P/...`.
  2. *NAS ACL Permissions*: Setting File Station Read/Write ACLs on destination folders.
  3. *Shared Media Volume*: Best practice centralization to `/Volumes/video/organized` accessible by all users and media server daemons (Jellyfin/Emby).
* **Streaming Fallback Resilience**: `moveFile` / `movePath` automatically detects cross-device mount link errors (`EXDEV`) and falls back to atomic stream copy + source deletion after destination byte-size assertion.

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

# Filter Management
./bin/r19dev filters show
./bin/r19dev filters purge
./bin/r19dev filters reset
```

