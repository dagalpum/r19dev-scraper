# 🎬 R19DEV Studio & Scraper

A modern, high-performance JAV video library scanner, pattern matcher, R18.dev metadata scraper, and Jellyfin NAS organizer written in Go. Available as both a **Single-Binary Web UI Studio** and an **Interactive Terminal TUI** with native GPU rendering.

---

## ✨ Key Features

### 🌐 1. Modern Web UI Studio (`r19dev web`)
- **Embedded Single Binary**: Built using Go's `embed.FS` — zero external runtime dependencies, zero Node.js required. Runs natively on macOS, Linux, or NAS servers.
- **📦 Multi-Part & Multi-File Aggregation**: Files belonging to the same movie (e.g. `_1.mp4`, `_2.mp4`, `-cd1.mp4`, `-cd2.mp4`) are automatically merged into a **single card** with part chips (`P1, P2 (2 parts • 8.4 GB)`).
- **🎛️ Dynamic Grid Density (1–5 Cards/Row)**: Adjust view layout from **1 card/row** (wide showcase layout with large cover) up to **5 cards/row** (compact grid) or **Auto**. Preferences are automatically saved in `localStorage`.
- **🖼️ Full-Width Hero Cover Modal**: Clicking any movie card displays a cinematic, full-width high-resolution cover banner with an ambient blurred backdrop, interactive rating stars (1–5 ⭐), watched toggle (👁️), favorite toggle (❤️), and direct full-screen zoom.
- **📸 High-Resolution Screenshot Lightbox**: Safe DMM high-resolution image upgrader (`jp-` format) with dual-layer fallback to prevent 404s, backend image proxy fallback, and `<meta name="referrer" content="no-referrer">` to prevent CDN hotlink blocking.
- **📊 Real-Time Streaming Progress Bars**: Live Server-Sent Events (SSE) stream progress bars for both **Scanning** (live file discovery & matching) and **NAS Organizing** (step-by-step progress, target path, and live console logs).
- **♿ WCAG 2.1 AA/AAA Compliant**: High-contrast typography, explicit `:focus-visible` keyboard rings, semantic landmark roles (`banner`, `main`, `tablist`, `progressbar`, `dialog`), `aria-label` tags, and a Skip-to-content navigation link.

### ⭐ 2. Actress Hub: Interactive Chat UI & Filmography Tracker
- **💬 Chat-Style Filmography Timeline**: An immersive messaging interface (LINE / Discord / Telegram style) where your followed actresses announce their releases chronologically:
  - **Left Contacts Sidebar**: Real-time list of followed actresses with avatars, online status, latest release snippet, and badge count for missing releases. Includes an instant search filter and "+ Follow Actress" friend box.
  - **Chat Dialogue Stream**: Left bubble shows the actress introducing her release with jacket cover, title, studio, release date, and `[📋 Copy ID]` button. Right bubble shows system responses confirming Jellyfin library status (`✅ จัดเก็บเข้า Jellyfin เรียบร้อยแล้ว` with folder path or `⏳ ยังไม่ได้ดาวน์โหลด`).
  - **Missing Release Styling**: Unacquired titles are styled with a sleek 90% grayscale cover and dashed border, smoothly animating back to full vibrant color on hover.
  - **Bottom Action Bar**: Quick access buttons for `[+ Track JAV-ID]`, `[🔄 Refresh Releases]`, and `[📂 Open Folder in Finder]`.
- **👤 Slide-Over Profile & Stats Drawer**:
  - Detailed actress profile with Japanese Kanji and Romaji names.
  - **Collection Progress Bar**: Visual percentage bar showing library completeness (e.g. 67% collected).
  - **Stats Grid**: Comprehensive counters for Total, Downloaded in Library, Missing, Watched, and Favorites.
  - **Verified R18.dev Links**: Direct links using the official actress ID format: `https://r18.dev/videos/vod/movies/list/?id={r18_id}&type=actress` (e.g. `1109487` for Hayasakakanon).
- **📋 One-Click Copy ID**: Dedicated `[📋 Copy ID]` buttons on every release card and within the movie detail modal for effortless clipboard copying.

### 📂 3. NAS Directory Organizer & Jellyfin Pipeline
- Organizes videos into the standardized folder structure:
  ```
  {Destination}/{Actress_English_Name}/{JAV-ID} {English_Title}/
  ```
  - **English Naming Priority**: Folders prioritize English/Romaji names for both Actresses and Titles, seamlessly falling back to Japanese only if English metadata is unavailable.
  - **Filesystem Safety**: Names are safely capped at 180 bytes with UTF-8 boundary validation to prevent OS filesystem `ENAMETOOLONG` errors (255-byte `NAME_MAX`).
- **Consolidation**: Consolidates multi-part videos (e.g. `SNOS-038-cd1.mp4`, `SNOS-038-cd2.mp4`) into single unified Jellyfin movie entries.
- **Jellyfin Metadata (.nfo)**: Generates official Kodi/Jellyfin Movie NFO XML with title, original title, plot, studio, release date, runtime, actresses with thumbnail URLs, genres, watched status, and user ratings.
- **Standalone `movie.html`**: Generates a responsive, standalone dark-mode summary page with embedded actress cards and sample screenshots for offline browsing.
- **High-Res Assets**: Downloads full-resolution `poster.jpg` (cover jacket), `fanart.jpg` (backdrop), and all sample gallery screenshots into `extrafanart/`.
- **One-Click Reveal in Finder / File Manager**: Click `[📂 Open in Finder]` directly on any card or modal to immediately reveal the organized files in macOS Finder, Windows Explorer, or Linux.
- **Safe Dry-Run Mode**: Supports `--dry-run` to preview all target folder moves and asset creations safely before applying changes.

