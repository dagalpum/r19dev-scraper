# R19DEV Studio & Scraper

A modern, high-performance JAV video library scanner, pattern matcher, R18.dev metadata scraper, and Jellyfin NAS organizer written in Go. Available as both a **Single-Binary Web UI Studio** and an **Interactive Terminal TUI**.

---

## ✨ Features

- 🌐 **Modern Web UI Studio**:
  - **Embedded Single Binary**: Runs on any Mac, Linux, or NAS with zero external runtime dependencies (`embed.FS`).
  - **Full HD Visuals**: Real high-resolution covers, backdrop banners, actress profile avatars, and interactive lightbox galleries for sample screenshots.
  - **Live Search & Filter**: Instantly filter by JAV ID, title, filename, actress, studio, watched status, or rating.
  - **Batch Operations**: One-click "Scrape All" and "Batch Organize".
- ⭐ **Actress Hub & Release Tracker**:
  - Follow favorite actresses and track their entire filmography.
  - Automatically cross-references with your local library to classify releases:
    - 🟢 **Downloaded** (available on your disk/NAS)
    - 🔴 **Missing / New** (announced/released but not downloaded yet)
    - 👁️ **Watched / Unwatched**
    - ⭐ **User Rating (1-5 stars) / Favorite ❤️**
- 📂 **NAS Directory Organizer & Jellyfin Pipeline**:
  - Organizes movies into: `{Destination}/{Actress}/{JAV-ID} {Title}/`
  - Moves video files (with multi-part `-cd1`, `-cd2` consolidation).
  - Generates official Kodi / Jellyfin XML `.nfo` files.
  - Generates standalone, responsive dark-mode `movie.html` summary pages.
  - Auto-upgrades DMM URLs to download Full HD `poster.jpg`, `fanart.jpg`, and all sample screenshots into `extrafanart/`.
  - Supports `--dry-run` to preview all folder operations safely before moving.
- 💾 **SQLite Local Database**:
  - Embedded SQLite database (`~/.cache/r19dev/r19dev.db`) storing actress follow lists, filmographies, watch history, and 1-5 star ratings.
- 🖥️ **Interactive Terminal TUI**:
  - Built with Charm's Bubble Tea and Lip Gloss.
  - 24-bit Truecolor and native Kitty GPU graphics mode.

---

## 🚀 Quick Start

### 1. Build
```bash
make build
# Creates standalone binary at ./bin/r19dev
```

### 2. Launch Web UI Studio (Recommended)
```bash
# Launches local web server & automatically opens your browser at http://localhost:8080
./bin/r19dev web /Volumes/home/BT/2026

# Or specify custom port
./bin/r19dev web /Volumes/home/BT/2026 --port 9090
```

### 3. Launch Interactive Terminal TUI
```bash
./bin/r19dev tui /Volumes/home/BT/2026
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

### 5. NAS Jellyfin Organize via CLI
```bash
# Preview folder organization without moving files (Dry-Run)
./bin/r19dev organize /Volumes/home/BT/2026 /Volumes/home/JAV_Library --dry-run

# Execute organization
./bin/r19dev organize /Volumes/home/BT/2026 /Volumes/home/JAV_Library
```

---

## 📂 NAS Jellyfin Directory Structure

```
/Volumes/home/JAV_Library/
└── 瀬戸環奈 (Kanna Seto)/
    └── SNOS-038 AV Debut 1st Anniversary Work.../
        ├── SNOS-038.mp4                 # Video file (or -cd1.mp4, -cd2.mp4)
        ├── SNOS-038.nfo                 # Kodi / Jellyfin Metadata XML
        ├── movie.html                   # Standalone responsive summary page
        ├── poster.jpg                   # High-Res Cover Jacket
        ├── fanart.jpg                   # Landscape Backdrop
        └── extrafanart/                 # High-Res Sample Screenshots
            ├── fanart1.jpg
            ├── fanart2.jpg
            └── fanart8.jpg
```
