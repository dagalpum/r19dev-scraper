# 📘 Operational Knowledge Framework (OKF) — R19DEV Scraper

## 1. System Overview & Core Objectives

**R19DEV Scraper** is a high-performance, modular CLI and Terminal User Interface (TUI) application written in Go. Its primary mission is to solve the complex challenges of organizing and discovering metadata for Japanese Adult Video (JAV) media collections.

### Core Objectives:
1. **Zero-Friction Discovery**: Quickly index large directories of video files without hanging on cyclic symlinks or stalling on non-video artifacts.
2. **Robust Pattern Recognition**: Accurately parse JAV identifiers across dozens of naming conventions, release group watermarks, tracker prefixes, VR content numbers, and multi-part designations.
3. **Seamless Metadata Enrichment**: Normalize extracted IDs into `R18.dev` API combined format and retrieve complete, structured metadata.
4. **Interactive Operator Ergonomics**: Provide a fast, keyboard-first TUI powered by Charm's Bubble Tea, allowing users to inspect files, view fetched metadata, and manually override IDs with immediate visual feedback.

---

## 2. System Architecture & Component Interactions

```mermaid
graph TD
    User([User / CLI Invocation]) --> Main[cmd/r19dev/main.go]

    subgraph Core Pipeline
        Main -->|TUI / Scan| Scanner[pkg/scanner]
        Scanner -->|[]FileInfo| Matcher[pkg/matcher]
        Matcher -->|[]MatchResult| PipelineOrUI[TUI / Scan Output]
        
        PipelineOrUI -->|Extract ID| Normalizer[pkg/scraper/normalizer]
        Normalizer -->|Combined ID| ScraperClient[pkg/scraper/r18dev]
        ScraperClient -->|HTTP GET JSON| R18Dev[(R18.dev Public API)]
        R18Dev -->|Raw Payload| ScraperClient
        ScraperClient -->|*scraper.Movie| PipelineOrUI
    end

    subgraph Interactive UI
        PipelineOrUI --> TUIModel[pkg/tui Model]
        TUIModel -->|Render| Views[pkg/tui Views & Styles]
        TUIModel -->|Modal Trigger| EditModal[pkg/tui EditModal]
    end
```

---

## 3. Component Deep Dive & Specifications

### 3.1 Scanner (`pkg/scanner`)

The scanner is responsible for file discovery and validation.

* **Traversal Mechanism**: Uses `filepath.WalkDir` with `context.Context` cancellation checks every 100 files to prevent indefinite blocking on network-attached storage (NAS) or large file trees.
* **Directory Pruning & Network Traversal Optimization**: Checks `d.IsDir()` immediately upon entering directory traversal and executes `filepath.SkipDir` for `.actors`, `extrafanart`, `@eaDir`, and hidden dot folders (`.`) without executing redundant `os.Lstat` syscalls. This completely eliminates network SMB latency bottlenecks, speeding up scans of archives with thousands of asset images from timeouts down to seconds.
* **Symlink Defense**: Checks `lstat.Mode() & os.ModeSymlink != 0`. Symlink directories are skipped (`filepath.SkipDir`) and symlink files are bypassed to prevent circular loops or out-of-boundary access.
* **Filter Criteria**:
  * **Extension Set**: Validates against a fast hash set `map[string]struct{}` (default: `.mp4`, `.mkv`, `.avi`, `.wmv`, `.flv`, `.iso`, `.ts`, `.m4v`, `.mov`).
  * **Size Threshold**: Ignores sample files, trailers, or corrupt dumps below `MinSizeMB` (default: 50MB).
  * **Glob Exclusions**: Applies `filepath.Match` against configured patterns (e.g., `*sample*`, `*trailer*`, `*.url`, `*.txt`).

### 3.2 Pattern Matcher (`pkg/matcher`)

The matcher converts irregular filenames into normalized JAV IDs.

#### Matching Priority Cascade:
1. **Noise Stripping**:
   Pre-processes the filename using `domainNoiseRegex`:
   ```regex
   ^(?:(?:https?://)?(?:www\.)?[\w\.-]+\.(?:com|me|net|org|cc|to|xyz|tv|vip|guru|fun|top|site|link|pw|club|vip)[@_]?|\s*\[[^\]]+\]|\s*\([^\)]+\)|[a-z0-9\._-]+@)\s*
   ```
   *Examples stripped:* `hhd800.com@`, `[4k2.com]`, `twojav.com@`, `user.name@`.