### 📜 4. Console Log & SQLite Operation History (Audit Trail)
- **Smart Auto-Scroll**: Console Log automatically pauses auto-scrolling when the user scrolls up to inspect previous lines (`Auto-Scroll: PAUSED`), displaying a floating `⬇️ New logs below (Click to resume)` button.
- **One-Click Log Copy**: Instant `📋 Copy` button to copy complete console logs to the clipboard.
- **SQLite Operation History (`operation_history`)**: All batch and single Organize/Scrape operations are automatically recorded into SQLite with complete timestamps, counts (success/fail), parameters, and full audit logs.
- **Zero Clutter & Auto-Retention**: Auto-prunes entries older than 30 days and maintains a maximum of 100 entries, ensuring a tiny database footprint with zero disk clutter.
- **History Viewer Modal**: Accessible via `📜 History` on the console header with full detail inspection and log copying.

### 🖥️ 5. Interactive Terminal TUI (`r19dev tui`)
- Built with Charm's **Bubble Tea** and **Lip Gloss**.
- **Kitty GPU Graphics Protocol**: Native pixel-perfect bitmap rendering on Kitty, Ghostty, and WezTerm.
- **iTerm2 Inline Images Protocol**: Native bitmap rendering on iTerm2, WezTerm, Warp, and VS Code.
- **Sixel Graphics Protocol**: 6-pixel band bitmap rendering on Foot, XTerm, and Sixel-enabled terminals.
- **24-bit Truecolor Half-Block (`▀`)**: Universal fallback for any terminal emulator.
- Quick shortcut keys: `Enter` to scrape, `v` for Kitty GPU cover, `t` for watched, `1`-`5` to rate, `a` to follow actress, `w` to organize.

---

## 🚀 Quick Start

### 1. Build Single Binary
```bash
make build
# Compiles standalone binary to ./bin/r19dev
```

### 2. Launch Web UI Studio (Recommended)
```bash
# Launches web server & opens browser automatically at http://localhost:8080
./bin/r19dev web /Volumes/home/BT/2026

# Custom port or no-open mode for headless servers / NAS:
./bin/r19dev web /Volumes/home/BT/2026 --port 9090 --no-open
```

### 3. Launch Interactive Terminal TUI
```bash
# Auto-detect terminal graphics capability
./bin/r19dev tui /Volumes/home/BT/2026

# Force Kitty GPU protocol (Ghostty / Kitty):
./bin/r19dev tui /Volumes/home/BT/2026 --proto kitty
```

### 4. Actress Tracking via CLI
```bash
# Follow an actress
./bin/r19dev actress follow "Kanna Seto"

# List followed actresses
./bin/r19dev actress list

# Check new releases vs local library
./bin/r19dev actress check
```

### 5. NAS Directory Organize via CLI
```bash
# Safe preview without moving files (Dry-Run)
./bin/r19dev organize /Volumes/home/BT/2026 /Volumes/home/BT/2026/JAV_Library --dry-run

# Execute organization and asset generation
./bin/r19dev organize /Volumes/home/BT/2026 /Volumes/home/BT/2026/JAV_Library
```

---

## 📂 NAS Jellyfin Directory Structure

```
/Volumes/home/BT/2026/JAV_Library/
└── Kanna Seto/                                            # English / Romaji Actress Name
    └── SNOS-038 AV Debut 1st Anniversary Work.../         # JAV-ID + English Title (capped at 180 bytes)
        ├── SNOS-038.mp4                                   # Video file (or -cd1.mp4, -cd2.mp4)
        ├── SNOS-038.nfo                                   # Kodi / Jellyfin Metadata XML
        ├── movie.html                                     # Standalone responsive summary page
        ├── poster.jpg                                     # High-Res Cover Jacket
        ├── fanart.jpg                                     # Landscape Backdrop
        └── extrafanart/                                   # High-Res Sample Screenshots
            ├── fanart1.jpg
            ├── fanart2.jpg
            └── fanart12.jpg
```

---

## ⌨️ TUI Keybindings

| Key | Action |
|---|---|
| `↑` / `k` / `↓` / `j` | Navigate files |
| `PgUp` / `PgDn` | Page scroll |
| `Enter` / `Space` | Fetch R18.dev metadata & cover preview |
| `v` | Fullscreen Cover Jacket (Native Kitty GPU mode) |
| `c` | Toggle Cover Art preview on/off |
| `p` | Cycle graphics protocol (Auto $\rightarrow$ Kitty $\rightarrow$ iTerm2 $\rightarrow$ Sixel $\rightarrow$ HalfBlock) |
| `t` | Toggle Watched status (👁️) |
| `f` | Toggle Favorite status (❤️) |
| `1` – `5` | Set User Rating (1 to 5 stars ⭐) |
| `a` | Follow primary actress |
| `w` | Organize current movie for NAS Jellyfin |
| `e` | Edit / Override JAV ID for selected file |
| `r` | Rescan directory |
| `q` / `Ctrl+C` | Quit |

---

## 🛠️ Architecture & Tech Stack

- **Language**: Go 1.22+
- **Database**: Pure Go SQLite (`modernc.org/sqlite` - zero CGO required)
- **TUI Framework**: Charm Bubble Tea (`tea.Model`), Lip Gloss styling
- **Web Frontend**: Vanilla ES6+ SPA, Vanilla CSS with CSS Grid & Custom Tokens, Embedded via `embed.FS`
- **Metadata Source**: R18.dev REST API with persistent LRU disk caching (`~/.cache/r19dev` or `~/Library/Caches/r19dev`)
- **Organize Pipeline**: Atomic file rename with cross-filesystem copy fallback, sanitized filenames, XML generator, and HTTP client asset downloader.
- **Audit Logging**: SQLite-backed `operation_history` table with 30-day / 100-run auto-retention policy.
