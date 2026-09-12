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
  - `movies`: Full cached R18.dev JSON payloads (titles, dates, directors, studio, actresses, genres, screenshots).
  - `user_state`: User watch state (`is_watched`), ratings (1–5 ⭐), and favorites (`is_favorite`).
  - `library_files`: Scanned file catalog with size, part number, and destination paths.
  - `organized_movies`: Maps `movie_id` to `target_folder` and `target_video` for instant status detection and One-Click Finder access.
  - `operation_history`: Audit trail for all organize and scrape runs storing execution metadata, success/fail metrics, and complete console output.
* **Safe Updates & Data Integrity**:
  - `SaveMovie` employs guarded `ON CONFLICT(id) DO UPDATE SET` clauses using SQL `CASE WHEN excluded.<field> != '' THEN excluded.<field> ELSE movies.<field> END` for `cover_url`, `poster_url`, `trailer_url`, `title`, and metadata arrays. This guarantees partial saves, mock objects, or network dropouts never overwrite existing rich metadata with empty strings.
* **Test Isolation (`SetDB`)**:
  - `pkg/organizer` and `pkg/web` allow overriding the active database via `organizer.SetDB(testDB)` and `web.Config{DB: testDB}` so test suites run against isolated temporary databases without mutating `~/Library/Application Support/r19dev/r19dev.db`.
* **Auto-Migration & R18 ID Backfill**:
  - Automatically migrates existing databases on boot: `ALTER TABLE actresses ADD COLUMN r18_id INTEGER DEFAULT 0;`.
  - Runs `backfillActressR18IDs()` on startup to inspect `movies.actresses_json` and automatically populate `r18_id` for followed actresses without requiring manual DB updates.
* **Auto-Retention & Clutter Prevention**:
  - Automatically prunes records older than 30 days: `DELETE FROM operation_history WHERE created_at < datetime('now', '-30 days')`.
  - Automatically enforces a 100-run ceiling: `DELETE FROM operation_history WHERE id NOT IN (SELECT id FROM operation_history ORDER BY id DESC LIMIT 100)`.
  - Maintains a tiny database footprint (< 5MB) with zero `.log` file clutter on user disks.
* **Local Storage & Git Exclusion Policy**:
  - File name: `r19dev.db`.
  - Location: `~/Library/Application Support/r19dev/r19dev.db` (macOS) or `~/.config/r19dev/r19dev.db` (Linux).
  - Git status: Resides outside the Git workspace; root `.gitignore` explicitly blocks `*.db`, `*.db-shm`, and `*.db-wal`. Local databases are strictly never committed or pushed to Git.

### 3.6 Jellyfin Organizer Pipeline (`pkg/organizer` & `pkg/jellyfin`)

* **Directory Layout & Priority**:
  ```
  /Volumes/home/BT/organized/<Actress_Name>/<JAV-ID Sanitized_Title>/
  ```
  1. **Actress Name**: English/Romaji name preferred. Falls back to Japanese Kanji if English is empty; defaults to `Unknown Actress` if neither exists.
  2. **Movie Title**: English title preferred. Falls back to Original Japanese Title if English is empty; defaults to `JAV-ID` if neither exists.
  3. **Multi-Part Consolidation**: All parts of the same movie are moved into the same destination directory as `<JAV-ID>-cd1.mp4`, `<JAV-ID>-cd2.mp4` per Jellyfin multi-disc specifications.
* **Default Destination**: Defaults to `/Volumes/home/BT/organized` across both backend services and frontend inputs.
* **Filesystem Boundary Safety (ENAMETOOLONG Prevention)**:
  - `SanitizeFilename` strips invalid OS characters (`/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`) and collapses whitespace.
  - **180-Byte Hard Limit**: Truncates names strictly at $\le 180$ bytes without splitting UTF-8 multi-byte runes, preventing `ENAMETOOLONG` errors on APFS, ext4, NTFS, and SMB shares (where `NAME_MAX` is 255 bytes).
* **Metadata & Asset Generation**:
  - `<JAV-ID>.nfo`: Full XML metadata with premiered date, year, actors, plot, MPAA rating, and unique IDs.
  - `movie.html`: Standalone offline dark-mode HTML summary page with gallery lightbox.
  - Asset Downloader: Full jacket cover (`poster.jpg`), backdrop (`fanart.jpg`), and sample screenshots (`extrafanart/fanart{N}.jpg`).