2. **Pattern Hierarchy**:
   | Priority | Pattern Type | Regex | Example Input | Normalized ID |
   |---|---|---|---|---|
   | **1** | Custom Regex | User configured | — | — |
   | **2** | FC2 | `(?:^\|[^a-zA-Z0-9])FC2(?:-PPV)?-(\d{5,8})` | `FC2-PPV-1234567` | `FC2-PPV-1234567` |
   | **3** | Uncensored Date-Based | `(?:^\|[^a-zA-Z0-9])(\d{6}[-_]\d{2,3}-(?:1PON\|10MU\|CARIB))` | `020326_001-1PON` | `020326_001-1PON` |
   | **4** | Standard Hyphenated | `(?:^\|[^a-zA-Z0-9])([A-Za-z]{2,6})-(\d{2,5})(?:[ZE])?` | `MIDA-517`, `SNOS-028` | `MIDA-517`, `SNOS-028` |
   | **5** | VR 5-Digit Content ID | `(?:^\|[^a-zA-Z0-9])([A-Za-z]{3,5})(\d{5})` | `kavr00428`, `sivr00394` | `KAVR-428`, `SIVR-394` |
   | **6** | Standard No-Hyphen | `(?:^\|[^a-zA-Z0-9])([A-Za-z]{3,6})(\d{3,4})` | `SNOS028`, `WAAA615` | `SNOS-028`, `WAAA-615` |
   | **7** | DMM Content ID | `(?:^\|[^a-zA-Z0-9])(h_\d+[a-z]+\d+)` | `h_1472smkcx003` | `h_1472smkcx003` |

3. **Multi-Part Heuristics (`pkg/matcher/multipart.go`)**:
   - Analyzes remainder string after matched token for indicators: `_1`, `_2_8k`, `pt1`, `part2`, `-A`, `-B`.
   - **Sibling Directory Validation (`ValidateMultipartInDirectory`)**: Single letter suffixes (`-A`, `-B`) are only flagged as multi-part if at least two matching parts exist within the same directory, preventing false positives on titles ending in a letter.

### 3.3 Normalizer & Scraper (`pkg/scraper`)

* **ID Normalization (`NormalizeToCombinedID`)**:
  R18.dev queries use combined IDs where the series prefix is lowercase and the number is 0-padded to 5 digits:
  $$\text{MIDA-517} \longrightarrow \text{"mida"} + 00517 \longrightarrow \text{mida00517}$$
  $$\text{SNOS-028} \longrightarrow \text{"snos"} + 00028 \longrightarrow \text{snos00028}$$
  $$\text{FC2-PPV-1234567} \longrightarrow \text{fc2-1234567}$$

* **HTTP Client Semantics**:
  - Endpoint: `https://r18.dev/videos/vod/movies/detail/-/combined={combined_id}/json`
  - Headers: Standard browser `User-Agent`, `Referer: https://r18.dev/`, `Accept: application/json`.
  - Non-200 / 404 responses are translated into strongly-typed errors without crashing.

### 3.4 Tier-1 Offline Dump Store (`pkg/scraper/dump.go`)

To permanently eliminate Cloudflare rate limiting (HTTP 429 and error 1015 IP ban), R19DEV integrates an offline query engine reading directly from PostgreSQL weekly dumps distributed by `https://r18.dev/dumps`:

* **Data Model & Ingestion**:
  - Offline SQLite database: `~/Library/Application Support/r19dev/r18_dump.db` (~471 MB).
  - Stream-parsed from `r18dotdev_dump_YYYY-MM-DD.sql.gz` in under 35 seconds.
  - Extracted & indexed datasets:
    - `r18_movies`: **1,902,762 videos** with `content_id`, `dvd_id`, `clean_id`, `title_en`, `title_ja`, `maker_name_en`, `release_date`, and `jacket_full_url`.
    - `actresses`: **101,906 performers** with Romaji & Kanji names, and DMM profile photos.
    - `video_actresses`: **2,480,384 video-actress relationships**.
    - `translations`: **540,500 DeepL translations** (`source_ja` $\rightarrow$ `target_en`) seamlessly backfilling English titles when `title_en` is missing.
* **Concurrent Read-Only Performance**:
  - Opened with `file:%s?mode=ro&_pragma=busy_timeout(3000)&_pragma=query_only(true)` — zero locking contention, safe for concurrent scans.
  - Sub-millisecond lookup latency (**`< 1ms`** / 0.00s in unit tests).
* **Metadata Resolution Sequence in `Client.Scrape`**:
  1. **Tier 0**: In-memory / filesystem JSON cache (`~/.cache/r19dev/metadata/{combined_id}.json`).
  2. **Tier 1**: `DumpStore.GetMovie(id, lang)` from `r18_dump.db`. Resolves English title, maker, dates, jackets, and full actress list completely offline. Populates Tier 0 cache upon return.
  3. **Tier 2**: Fallback to live R18.dev HTTP API only if the movie was released after the dump cutoff date.

### 3.5 Database & Audit Trail (`pkg/db`)

* **Storage Engine**: Pure Go SQLite (`modernc.org/sqlite` without CGO), stored at `~/Library/Application Support/r19dev/r19dev.db` (macOS) or `~/.config/r19dev/r19dev.db` (Linux) via `os.UserConfigDir()`.
* **Storage Location Strategy & SMB Mount Constraint**:
  - Network-attached storage shares mounted via SMB (`smbfs`, e.g. `/Volumes/home/...`) are fundamentally incompatible with SQLite WAL mode (`_pragma=journal_mode(wal)`) because Darwin `smbfs` lacks POSIX shared-memory `mmap` support for `.db-shm` and `modernc.org/libc` lacks Darwin `fsctl` support (`libc_darwin.go:277:Xfsctl: TODOTODO`).
  - Therefore, the active working SQLite database strictly resides on the local NVMe/SSD filesystem in `Application Support` (safe from OS cache cleaners), ensuring maximum read/write performance and crash resiliency.
