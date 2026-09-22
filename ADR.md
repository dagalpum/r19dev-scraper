# 🏛️ Architecture Decision Records (ADR) — R19DEV Studio & Scraper

This document records the architectural and technical decisions made in the development of `r19dev-scraper` and `R19DEV Studio`.

---

## ADR 001: Western Order (`First Last`) Canonical Actress Directory Naming

### Status
Accepted & Implemented

### Context
In Japanese media, performer names can be represented in Japanese Order (`Family Given`, e.g., `Aida Asuka`, `あいだ飛鳥`) or Western Order (`Given Family`, e.g., `Asuka Aida`). Jellyfin, Kodi, and R18.dev standardize on Western Order (`First Last`) for English metadata scrapers. Having split directories (`/organized/Aida Asuka` and `/organized/Asuka Aida`) created duplicate library sections, scattered filmographies, and confused Jellyfin scrapers.

### Decision
1. All organized destination paths MUST strictly use **Western Order (`First Last`)** (e.g., `Asuka Aida`, `Mayu Shino`, `Ria Yamate`).
2. Japanese Order folders in legacy archives (e.g. `/Archived/Aida Asuka/`) are automatically recognized as aliases and mapped to canonical Western Order.
3. Database records across `actresses`, `movie_actresses`, `movies`, and `organized_movies` store the canonical Western name as `name` and Japanese Kanji in `ja_name`.

### Consequences
- Jellyfin actress scrapers achieve 100% hit rate without manual person identification.
- Eliminates fragmented disk storage and prevents duplicate downloads.

---

## ADR 002: Offline-First Sub-Millisecond Metadata Resolution (`r18_dump.db` Tier-1)

### Status
Accepted & Implemented

### Context
Querying R18.dev's public REST API over live HTTP for thousands of movies during library scans leads to Cloudflare HTTP 429 rate limiting, IP bans, network latency (300–800ms per request), and downtime when R18.dev servers fail.

### Decision
1. Implement a Tier-1 local SQLite dump store (`r18_dump.db`) containing ~1.9 million movies and ~102,000 actresses.
2. Metadata resolution queries the local dump store first (`< 1ms` query latency, 0.00s per title).
3. Live HTTP network scraping is strictly a fallback reserved for brand-new titles not yet indexed in the dump.

### Consequences
- Complete library indexing of 7,000+ titles takes seconds instead of hours.
- Scraper operates completely offline without internet connectivity or Cloudflare bypass proxies.

---

## ADR 003: DMM Outlet (`77`/`88`) Demotion & Earliest Release Date Invariant

### Status
Accepted & Implemented

### Context
DMM/FANZA issues rental, campaign, and outlet re-releases using numerical prefixes (e.g., `77...` for DVD outlets, `88...` for Blu-ray/VOD outlets). These SKUs often have placeholder future expiration dates (e.g., `2026-07-31` on `88ssis614`, an original 2023 release). Naive deduplication logic taking `max(release_date)` caused older movies to jump to the top of "Newest Releases" in performer filmographies.

### Decision
1. **Release Date Invariant**: The canonical `release_date` of any movie variant MUST be the **earliest valid release date** (Digital Premiere or DVD release), never a future re-issue or outlet contract expiration date.
2. **SKU Demotion**: SKUs with `77` or `88` prefixes are strictly demoted and stripped during canonical matching.
3. `combined_id` resolution must prioritize authentic digital or DVD content IDs (e.g. `ssis00614` / `ssis614`) over outlet variants (`88ssis614`).
4. Standalone phantom outlet records (e.g. `77SSIS-349`) are merged into the clean canonical record and deleted.

### Consequences
- Filmographies maintain strict chronological accuracy.
- Outbound R18/DMM links point to valid product detail pages rather than dead outlet campaigns.

---

## ADR 004: SMB Network I/O Protection & Path Auto-Healing (`resolvePathToExisting`)

### Status
Accepted & Implemented

### Context
When organizing or opening folders over Samba (SMB) network shares on Synology/QNAP NAS:
1. Recursive wildcard file scanning (`filepath.Glob("*/*ID*")`) over SMB saturated I/O, stalling concurrent HTTP web requests for 30–60 seconds.
2. When switching macOS network mount points between personal shares (`/Volumes/home/BT/`) and administrative shares (`/Volumes/homes/plagad/BT/`), old paths in SQLite resulted in broken "Show in Finder" clicks.