* **One-Click Reveal**: Backend `POST /api/open-folder` invokes native file managers (`open` on macOS Finder, `explorer` on Windows, `xdg-open` on Linux) with direct resolution for both actress folders and specific movie directories.

### 3.7 Web Studio Architecture (`pkg/web`)

* **Single Binary Embedding**: Frontend assets (`index.html`, `style.css`, `app.js`, `vendor/lucide.min.js`) embedded via `embed.FS`.
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

* **Followed Actresses Directory**:
  - **4-Layer Gatekeeper**: Strips compilation titles (総集編, BEST), photobooks, and duplicate SKU formats (BOD, 9SNOS, K9SNOS), retaining 100% genuine solo releases.
  - **Minimalist Progress Track**: Sleek 6px progress bar with downloaded vs. total releases and completion percentage: `${dl}/${total} (${pct}%)`.
  - **Multi-Sort & Live Search**: Filter by name and sort by `% Completed`, `Most Missing`, `Name A-Z`, or `Total Works`.
* **2-Column Bento Profile & Dedicated Filmography Stage**:
  - **Left Sticky Bento Sidebar (~340px)**:
    - **Identity Bento**: 140px HD avatar with hover zoom, Romaji/Kanji names, verified R18 ID badge, and dynamic **Career Span** (`📅 2021 – 2026`).
    - **Storage & Library Bento**: Highlight of **Total NAS Storage** in GB (`TotalSizeBytes`), completion progress bar, and average file size (`Avg X.X GB / file`).
    - **Top Genres Bento**: Interactive tag cloud of the actress's top 8 most frequent categories (`TopGenres`). Clicking any genre chip dynamically filters her filmography on the right stage.
    - **Quick Actions Bento**: Direct `[📂 Open in Finder]` on NAS, `[🌐 R18.dev Profile ↗]`, and `[🔄 Refresh Releases]`.
  - **Right Filmography Main Stage**:
    - **In-Page Real-Time Search**: Instant filtering by movie ID or title substring.
    - **Sub-Filter Pills**: `All Works`, `In Library`, and `Missing`.
    - **Active Genre Indicator**: Visual tag chip with one-click clear button.
    - **Multi-Key Sorting**: `Release Date (Newest/Oldest)`, `File Size (Largest)`, and `Movie ID (A-Z)`.
    - **Standardized Poster Grid**: Ergonomic, uniform aspect ratio cards with hover actions.
    - **Responsive Breakpoint**: Auto-stacks cleanly to single-column layout on narrower screens (<960px).
* **Native Finder Controls**:
  - One-click `[📂 Open in Finder]` buttons in header, cards, and modals via `/api/open-folder`.

---

## 4. Operational Runbook & Edge Cases

| Scenario | Symptom / Behavior | Engine Handling |
|---|---|---|
| **Noise Prefix in Filename** | `4k2.com@kavr00428_1_8k.mp4` | Matcher strips `4k2.com@`, parses `kavr00428` as `KAVR-428`, identifies part 1. |
| **VR 5-Digit zero padding** | `sivr00045` | Normalizer correctly maps to `SIVR-045` (3-digit minimum format) and `sivr00045` for R18.dev. |
| **Excessive Title Length** | Title > 250 characters (`CJOD-505`) | `SanitizeFilename` caps directory component at 180 bytes along UTF-8 boundaries, avoiding `ENAMETOOLONG`. |
| **Long-Running Organize Stream** | Connection closed at 60s | Global `WriteTimeout` removed, `SetWriteDeadline(time.Time{})` applied on SSE response controller. |
| **Multi-Part Video (CD1/CD2)** | Sibling parts in directory | Consolidated into a single Jellyfin folder with `-cd1.mp4`, `-cd2.mp4` naming. |
| **Network Failure during Scrape** | 503 / DNS / Timeout | UI displays clear warning badge in the detail panel with retry hint or manual ID override. |
| **Missing Remote Cover URL** | Empty `cover_url` in DB | `/api/images/{id}` automatically checks disk for `poster.jpg` / `fanart.jpg` and serves high-res local image. |
| **Unit Test Database Pollution** | Mock data wipes real DB | Isolated temporary SQLite DB passed via `organizer.SetDB()` and `web.Config{DB}`, preventing production DB mutation. |
| **Partial Metadata Overwrite** | Empty fields on re-save | `SaveMovie` SQL uses `CASE WHEN excluded.* != ''` preserving existing covers, titles, and metadata. |
| **Symlink Recursion** | Cyclic links in NAS | Skipped unconditionally at `os.Lstat` evaluation phase. |