* **Auto-Migration from Legacy Cache**:
  - On startup, if `~/Library/Application Support/r19dev/r19dev.db` does not exist, `db.Default()` checks the legacy location `~/Library/Caches/r19dev/r19dev.db` and seamlessly copies existing user data, watched status, and actress records forward.
* **NAS Auto-Backup Snapshot via `VACUUM INTO` (`BackupTo`)**:
  - When batch or single organize operations complete successfully, an atomic backup snapshot is saved to the destination root on the NAS as `.r19dev_backup.db`.
  - To prevent Darwin SMB `fsctl` errors, `BackupTo` executes `VACUUM INTO` targeting a local SSD temp file (`os.TempDir()`), then streams the clean, defragmented single-file snapshot to the destination via `io.Copy`.
* **Automatic Disaster Recovery on Fresh Machines**:
  - If a new machine launches R19DEV Studio without an existing local database, `db.Default()` checks for NAS backup candidates (`/Volumes/home/BT/organized/.r19dev_backup.db` or `/Volumes/home/BT/2026/organized/.r19dev_backup.db`) and automatically restores full user history and followed actresses.
* **UI & API Backup Access**:
  - Direct HTTP endpoint `/api/db/backup` triggers on-demand NAS snapshots, while `/api/db/backup?download=1` streams an instant `.db` download to the browser.
  - The History Modal features a dedicated `[💾 Backup DB]` button.
* **Schema & Relations**:
  - `actresses`: Tracked performers with Japanese/Romaji names, `r18_id INTEGER DEFAULT 0` for direct R18.dev links, follower status, and notes.
  - `movies`: Full cached R18.dev JSON payloads (titles, dates, directors, studio, label, series, actresses, genres, screenshots). Over 97% of titles populated with authentic English titles.
  - `user_state`: User watch state (`is_watched`), ratings (1–5 ⭐), and favorites (`is_favorite`).
  - `library_files`: Scanned file catalog with size, part number, and destination paths.
  - `organized_movies`: Maps `movie_id` to `target_folder` and `target_video` for instant status detection and One-Click Finder access.
  - `operation_history`: Audit trail for all organize and scrape runs storing execution metadata, success/fail metrics, and complete console output.
* **Safe Updates & Data Integrity**:
  - `SaveMovie` employs guarded `ON CONFLICT(id) DO UPDATE SET` clauses using SQL `CASE WHEN excluded.<field> != '' THEN excluded.<field> ELSE movies.<field> END` for `cover_url`, `poster_url`, `trailer_url`, `title`, and metadata arrays. This guarantees partial saves, mock objects, or network dropouts never overwrite existing rich metadata with empty strings.
* **Null-Safe Scanning & Content ID Fallback (`GetMovie`)**:
  - For DMM physical goods or legacy releases where `dvd_id` or timestamps are null (e.g., `EBDB-998` / `h_346rebdb998`), `GetMovie` uses SQL `COALESCE(dvd_id, id)` and `sql.NullTime` scanning. This prevents database driver scan errors and guarantees smooth fallback to `combined_id` across both scraper endpoints and the web UI.
* **Test Isolation (`SetDB`)**:
  - `pkg/organizer` and `pkg/web` allow overriding the active database via `organizer.SetDB(testDB)` and `web.Config{DB: testDB}` so test suites run against isolated temporary databases without mutating `~/Library/Application Support/r19dev/r19dev.db`.
* **Auto-Migration & R18 ID Backfill**:
  - Automatically migrates existing databases on boot: `ALTER TABLE actresses ADD COLUMN r18_id INTEGER DEFAULT 0;`.
  - Runs `backfillActressR18IDs()` on startup to inspect `movies.actresses_json` and automatically populate `r18_id` for followed actresses without requiring manual DB updates.
* **Auto-Retention & Clutter Prevention**:
  - Automatically prunes records older than 30 days: `DELETE FROM operation_history WHERE created_at < datetime('now', '-30 days')`.
  - Automatically enforces a 100-run ceiling: `DELETE FROM operation_history WHERE id NOT IN (SELECT id FROM operation_history ORDER BY id DESC LIMIT 100)`.
  - Maintains a tiny database footprint (< 5MB) with zero `.log` file clutter on user disks.
* **Local Storage & Strict Git Exclusion Policy**:
  - Application DB: `~/Library/Application Support/r19dev/r19dev.db` (macOS) or `~/.config/r19dev/r19dev.db` (Linux).
  - Dump DB: `~/Library/Application Support/r19dev/r18_dump.db`.
  - Compressed Dumps: `~/Library/Application Support/r19dev/dumps/`.
  - Git status: Resides outside the Git workspace; root `.gitignore` explicitly blocks `*.db`, `*.db-shm`, `*.db-wal`, `*.sql`, `*.sql.gz`, and `dumps/`. Local databases and dumps are strictly **never committed or pushed to Git**.

### 3.6 Jellyfin Organizer Pipeline (`pkg/organizer` & `pkg/jellyfin`)

