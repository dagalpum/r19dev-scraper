# R19DEV Scraper

A modern, standalone JAV video library scanner, pattern matcher, and R18.dev metadata scraper written in Go, featuring a sleek interactive **Terminal User Interface (TUI)** built with Charm's Bubble Tea and Lip Gloss.

---

## ✨ Features

- 📁 **Safe & Fast Video Scanner**: Recursively discovers video files (`.mp4`, `.mkv`, `.avi`, `.wmv`, `.iso`, etc.), filters out ad/sample files below configured thresholds (MinSizeMB), and safely ignores symlinks.
- 🎯 **Advanced JAV ID Matcher**:
  - Automatically strips domain and tracker noise prefixes (e.g. `hhd800.com@`, `4k2.com@`, `twojav.com@`).
  - Supports standard hyphenated IDs (`MIDA-517`, `SNOS-028`, `WAAA-615`).
  - Supports 5-digit VR and Content IDs (`kavr00428` $\rightarrow$ `KAVR-428`, `sivr00394`, `mdvr00406`).
  - Supports FC2 (`FC2-PPV-1234567`) and uncensored date-based IDs (`020326_001-1PON`).
  - Multi-part detection and sibling directory validation (`_1`, `_2`, `-pt1`, `-pt2`).
- 🌐 **R18.dev Scraper**: Direct JSON API integration with R18.dev in English (default) or Japanese for rich movie metadata, cast information, genre tags, cover jackets, and sample screenshot galleries.
- 🖼️ **Pixel Graphics Protocols & Truecolor Cover Art**:
  - **Kitty Graphics Protocol**: Native pixel-perfect bitmap rendering on Kitty, Ghostty, WezTerm.
  - **iTerm2 Inline Images Protocol**: Native bitmap rendering on iTerm2, WezTerm, Warp, VS Code.
  - **Sixel Graphics Protocol**: 6-pixel band bitmap rendering on Foot, XTerm, and Sixel-enabled terminals.
  - **ANSI Half-Block (`▀`)**: Universal 24-bit Truecolor fallback for any standard terminal emulator.
  - **Auto-Detection & Real-Time Switching**: Press `p` in TUI to cycle through protocols!
- 🖥️ **Interactive TUI**:
  - Live split-view table and metadata inspector.
  - Manual ID override modal (`e` key) for edge-case files.
  - One-key live scraping preview (`Enter`).
  - Toggle cover view (`c` key).
  - Switch graphics protocol (`p` key).

---

## 🚀 Quick Start

### 1. Build
```bash
make build
# or: go build -o bin/r19dev ./cmd/r19dev
```

### 2. Launch Interactive TUI
```bash
# Auto-detect best graphics protocol
./bin/r19dev tui /Volumes/home/BT/2026

# Or specify a protocol explicitly
./bin/r19dev tui /Volumes/home/BT/2026 --proto kitty
./bin/r19dev tui /Volumes/home/BT/2026 --proto iterm2
./bin/r19dev tui /Volumes/home/BT/2026 --proto sixel
./bin/r19dev tui /Volumes/home/BT/2026 --proto halfblock
```

### 3. CLI Scan & Match
```bash
./bin/r19dev scan /Volumes/home/BT/2026
```

### 4. Direct R18.dev Scrape (English)
```bash
./bin/r19dev scrape MIDA-517
```

---

## ⌨️ TUI Keybindings

| Key | Action |
|---|---|
| `↑` / `k` / `↓` / `j` | Navigate files |
| `PgUp` / `PgDn` | Page scroll |
| `Enter` / `Space` | Fetch R18.dev metadata & cover preview |
| `c` | Toggle Cover Art preview on/off |
| `p` | Cycle graphics protocol (Auto $\rightarrow$ Kitty $\rightarrow$ iTerm2 $\rightarrow$ Sixel $\rightarrow$ HalfBlock) |
| `e` | Edit / Override JAV ID for selected file |
| `r` | Rescan directory |
| `q` / `Ctrl+C` | Quit |