### Decision
1. Replace recursive SMB filesystem scans with direct SQLite indexed lookups and single-level directory reads.
2. Implement `resolvePathToExisting(path, targetDir)` in `pkg/web/server.go`:
   - Checks if the recorded path exists on disk immediately.
   - If not found, systematically tests auto-healing substitutions (`/Volumes/home/` -> `/Volumes/homes/plagad/`, `/Volumes/homes/Inmad/` -> `/Volumes/homes/plagad/`).
   - Resolves target actress folder or library root gracefully if specific subfolder moved.

### Consequences
- One-click Finder actions work seamlessly regardless of whether the user mounted the volume as `/Volumes/home` or `/Volumes/homes/plagad`.
- Zero SMB network latency freezes in the web application.

---

## ADR 005: HTTP Avatar Cache Invalidation via ETag and R18 ID Versioning

### Status
Accepted & Implemented

### Context
Actress avatars served at `/api/actresses/avatar/{name}` were previously given `Cache-Control: public, max-age=31536000` (1 year). When a performer's avatar image was updated or replaced on disk with an authentic DMM photo, browser memory/disk cache refused to re-fetch the image, showing stale or placeholder avatars indefinitely.

### Decision
1. Web server (`handleActressAvatar` in `pkg/web/server.go`) now computes an MD5 checksum of the local image file, emits an `ETag: "{hash}"`, and specifies `Cache-Control: no-cache, must-revalidate`.
2. Frontend queries and API outputs append a version parameter `?v={r18_id}` to all avatar URLs (`/api/actresses/avatar/Mayu Shino?v=1096999`).
3. If an avatar file changes, the browser immediately requests the new asset or receives `304 Not Modified` with zero bandwidth waste.

### Consequences
- Immediate UI updates when actress avatars are updated or refreshed.
- Zero stale cached images in user browsers.

---

## ADR 006: Transmission JSON-RPC Integration via Dynamic Session ID Handshake

### Status
Accepted & Implemented

### Context
Direct integration with Transmission BitTorrent client is required to automate downloads of missing titles from performer filmographies. Transmission protects its JSON-RPC endpoint (`/transmission/rpc`) against Cross-Site Request Forgery (CSRF) by requiring an `X-Transmission-Session-Id` header. Any request without a valid session token receives `HTTP 409 Conflict` containing the token in the response headers.

### Decision
1. Implement native HTTP JSON-RPC client in `pkg/torrent/transmission.go` with zero external dependencies.
2. In `doRPC`, automatically intercept `HTTP 409 Conflict`, extract `X-Transmission-Session-Id`, store it thread-safely using `sync.RWMutex`, and re-issue the original request transparently.
3. Support HTTP Basic Authentication and configurable target download directories.
4. Provide `session-get` endpoint for instant connection verification from the Web UI settings modal.

### Consequences
- Zero-configuration connection management with Transmission daemon.
- Single-click torrent and magnet submission from filmography search cards.

---

## ADR 007: Smart Torrent Ranking & Scoring Algorithm

### Status
Accepted & Implemented

### Context
Searching public torrent indexers (like Sukebei Nyaa) for a specific JAV title produces multiple releases varying drastically in resolution (4K vs 480p), edition (uncensored vs censored), subtitles (Chinese/English hardsubs vs raw), and seeder availability. Operators need an automated recommendation engine to pick the best candidate.

### Decision
1. Parse Sukebei Nyaa RSS feeds (`pkg/torrent/sukebei.go`) and compute a composite score for each candidate:
   - **Resolution**: 4K/2160p/UHD (`+40 pts`), 1080p/FHD (`+20 pts`), 720p/HD (`+10 pts`).
   - **Edition**: Uncensored / 無碼 / `[UN]` (`+35 pts`).
   - **Subtitles**: Chinese / English hardsubs (`+15 pts`).
   - **Seeder Health**: $\ge 100$ seeds (`+30 pts`), $\ge 50$ seeds (`+25 pts`), $\ge 10$ seeds (`+10 pts`), $0$ seeds (`-50 pts`).
2. Sort torrent search results by composite score descending.
3. Automatically flag the top-scoring item with positive seeders as `is_recommended: true`.

### Consequences
- Operators can queue the highest quality available release in a single click without manually parsing tracker titles.