* **Directory Layout & Priority**:
  ```
  /Volumes/home/BT/organized/<Actress_Name>/<JAV-ID Sanitized_Title>/
  ```
  1. **Actress Name**: English/Romaji name preferred. Falls back to Japanese Kanji if English is empty; defaults to `Unknown Actress` if neither exists.
  2. **Multi-Actress Group Work Prioritization**: When organizing group or crossover works (e.g. duo or harem titles), the organizer inspects followed actresses in SQLite and prioritizes placing the physical directory under **followed/tracked actresses** over untracked co-stars.
  3. **Zero Storage Waste (Single Physical Instance)**: Video files reside in exactly one physical folder on the NAS without duplication. Both Jellyfin (via multi-`<actor>` NFO tags) and R19DEV Studio (via SQLite metadata linking) display the movie under all participating co-stars' libraries simultaneously.
  4. **Movie Title**: English title preferred. Falls back to Original Japanese Title if English is empty; defaults to `JAV-ID` if neither exists.
  5. **Multi-Part Consolidation**: All parts of the same movie are moved into the same destination directory as `<JAV-ID>-cd1.mp4`, `<JAV-ID>-cd2.mp4` per Jellyfin multi-disc specifications.
* **Default Destination**: Defaults to `/Volumes/home/BT/organized` across both backend services and frontend inputs.
* **Filesystem Boundary Safety (ENAMETOOLONG Prevention)**:
  - `SanitizeFilename` strips invalid OS characters (`/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`) and collapses whitespace.
  - **180-Byte Hard Limit**: Truncates names strictly at $\le 180$ bytes without splitting UTF-8 multi-byte runes, preventing `ENAMETOOLONG` errors on APFS, ext4, NTFS, and SMB shares (where `NAME_MAX` is 255 bytes).
* **Metadata & Asset Generation**:
  - `<JAV-ID>.nfo`: Full XML metadata with premiered date, year, actors, plot, MPAA rating, and unique IDs.
  - `movie.html` (Cinematic Offline-First Interactive Viewer):
    - **Ambient Backdrop Hero**: Features full-width `fanart.jpg` backdrop with cinematic blur and dark gradient overlay, mirroring modern streaming UI aesthetics (Netflix, Apple TV).
    - **Direct Play Action Bar**: Prominent `▶ Play Movie` button launching the local file in the OS default player (VLC, IINA, QuickTime) and a `🖥️ Watch in Browser` modal player with HTML5 `<video controls>`.
    - **Multi-Part Video Routing**: Automatically detects multi-part files (e.g. `-pt1.mp4`, `-pt2.mp4` / `-cd1`, `-cd2`) and renders distinct `▶ Play Part 1` and `▶ Play Part 2` buttons.
    - **In-Page Lightbox Gallery**: Browsing sample screenshots opens a fluid, full-screen in-page modal without opening disruptive new browser tabs. Supports keyboard navigation (`←` / `→` arrows, `Esc` to close) and image counter (`X / Y`).
    - **Utility Toolbar**: One-click JAV ID copy (`📋 {JAV-ID}`) with floating toast notification, `🎬 Watch Trailer` button, and direct deep-link back to `🏠 R19dev Hub`.
    - **Local Asset Auto-Discovery**: Intelligently scans local `extrafanart/` and video files on disk, ensuring 100% offline functionality with zero external CDN dependencies.
  - Asset Downloader: Full jacket cover (`poster.jpg`), backdrop (`fanart.jpg`), and sample screenshots (`extrafanart/fanart{N}.jpg`).
* **One-Click Reveal**: Backend `POST /api/open-folder` invokes native file managers (`open` on macOS Finder, `explorer` on Windows, `xdg-open` on Linux) with direct resolution for both actress folders and specific movie directories.

### 3.7 Web Studio Architecture (`pkg/web`)

* **Single Binary Embedding**: All frontend assets (`index.html`, `style.css`, `fonts/*`, `js/*`, `vendor/lucide.min.js`) embedded via Go's `embed.FS` with zero external runtime dependencies.
* **100% Offline-Ready Architecture**:
  - Embedded local font files in `pkg/web/static/fonts/`:
    - `inter-variable.woff2` (Inter Variable 100–900)
    - `jetbrains-mono-latin.woff2` (JetBrains Mono Latin)
    - `material-symbols-outlined.woff2` (Complete glyph set of Google Material Symbols)
  - Absolute local `@font-face` declarations in `style.css`.
  - Zero external CDN links or font preconnects in `index.html`, allowing the UI to render flawlessly on offline NAS systems and isolated home networks.
* **Native ES Modules Architecture (`pkg/web/static/js/`)**:
  - Modularized frontend with zero build-step (no Node.js/npm required, served directly as native ES modules via `<script type="module" src="/js/app.js"></script>`):
    - `state.js`: Global reactive application state, DOM cache, sanitization, and toast system.
    - `api.js`: REST/SSE clients, scraping, user state updates, folder opening, actress operations.
    - `modal.js`: Full-width hero modal, PhotoSwipe 5 dynamic loader, ratings, and lightbox.
    - `scanner.js`: Media discovery stream, multi-part grouping, grid density, sorting, and directory rescan.
    - `organizer.js`: Jellyfin organizer stream, terminal log console, and auto-scroll controller.
    - `history.js`: Operation history modal and SQLite audit log inspection.
    - `actress.js`: Actress Hub (Followed/Unfollowed directory, Bento profile, dedicated filmography stage, and Chat mode).
    - `graph.js`: Force-directed relationship network graph (actresses, film styles, and studios).
    - `app.js`: Application bootstrap, 3-tier navigation tab routing, and `window.app` public interface binding.
