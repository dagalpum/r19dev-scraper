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

### 3.4 Terminal User Interface (`pkg/tui`)

Built on the **Elm Architecture** (Model - View - Update):

1. **State Isolation**:
   - `files`: Raw filesystem entities.
   - `matches`: Extracted identifiers.
   - `metadataCache`: In-memory map of `[ID] -> *Movie`.
   - `scrapeErrors`: In-memory map of `[ID] -> error string`.
   - `editModal`: Focused text-input overlay state.

2. **Async Commands (`tea.Cmd`)**:
   - Directory scanning and API fetching are dispatched as asynchronous tea commands (`scanDoneMsg`, `scrapeDoneMsg`), ensuring the UI never stutters or drops frames during network or disk IO.

3. **Layout Engine**:
   - Dynamically calculates viewport dimensions on `tea.WindowSizeMsg`.
   - Responsive split ratio: 58% table view, remaining width dedicated to metadata inspection panel.

---

## 4. Operational Runbook & Edge Cases

| Scenario | Symptom / Behavior | Engine Handling |
|---|---|---|
| **Noise Prefix in Filename** | `4k2.com@kavr00428_1_8k.mp4` | Matcher strips `4k2.com@`, parses `kavr00428` as `KAVR-428`, identifies part 1. |
| **VR 5-Digit zero padding** | `sivr00045` | Normalizer correctly maps to `SIVR-045` (3-digit minimum format) and `sivr00045` for R18.dev. |
| **Network Failure during Scrape** | 503 / DNS / Timeout | TUI displays clear warning badge in the detail panel with retry hint (`Enter`) or override (`e`). |
| **Ambiguous / Custom Filename** | Unmatched file | User presses `e`, inputs correct ID, model re-triggers targeted scrape immediately. |
| **Symlink Recursion** | Cyclic links in NAS | Skipped unconditionally at `os.Lstat` evaluation phase. |

---

## 5. Extensibility & Future Architecture Hooks

1. **Additional Metadata Providers**:
   The `scraper.Client` can be extended into an interface (`Provider`) to support fallback scrapers (e.g. JavLibrary, DMM/Fanza, JavBus, MGS).
2. **NFO & Media Exporter**:
   `Movie` model contains canonical fields ready for Kodi/Jellyfin/Plex `.nfo` XML serialization and thumbnail downloading.
3. **Database Caching Layer**:
   `metadataCache` can be backed by SQLite or BoltDB for offline persistence.
