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

* **Storage Engine**: Pure Go SQLite (`modernc.org/sqlite` without CGO), stored at `~/.cache/r19dev/r19dev.db` (Linux) or `~/Library/Caches/r19dev/r19dev.db` (macOS).
* **Schema & Relations**:
  - `actresses`: Tracked performers with Japanese/Romaji names, `r18_id INTEGER DEFAULT 0` for direct R18.dev links, follower status, and notes.
  - `movies`: Full cached R18.dev JSON payloads (titles, dates, directors, studio, actresses, genres, screenshots).
  - `user_state`: User watch state (`is_watched`), ratings (1–5 ⭐), and favorites (`is_favorite`).
  - `library_files`: Scanned file catalog with size, part number, and destination paths.
  - `organized_movies`: Maps `movie_id` to `target_folder` and `target_video` for instant status detection and One-Click Finder access.
  - `operation_history`: Audit trail for all organize and scrape runs storing execution metadata, success/fail metrics, and complete console output.
* **Auto-Migration & R18 ID Backfill**:
  - Automatically migrates existing databases on boot: `ALTER TABLE actresses ADD COLUMN r18_id INTEGER DEFAULT 0;`.
  - Runs `backfillActressR18IDs()` on startup to inspect `movies.actresses_json` and automatically populate `r18_id` for followed actresses without requiring manual DB updates.
* **Auto-Retention & Clutter Prevention**:
  - Automatically prunes records older than 30 days: `DELETE FROM operation_history WHERE created_at < datetime('now', '-30 days')`.
  - Automatically enforces a 100-run ceiling: `DELETE FROM operation_history WHERE id NOT IN (SELECT id FROM operation_history ORDER BY id DESC LIMIT 100)`.
  - Maintains a tiny database footprint (< 5MB) with zero `.log` file clutter on user disks.

### 3.6 Jellyfin Organizer Pipeline (`pkg/organizer` & `pkg/jellyfin`)

* **Directory Layout & Priority**:
  ```
  <Destination_Root>/<Actress_Name>/<JAV-ID Sanitized_Title>/
  ```
  1. **Actress Name**: English/Romaji name preferred. Falls back to Japanese Kanji if English is empty; defaults to `Unknown Actress` if neither exists.
  2. **Movie Title**: English title preferred. Falls back to Original Japanese Title if English is empty; defaults to `JAV-ID` if neither exists.
  3. **Multi-Part Consolidation**: All parts of the same movie are moved into the same destination directory as `<JAV-ID>-cd1.mp4`, `<JAV-ID>-cd2.mp4` per Jellyfin multi-disc specifications.
* **Filesystem Boundary Safety (ENAMETOOLONG Prevention)**:
  - `SanitizeFilename` strips invalid OS characters (`/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`) and collapses whitespace.
  - **180-Byte Hard Limit**: Truncates names strictly at $\le 180$ bytes without splitting UTF-8 multi-byte runes, preventing `ENAMETOOLONG` errors on APFS, ext4, NTFS, and SMB shares (where `NAME_MAX` is 255 bytes).
* **Metadata & Asset Generation**:
  - `<JAV-ID>.nfo`: Full XML metadata with premiered date, year, actors, plot, MPAA rating, and unique IDs.
  - `movie.html`: Standalone offline dark-mode HTML summary page with gallery lightbox.
  - Asset Downloader: Full jacket cover (`poster.jpg`), backdrop (`fanart.jpg`), and sample screenshots (`extrafanart/fanart{N}.jpg`).
* **One-Click Reveal**: Backend `POST /api/open-folder` invokes native file managers (`open` on macOS Finder, `explorer` on Windows, `xdg-open` on Linux).

### 3.7 Web Studio Architecture (`pkg/web`)

* **Single Binary Embedding**: Frontend assets (`index.html`, `style.css`, `app.js`, `vendor/lucide.min.js`) embedded via `embed.FS`.
* **Real-Time Streaming (SSE)**:
  - Streaming endpoints: `/api/scan/stream`, `/api/organize/stream`, `/api/scrape/stream`.
  - **Connection Timeout Handling**: Removed 60s `WriteTimeout` on global `http.Server`. Active SSE handlers clear write deadlines via `rc := http.NewResponseController(w); rc.SetWriteDeadline(time.Time{})` and employ an extended 30-minute context timeout.
* **Console Log Ergonomics**:
  - **Smart Auto-Scroll**: Listens to viewport scroll position; scrolling up pauses auto-scroll (`Auto-Scroll: PAUSED`) and reveals a floating resume button. Scrolling to the bottom resumes auto-scroll automatically.
  - **Clipboard Copy**: Direct copy button copies raw console output with toast confirmation.
  - **History Integration**: Header button opens SQLite Operation History modal with instant log inspection and audit trail review.

### 3.8 Actress Hub: Chat UI & Filmography Engine

* **Chat Interface Architecture**:
  - Replaces traditional grid cards with an interactive LINE / Discord / Telegram style two-column messaging app layout.
  - **Left Sidebar**: Real-time list of followed actresses with avatars, online status, latest release snippet, missing releases count badge, instant search filter, and quick follow friend box.
  - **Conversation Feed**: Left bubble features the actress announcing her release with jacket cover, JAV-ID, title, studio, release date, and `[📋 Copy ID]` button. Right bubble features system response displaying Jellyfin organized path or unacquired status.
  - **Grayscale Effect for Missing Items**: Covers of unacquired movies are rendered with a 90% grayscale filter and dashed border, transitioning smoothly to full color on hover.
  - **Slide-Over Profile Drawer**: Toggles smoothly from the right edge with avatar, Romaji/Kanji names, R18 ID, collection completeness progress bar, and comprehensive stats (Total, Downloaded, Missing, Watched, Favorites).
  - **Official R18.dev URLs**: Uses the verified actress URL structure `https://r18.dev/videos/vod/movies/list/?id={r18_id}&type=actress` rather than broken search endpoints.

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
| **Symlink Recursion** | Cyclic links in NAS | Skipped unconditionally at `os.Lstat` evaluation phase. |