* **3-Tier Navigation Architecture (User Journey Segregation)**:
  - **Tab 1: 📥 Incoming**: File intake and staging area for scanning unorganized downloads, matching SKUs, and triggering the Jellyfin organizer drawer.
  - **Tab 2: 👤 Actresses**: Focused performer tracking. Sub-tabs strictly separated into `Followed` (collection progress, backlog wishlist) and `Unfollowed` (performers found in local files eligible for quick follow).
  - **Tab 3: 🎬 Library**: Complete media catalog of all titles organized and ready to watch on NAS. Features status filter pills (`All Works`, `In Library`, `Missing`, `Watched`, `Favorites`), multi-criteria sorting (`Release Date`, `User Rating`, `Studio/Maker`, `JAV ID`, `Title`), and dropdown filters (`Genre`, `Studio`, `Actress`).
* **Universal Search in Sticky Top Header**:
  - Pinned centered glassmorphic search input (`#universal-search-input`) with context-aware routing (Incoming files/SKU, Followed Actress Directory, or Library catalog).
  - Global keyboard shortcuts: `⌘K` (macOS) / `Ctrl+K` (Windows/Linux) and `/` (when browsing) to focus; `Esc` to clear/blur.
  - Synchronized two-way with in-page stage search inputs.
* **Sticky Breadcrumb & Floating Quick Navigation**:
  - Header breadcrumb (`[← All Actresses] / {Actress Name}`) sticky on fixed navbar.
  - Floating Quick Navigation Pill (`[← All Actresses] | [↑ Top]`) auto-reveals via glassmorphism when scrolling down $> 300\text{px}$ in long filmographies.
  - Browser history integration: `history.pushState` and `popstate` event listeners enable hardware back button and trackpad two-finger swipe back.
* **Multi-Tier Image Endpoint (`/api/images/{id}`)**:
  - Layer 1: In-memory cache check (`cache.Default().GetImage(id)`).
  - Layer 2: Disk scan in organized folder (`poster.jpg`, `fanart.jpg`, `cover.jpg`) via SQLite `organized_movies` record or `/Volumes/home/BT/organized/*/*{id}*`.
  - Layer 3: Remote fetch via upgraded DMM URL using `scraper.DefaultUA` and `Referer: https://r18.dev/`, dynamically cached into RAM.
* **Real-Time Streaming (SSE)**:
  - Streaming endpoints: `/api/scan/stream`, `/api/organize/stream`, `/api/scrape/stream`.
  - **Connection Timeout Handling**: Removed 60s `WriteTimeout` on global `http.Server`. Active SSE handlers clear write deadlines via `rc := http.NewResponseController(w); rc.SetWriteDeadline(time.Time{})` and employ an extended 30-minute context timeout.
* **Console Log Ergonomics**:
  - **Smart Auto-Scroll**: Listens to viewport scroll position; scrolling up pauses auto-scroll (`Auto-Scroll: PAUSED`) and reveals a floating resume button. Scrolling to the bottom resumes auto-scroll automatically.
  - **Clipboard Copy**: Direct copy button copies raw console output with toast confirmation.
  - **History Integration**: Header button opens SQLite Operation History modal with instant log inspection and audit trail review.

### 3.8 Actress Hub: 2-Column Bento UI & Rich Metadata Engine

* **Followed & Unfollowed Actresses Directory**:
  - **Dual Sub-Tabs**: Toggle between `Followed Actresses` (active tracking) and `Unfollowed Actresses` (untracked performers with files found in local storage, with 1-click Quick Follow).
  - **Balanced 2-Element Card Layout**: Compact status pill `[ ✓ SNOS-140 ]` (emerald green if downloaded, rose/amber with download icon if missing) paired with a clean monospace release date `2026-03-24` (or `Recent`), preventing text overflow across all card widths.
  - **Multi-Layer Gatekeeper**: Strips compilation titles (総集編, BEST), photobooks, duplicate SKU formats (BOD, 9SNOS, K9SNOS), variety talk shows (`KCKC-`, `MLTN-`), AI Remaster re-issues (`JQRE-`, `AIリマスター`, `復刻`), and omnibus clip compilations (`BMW-`, `REbecca STARS`, $\ge 10$ performers).
  - **Canonical SKU Prioritization**: Smart deduplication engine favors standard maker disc codes over streaming outlet re-releases (e.g. `PPPD-485` preferred over `PPP-485`, `BOMN-169` over `BOM-169`).
  - **Minimalist Progress Track**: Sleek 6px progress bar with downloaded vs. total releases and completion percentage: `${dl}/${total} (${pct}%)`.
  - **Multi-Sort & Live Search**: Filter by name and sort by `% Completed`, `Most Missing`, `Name A-Z`, or `Total Works`.
* **2-Column Bento Profile & Dedicated Filmography Stage**:
  - **Left Sticky Bento Sidebar (~340px)**:
    - **Identity Bento**: 140px HD avatar with hover zoom, Romaji/Kanji names, verified R18 ID badge, and dynamic **Career Span** (`📅 2021 – 2026`).
    - **Storage & Library Bento**: Highlight of **Total NAS Storage** in GB (`TotalSizeBytes`), completion progress bar, and average file size (`Avg X.X GB / file`).
    - **Top Genres Bento**: Interactive tag cloud of the actress's top 8 most frequent categories (`TopGenres`). Clicking any genre chip dynamically filters her filmography on the right stage.
    - **Quick Actions Bento**: Direct `[📂 Open in Finder]` on NAS, `[🌐 R18.dev Profile ↗]`, and `[🔄 Check Releases]`.
  - **Right Filmography Main Stage**:
    - **In-Page Real-Time Search**: Instant filtering by movie ID or title substring.
    - **Sub-Filter Pills**: `All Works`, `In Library`, `Missing`, and **`Skipped`**.
    - **Auditable Exclusions (`Skipped` Tab)**: Displays excluded non-solo/duplicate works with specific reason badges (`Omnibus Compilation`, `AI Remaster`, `Variety Talk Show`, etc.) for complete transparency.
    - **Active Genre Indicator**: Visual tag chip with one-click clear button.
    - **Multi-Key Sorting**: `Release Date (Newest/Oldest)`, `File Size (Largest)`, and `Movie ID (A-Z)`.
    - **Standardized Poster Grid**: Ergonomic, uniform aspect ratio cards with hover actions.
    - **Responsive Breakpoint**: Auto-stacks cleanly to single-column layout on narrower screens (<960px).
* **Native Finder Controls**:
  - One-click `[📂 Open in Finder]` buttons in header, cards, and modals via `/api/open-folder`.


### 3.9 High-Speed Batch Migrator & Safe Reorganizer (`pkg/migrator`)

The migrator automates the reorganization of multi-terabyte unorganized or legacy media collections across local and SMB storage into standardized Jellyfin structures with zero data loss and full audit transparency.

* **5-Phase Migration Pipeline**:
  1. **Discovery & Pre-Flight Analysis**: Recursively crawls source directories, normalizes filenames via `pkg/matcher`, resolves rich metadata via `r18_dump.db`, maps multi-part segments (`-cd1` through `-cd5`), and computes target paths under `/Volumes/home/BT/organized/<Actress>/<Title>/`.
  2. **Pre-Flight Safety Gate**: Pauses execution before moving any files, presenting an operator confirmation card summarizing total movies found, duplicates handled, skipped non-JAV files, and destination targets (`[Enter] PROCEED` / `[q] CANCEL`).
  3. **Live Reorganization & Asset Merge**: Moves/renames video files, merges companion folder artwork (`poster.jpg`, `fanart.jpg`, `extrafanart/`), writes Jellyfin `.nfo` metadata and cinematic `movie.html`, and registers entries into SQLite `movies`, `organized_movies`, and `library_files`.
  4. **Concurrent HTML Upgrader**: Utilizes a 16-worker goroutine pool to refresh `movie.html` templates across destination folders in seconds, leveraging fast database discovery (`organized_movies` table queried in `< 0.01s`) to bypass slow SMB directory walks.
  5. **Safe Source Pruning (`cleanEmptyTree`)**: Executes bottom-up post-order directory traversal to safely delete only completely empty folders (`isDirEmpty`), strictly leaving non-empty folders containing skipped or foreign media intact.
* **Smart Collision & Quality Coexistence**:
  - Automatically identifies existing target titles in destination folders without throwing destructive errors or overwriting existing media.
  - Safely co-locates 4K editions (`-4k.mp4`), uncensored releases (`-uncensored.mp4`), and standard 1080p editions (`.mp4`) in the same movie directory.
  - Preserves and standardizes multi-part CD files (`-cd1.mp4` through `-cd5.mp4`), grouping all discs within the single title directory.
* **Dual Execution Modes**:
  - **Interactive Bubble Tea TUI (`tui.go`)**: Real-time progress bar, speed tracker (`X.X/s`), live activity cards, milestone notifications, and scrollable log pane.
  - **Headless CLI Runner (`cli.go`)**: Streamlined non-interactive execution with `--yes` and `--no-tui` flags for automated scripts and headless servers.

### 3.10 Atomic Crash-Consistent Database Backup Engine (`pkg/db`)

* **Problem Solved**: SQLite databases located on Darwin SMB network mounts (`smbfs`) suffer from file-locking latency, lack of POSIX shared-memory support, and corruption risks if backed up via naive file copies while write transactions are active.
* **Two-Phase Atomic Backup (`DB.BackupTo`)**:
  1. Executes SQLite's native `VACUUM INTO` command targeting an isolated temporary file on local SSD/NVMe storage (`os.TempDir()`), defragmenting database pages and capturing a 100% crash-consistent snapshot without holding long locks.
  2. Copies the defragmented backup file to the network destination (`/Volumes/home/BT/organized/.r19dev_backup.db`) via `io.Copy`.
* **Access Points**:
  - Command line: `./bin/r19dev backup /Volumes/home/BT/organized`
  - Web UI: Operation History modal `[💾 Backup DB]` button and direct REST endpoint `/api/db/backup?download=1`.

### 3.12 Configurable Filter Engine & Compilation Purge Pipeline (`pkg/scraper/filter.go`)

* **Problem Solved**: Re-edited omnibus compilations, Best-of compilations, promotional bonus variants, and multi-actress scene reels (e.g. `IPOK-035` with `100本番`, `Idea Pocket BEST` label/series, `MIZD-`, `MIDD-`, `OFJE-`, `SETH-`, `PBD-`, `OBST-`, `SDDE-`, `RBB-`, `MKCK-`, `MKMP-`, `OFRF-`, `OFMA-`) previously bypassed static keyword checks.
* **JSON Schema & Hot Reloading**:
  - Filter rules stored in `~/Library/Application Support/r19dev/filters.json` (macOS) or `~/.config/r19dev/filters.json` (Linux).
  - Configurable arrays: `blocked_prefixes`, `blocked_labels`, `blocked_series`, `blocked_genres`, `blocked_title_keywords`, `blocked_title_regex`, `blocked_cover_patterns`, `min_actress_omnibus_count`, `max_duration_minutes_threshold`.
  - Can be edited directly, via REST API (`POST /api/filters`), or managed via CLI (`r19dev filters purge`, `r19dev filters reset`).
* **Dynamic Database Sweep**:
  - `PurgePromotionalVariants` executes fast SQL matching alongside a comprehensive sweep through unowned movies using `scraper.IsPromotionalOrOmnibusVariantWithDetails(mid, mtitle, morig, mlabel, mseries, mcover, genres)`.
  - Movies with local media files on disk (`library_files`, `organized_movies`) or marked as watched/favorite in `user_state` are strictly protected and never purged.

---

## 4. Operational Runbook & Edge Cases

| Scenario | Symptom / Behavior | Engine Handling |
|---|---|---|
| **Noise Prefix in Filename** | `4k2.com@kavr00428_1_8k.mp4` | Matcher strips `4k2.com@`, parses `kavr00428` as `KAVR-428`, identifies part 1. |
| **VR 5-Digit zero padding** | `sivr00045` | Normalizer correctly maps to `SIVR-045` (3-digit minimum format) and `sivr00045` for R18.dev. |
| **Excessive Title Length** | Title > 250 characters (`CJOD-505`) | `SanitizeFilename` caps directory component at 180 bytes along UTF-8 boundaries, avoiding `ENAMETOOLONG`. |
| **Cloudflare Rate Limiting (429/1015)** | Massive scrapes trigger IP ban | Engine transparently routes queries to Tier-1 offline SQLite dump (`r18_dump.db`) in `< 1ms`, completely eliminating external HTTP traffic for 1.9M+ titles. |
| **Long-Running Organize Stream** | Connection closed at 60s | Global `WriteTimeout` removed, `SetWriteDeadline(time.Time{})` applied on SSE response controller. |
| **Multi-Part Video (CD1/CD2)** | Sibling parts in directory | Consolidated into a single Jellyfin folder with `-cd1.mp4`, `-cd2.mp4` naming. |
| **Network Failure during Scrape** | 503 / DNS / Timeout | UI displays clear warning badge in the detail panel with retry hint or manual ID override. |
| **Missing Remote Cover URL** | Empty `cover_url` in DB | `/api/images/{id}` automatically checks disk for `poster.jpg` / `fanart.jpg` and serves high-res local image. |
| **Null DVD ID on Physical Goods** | `EBDB-998` (`h_346rebdb998`) has `dvd_id = null` | `GetMovie` uses `COALESCE(dvd_id, id)` & `sql.NullTime`, safely returning movie via `combined_id` fallback without scan errors. |
| **Multi-Actress Group Works** | Co-star / duo / harem title with multiple performers | Organizer inspects followed actresses in SQLite, placing folder under the tracked performer; multi-`<actor>` NFO tags display movie in both actresses' libraries with zero storage duplication. |
| **Re-issue & AI Remasters** | `JQRE-027`, `AIリマスター`, `復刻` duplicate old catalog | Scraper filter checks maker prefix and title keywords, stripping re-issues to keep filmography strictly genuine original works. |
| **Omnibus Clip Compilations** | `BMW-364`, `REbecca STARS` (10–30 actresses) | Filter checks `BMW` prefix, studio tags, and actress count $\ge 10$, excluding omnibus clip reels. |
| **Unit Test Database Pollution** | Mock data wipes real DB | Isolated temporary SQLite DB passed via `organizer.SetDB()` and `web.Config{DB}`, preventing production DB mutation. |
| **Partial Metadata Overwrite** | Empty fields on re-save | `SaveMovie` SQL uses `CASE WHEN excluded.* != ''` preserving existing covers, titles, and metadata. |
| **Tokuten Goods Duplicate SKUs** | `TKCJOD-510`, `TKMFYD-123`, `チェキセット` | `CheckFilmographyInclusion` filters `TK[A-Z]{3,6}` and Japanese title markers; `deduplicateReleases` penalizes promo SKUs (`-100`) so canonical standard releases (`CJOD-510`, `MFYD-123`) always emerge as primary works. |
| **Omnibus Clip Compilations** | `RBB-334`, `\d+連発`, `80連発`, `\d+時間BOX` | Filter matches `RBB-` series and compilation regexes across both English and Japanese original titles, isolating them in the `Skipped` tab. |
| **Best-Of Studio Compilations** | `IPOK-035`, `MIZD-550`, `Idea Pocket BEST`, `MOODYZ Best`, `100本番` | Filter checks studio Label/Series name (`*BEST*`, `*総集編*`), compilation prefixes (`IPOK`, `IDBD`, `MIZD`, `PBD`, `OBST`, `SDDE`), and honban patterns (`[1-9]\d*本番`), purging unowned compilations automatically. |
| **Custom Filter Rules** | User wants to add custom blocked prefixes or keywords | User edits `~/Library/Application Support/r19dev/filters.json` or uses REST API `POST /api/filters`, applying rules instantly with `r19dev filters purge`. |
| **R18 Direct Outbound 404s** | Algorithmic `combined_id` (`start00223`, `fsdss00685`) yields 404 on R18.dev | 100% of movies are mapped to authentic DMM `content_id` from `r18_dump.db` (`1start223`, `1fsdss685`, `cjod510`), and direct links use `detail/-/id={id}/` ensuring 100% HTTP 200 OK responses. |
| **Homonymous Actress Search Collisions** | `r18_id = 0` triggers name search landing on wrong performer (e.g. Nao Satsuki 2007 vs 2026) | All 37 followed actresses are 100% matched to authentic DMM `r18_id` in SQLite, generating direct, collision-free profile URLs. |
| **Symlink Recursion** | Cyclic links in NAS | Skipped unconditionally at `os.Lstat` evaluation phase. |
| **Multi-Edition Quality Coexistence** | 4K, 1080p, and Uncensored versions of the same movie | Migrator detects existing folder, renames without collision (`-4k.mp4`, `-uncensored.mp4`), and registers all files in SQLite. |
| **Existing Target Collision** | Movie folder already exists in organized root | Migrator checks file existence; if distinct quality/part, moves safely; if identical duplicate, reports duplicate handled without overwriting. |
| **Pre-Flight Safety Abort** | User cancels migration at pre-flight review | Zero filesystem changes committed; engine cleanly halts and restores terminal state. |
| **Atomic Backup on Network Mount** | Backing up directly to SMB/NFS share | `BackupTo` executes `VACUUM INTO` to local NVMe temp directory first, defragmenting database pages before streaming to NAS share. |
| **Unfollowed Actress Avatar Missing** | Generic SVG placeholder on Unfollowed tab | `handleActressAvatar` automatically queries `r18_dump.db` and downloads authentic HD headshot from DMM CloudFront CDN. |
| **Darwin SMBfs fsctl Limitations on Backup Inspect** | SQLite `mode=ro` crashes on Darwin SMB mounts with `libc_darwin.go:277:Xfsctl: TODOTODO` | `InspectLatestActivityTime` safely streams network backup candidates to local SSD temp file before inspecting metadata. |
| **Low-Res Stale Thumbnail Served on Web Modal** | `MFYD-123` or other titles render small 147x200px cover despite Full HD `poster.jpg` existing on NAS | `/api/images/{id}` prioritizes local organized folder `poster.jpg` over stale RAM/disk cache, enforces $>20\text{KB}$ threshold, and upgrades remote URLs to `pl.jpg`. |
| **Multi-Studio Prefix Mismatch (Prestige, SOD, VR)** | `ABP-966`, `DLDSS-077`, `KAVR-403` fail naive normalization | `CandidateCombinedIDs` generates studio-specific numerical prefixes (`118`, `1`, `13`) testing candidate queries in sequence against dump DB and live API. |
| **Incomplete Post-Migration Assets** | Migrated folders missing `poster.jpg`, `fanart.jpg`, or sample screenshots | `r19dev migrate --audit` (or `-a`) runs post-migration verification with `pkg/audit` and automatically heals/downloads missing assets. |
| **Cross-User NAS Permissions (User D to User P)** | Moving files between distinct user home shares across SMB mounts | Mount `homes` root share, configure DSM File Station ACLs, or use central `/Volumes/video/` media share with automatic streaming copy fallback. |
| **DMM Outlet SKU Future Date Inversion** | DMM outlet SKUs (`88ssis614`, `77ssis688`) carrying future expiration dates (`2026-07-31`) sorted to top of filmography | Enforces earliest release date invariant (digital premiere `2023-02-24` / DVD `2023-02-28`); `scripts/fix_outlet_dates.py` demotes 77/88 SKUs and recovers genuine release dates. |
| **Stale Browser Avatar Cache** | Browser memory cache keeps old or placeholder headshot due to 1-year `max-age` | `/api/actresses/avatar/{name}` responds with `ETag` + `Cache-Control: no-cache, must-revalidate` and `?v={r18_id}` cache-buster. |
| **Mount Point Shift on "Show in Finder"** | User switches macOS mount from `/Volumes/home/` to `/Volumes/homes/plagad/` | `resolvePathToExisting` tests known path substitutions and falls back gracefully to parent actress directories. |
| **Bento Profile Unfollow Action** | Clicking Unfollow on actress profile triggers `TypeError: promptUnfollowActress is not a function` | Implemented `promptUnfollowActress` in `api.js` and exported to `window.app` in `app.js` with confirmation dialog. |

