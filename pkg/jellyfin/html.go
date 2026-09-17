package jellyfin

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

var reFanartFile = regexp.MustCompile(`(?i)fanart(\d+)\.(jpg|jpeg|png|webp)`)

// GenerateHTML produces a standalone, cinematic dark-mode HTML summary page for the movie.
func GenerateHTML(movie *scraper.Movie, userState *db.UserState, videoFilenames ...string) string {
	if movie == nil {
		return ""
	}

	displayTitle := strings.TrimSpace(movie.Title)
	if displayTitle == "" {
		displayTitle = strings.TrimSpace(movie.OriginalTitle)
	}
	titleEsc := html.EscapeString(displayTitle)
	origTitleEsc := html.EscapeString(movie.OriginalTitle)
	idEsc := html.EscapeString(movie.ID)
	makerEsc := html.EscapeString(movie.Maker)
	directorEsc := html.EscapeString(movie.Director)
	if directorEsc == "" {
		directorEsc = "N/A"
	}

	// Actress Cards
	var actressCards strings.Builder
	for _, act := range movie.Actresses {
		nameEsc := html.EscapeString(act.Name)
		jaNameEsc := html.EscapeString(act.JaName)
		thumb := strings.TrimSpace(act.ImageURL)
		if thumb != "" && !strings.HasPrefix(thumb, "http://") && !strings.HasPrefix(thumb, "https://") {
			thumb = "https://pics.dmm.co.jp/mono/actjpgs/" + thumb
		}
		if thumb == "" {
			thumb = "https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg"
		}
		actressCards.WriteString(fmt.Sprintf(`
			<div class="actress-card">
				<img class="actress-avatar" src="%s" alt="%s" loading="lazy" onerror="this.src='https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg'" />
				<div class="actress-name">%s</div>
				<div class="actress-ja">%s</div>
			</div>`, html.EscapeString(thumb), nameEsc, nameEsc, jaNameEsc))
	}
	if len(movie.Actresses) == 0 {
		actressCards.WriteString(`<div style="color: var(--text-muted); font-size: 0.9rem; padding: 0.5rem 0;">No cast information.</div>`)
	}

	// Genre Badges
	var genreBadges strings.Builder
	for _, g := range movie.Genres {
		genreBadges.WriteString(fmt.Sprintf(`<span class="badge genre">%s</span>`, html.EscapeString(g)))
	}

	// Action Play Buttons (Handles single or multi-part videos)
	var playButtons strings.Builder
	cleanVideos := make([]string, 0, len(videoFilenames))
	for _, vf := range videoFilenames {
		if strings.TrimSpace(vf) != "" {
			cleanVideos = append(cleanVideos, strings.TrimSpace(vf))
		}
	}
	if len(cleanVideos) == 0 {
		cleanVideos = append(cleanVideos, fmt.Sprintf("%s.mp4", movie.ID))
	}

	if len(cleanVideos) > 1 {
		for i, vf := range cleanVideos {
			vEsc := html.EscapeString(vf)
			playButtons.WriteString(fmt.Sprintf(`<a class="btn btn-play" href="%s">▶ Play Part %d</a> `, vEsc, i+1))
		}
		for i, vf := range cleanVideos {
			vEsc := html.EscapeString(vf)
			playButtons.WriteString(fmt.Sprintf(`<button class="btn btn-secondary" onclick="openPlayer('%s')">🖥️ Watch Part %d</button> `, vEsc, i+1))
		}
	} else {
		vEsc := html.EscapeString(cleanVideos[0])
		playButtons.WriteString(fmt.Sprintf(`<a class="btn btn-play" href="%s">▶ Play Movie</a> `, vEsc))
		playButtons.WriteString(fmt.Sprintf(`<button class="btn btn-secondary" onclick="openPlayer('%s')">🖥️ Watch in Browser</button> `, vEsc))
	}

	if movie.TrailerURL != "" {
		playButtons.WriteString(fmt.Sprintf(`<button class="btn btn-secondary" onclick="openPlayer('%s')">🎬 Watch Trailer</button> `, html.EscapeString(movie.TrailerURL)))
	}
	playButtons.WriteString(fmt.Sprintf(`<a class="btn btn-secondary" href="http://localhost:8080/#movie/%s" target="_blank" title="Open in R19dev Web Dashboard">🏠 R19dev Hub</a>`, idEsc))

	// Sample Screenshots
	var galleryItems strings.Builder
	var galleryURLs []string
	for i, shot := range movie.SampleScreenshots {
		localPath := shot
		if !strings.HasPrefix(shot, "http://") && !strings.HasPrefix(shot, "https://") && !strings.HasPrefix(shot, "extrafanart/") {
			localPath = filepath.Join("extrafanart", shot)
		}
		galleryURLs = append(galleryURLs, localPath)
		galleryItems.WriteString(fmt.Sprintf(`
			<div class="gallery-thumb" onclick="openLightbox(%d)" title="View screenshot %d">
				<img src="%s" alt="Screenshot %d" loading="lazy" onerror="this.parentElement.style.display='none'" />
			</div>`, i, i+1, html.EscapeString(localPath), i+1))
	}
	if len(galleryURLs) == 0 {
		galleryItems.WriteString(`<div style="color: var(--text-muted); font-size: 0.9rem; padding: 1rem 0;">No sample screenshots available.</div>`)
	}

	galleryJSON, _ := json.Marshal(galleryURLs)

	// User State Badges
	ratingBadge := ""
	watchedBadge := `<span class="badge unwatched">👁️ Unwatched</span>`
	if userState != nil {
		if userState.IsWatched {
			watchedBadge = `<span class="badge watched">✅ Watched</span>`
		}
		if userState.UserRating > 0 {
			ratingBadge = fmt.Sprintf(`<span class="badge rating">⭐ %d/10</span>`, userState.UserRating)
		}
		if userState.IsFavorite {
			ratingBadge += ` <span class="badge rating">❤️ Favorite</span>`
		}
	}

	// Series and Label Rows
	seriesRow := ""
	if movie.Series != "" {
		seriesRow = fmt.Sprintf(`<div class="meta-item"><div class="label">Series</div><div class="value">%s</div></div>`, html.EscapeString(movie.Series))
	}
	labelRow := ""
	if movie.Label != "" {
		labelRow = fmt.Sprintf(`<div class="meta-item"><div class="label">Label</div><div class="value">%s</div></div>`, html.EscapeString(movie.Label))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>[%s] %s</title>
	<style>
		:root {
			--bg-base: #0c0d14;
			--bg-surface: #141522;
			--bg-card: rgba(22, 24, 38, 0.78);
			--bg-card-hover: rgba(35, 38, 58, 0.92);
			--text-main: #f1f5f9;
			--text-muted: #94a3b8;
			--primary: #8b5cf6;
			--primary-hover: #7c3aed;
			--primary-light: #c4b5fd;
			--accent: #ec4899;
			--success: #10b981;
			--warning: #f59e0b;
			--border: rgba(255, 255, 255, 0.09);
			--border-glow: rgba(139, 92, 246, 0.35);
			--glass: rgba(16, 18, 28, 0.78);
		}
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body {
			background: var(--bg-base);
			color: var(--text-main);
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
			line-height: 1.6;
			overflow-x: hidden;
			min-height: 100vh;
		}

		/* Cinematic Backdrop Hero */
		.backdrop-container {
			position: absolute;
			top: 0;
			left: 0;
			width: 100%%;
			height: 580px;
			overflow: hidden;
			z-index: 0;
			pointer-events: none;
		}
		.backdrop-img {
			width: 100%%;
			height: 100%%;
			object-fit: cover;
			filter: blur(30px) brightness(0.32) saturate(1.25);
			transform: scale(1.15);
		}
		.backdrop-overlay {
			position: absolute;
			inset: 0;
			background: linear-gradient(180deg, rgba(12, 13, 20, 0.25) 0%%, rgba(12, 13, 20, 0.75) 65%%, var(--bg-base) 100%%);
		}

		.container {
			max-width: 1240px;
			margin: 0 auto;
			padding: 2.5rem 1.5rem;
			position: relative;
			z-index: 1;
		}

		/* Movie Header Card */
		.movie-header {
			display: grid;
			grid-template-columns: minmax(380px, 480px) 1fr;
			gap: 2.5rem;
			background: var(--glass);
			backdrop-filter: blur(20px);
			-webkit-backdrop-filter: blur(20px);
			border: 1px solid var(--border);
			border-radius: 20px;
			padding: 2.2rem;
			box-shadow: 0 20px 48px rgba(0,0,0,0.65);
			margin-bottom: 2.5rem;
			align-items: start;
		}
		@media (max-width: 1024px) {
			.movie-header { grid-template-columns: 1fr; gap: 1.8rem; }
		}

		/* Cover Jacket */
		.poster-wrapper {
			position: relative;
			border-radius: 14px;
			overflow: hidden;
			box-shadow: 0 12px 36px rgba(0,0,0,0.75);
			border: 1px solid var(--border);
			background: #08080f;
			display: flex;
			align-items: center;
			justify-content: center;
			cursor: zoom-in;
		}
		.poster-img {
			width: 100%%;
			height: auto;
			display: block;
			transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1);
		}
		.poster-wrapper:hover .poster-img {
			transform: scale(1.025);
		}
		.poster-zoom-hint {
			position: absolute;
			bottom: 12px;
			right: 12px;
			background: rgba(12, 13, 20, 0.82);
			backdrop-filter: blur(6px);
			border: 1px solid rgba(255, 255, 255, 0.2);
			color: #fff;
			padding: 0.35rem 0.75rem;
			border-radius: 8px;
			font-size: 0.8rem;
			font-weight: 700;
			opacity: 0;
			transition: opacity 0.2s ease;
			pointer-events: none;
		}
		.poster-wrapper:hover .poster-zoom-hint {
			opacity: 1;
		}

		/* Top Badges & ID */
		.badge-row {
			display: flex;
			flex-wrap: wrap;
			gap: 0.6rem;
			align-items: center;
			margin-bottom: 1rem;
		}
		.id-badge {
			background: linear-gradient(135deg, #8b5cf6, #6366f1);
			color: #fff;
			font-weight: 800;
			font-size: 0.95rem;
			padding: 0.45rem 1rem;
			border-radius: 8px;
			cursor: pointer;
			display: inline-flex;
			align-items: center;
			gap: 0.45rem;
			transition: all 0.2s ease;
			user-select: none;
			box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
		}
		.id-badge:hover {
			transform: translateY(-2px);
			box-shadow: 0 6px 18px rgba(139, 92, 246, 0.5);
			background: linear-gradient(135deg, #9d6ff9, #7c3aed);
		}
		.id-badge:active { transform: translateY(0); }

		.badge {
			padding: 0.4rem 0.85rem;
			border-radius: 20px;
			font-size: 0.82rem;
			font-weight: 600;
			display: inline-flex;
			align-items: center;
			gap: 0.35rem;
		}
		.badge.watched { background: rgba(16, 185, 129, 0.16); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.35); }
		.badge.unwatched { background: rgba(245, 158, 11, 0.16); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.35); }
		.badge.rating { background: rgba(236, 72, 153, 0.16); color: #f472b6; border: 1px solid rgba(236, 72, 153, 0.35); }
		.badge.genre {
			background: rgba(255, 255, 255, 0.06);
			color: var(--text-main);
			border: 1px solid var(--border);
			transition: all 0.2s ease;
		}
		.badge.genre:hover {
			background: rgba(139, 92, 246, 0.2);
			border-color: var(--primary-light);
		}

		/* Titles */
		.movie-info h1 {
			font-size: 1.65rem;
			font-weight: 700;
			color: #fff;
			line-height: 1.35;
			margin-bottom: 0.5rem;
		}
		.orig-title {
			font-size: 0.95rem;
			color: var(--text-muted);
			margin-bottom: 1.3rem;
			line-height: 1.45;
		}

		/* Action Bar */
		.action-bar {
			display: flex;
			flex-wrap: wrap;
			gap: 0.75rem;
			margin: 1.4rem 0 1.6rem 0;
			align-items: center;
		}
		.btn {
			display: inline-flex;
			align-items: center;
			gap: 0.5rem;
			padding: 0.65rem 1.3rem;
			border-radius: 10px;
			font-size: 0.92rem;
			font-weight: 600;
			text-decoration: none;
			cursor: pointer;
			border: none;
			transition: all 0.2s ease;
		}
		.btn-play {
			background: linear-gradient(135deg, #8b5cf6, #6d28d9);
			color: #fff;
			box-shadow: 0 4px 16px rgba(139, 92, 246, 0.45);
		}
		.btn-play:hover {
			background: linear-gradient(135deg, #9d6ff9, #7c3aed);
			transform: translateY(-2px);
			box-shadow: 0 6px 22px rgba(139, 92, 246, 0.6);
		}
		.btn-secondary {
			background: rgba(255, 255, 255, 0.08);
			color: var(--text-main);
			border: 1px solid var(--border);
		}
		.btn-secondary:hover {
			background: rgba(255, 255, 255, 0.16);
			transform: translateY(-2px);
		}

		/* Metadata Grid */
		.meta-grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
			gap: 0.9rem;
			background: rgba(0, 0, 0, 0.38);
			padding: 1.2rem;
			border-radius: 12px;
			border: 1px solid var(--border);
			margin-bottom: 1.4rem;
		}
		.meta-item .label {
			font-size: 0.75rem;
			text-transform: uppercase;
			color: var(--primary-light);
			font-weight: 700;
			letter-spacing: 0.5px;
			margin-bottom: 0.25rem;
		}
		.meta-item .value {
			font-size: 0.95rem;
			color: #fff;
			font-weight: 500;
		}

		.badge-list {
			display: flex;
			flex-wrap: wrap;
			gap: 0.5rem;
		}

		/* Section Titles */
		.section-header {
			display: flex;
			align-items: center;
			justify-content: space-between;
			margin: 2.5rem 0 1.2rem 0;
			padding-bottom: 0.6rem;
			border-bottom: 1px solid var(--border);
		}
		.section-title {
			font-size: 1.35rem;
			font-weight: 700;
			color: #fff;
			display: flex;
			align-items: center;
			gap: 0.6rem;
		}

		/* Cast Cards */
		.actress-grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
			gap: 1.2rem;
			margin-bottom: 2rem;
		}
		.actress-card {
			background: var(--bg-card);
			border: 1px solid var(--border);
			border-radius: 14px;
			padding: 1.2rem 0.8rem;
			text-align: center;
			transition: all 0.25s ease;
		}
		.actress-card:hover {
			transform: translateY(-4px);
			background: var(--bg-card-hover);
			border-color: var(--border-glow);
		}
		.actress-avatar {
			width: 88px;
			height: 88px;
			border-radius: 50%%;
			object-fit: cover;
			margin: 0 auto 0.8rem auto;
			border: 2px solid var(--primary);
			box-shadow: 0 4px 14px rgba(0,0,0,0.5);
			display: block;
			background: #181825;
		}
		.actress-name { font-weight: 700; color: #fff; font-size: 0.95rem; margin-bottom: 0.2rem; }
		.actress-ja { font-size: 0.8rem; color: var(--text-muted); }

		/* Gallery Grid */
		.gallery-grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
			gap: 1rem;
		}
		.gallery-thumb {
			position: relative;
			border-radius: 10px;
			overflow: hidden;
			border: 1px solid var(--border);
			box-shadow: 0 4px 14px rgba(0,0,0,0.45);
			cursor: pointer;
			transition: all 0.25s ease;
			aspect-ratio: 16 / 9;
			background: #151522;
		}
		.gallery-thumb:hover {
			transform: scale(1.03);
			border-color: var(--primary);
			box-shadow: 0 8px 24px rgba(139, 92, 246, 0.4);
		}
		.gallery-thumb img {
			width: 100%%;
			height: 100%%;
			object-fit: cover;
			display: block;
		}

		/* Lightbox Modal */
		.lightbox-modal {
			position: fixed;
			inset: 0;
			background: rgba(5, 5, 10, 0.95);
			backdrop-filter: blur(14px);
			z-index: 1000;
			display: none;
			align-items: center;
			justify-content: center;
			flex-direction: column;
		}
		.lightbox-modal.active { display: flex; }
		.lightbox-content {
			position: relative;
			max-width: 90vw;
			max-height: 82vh;
			display: flex;
			align-items: center;
			justify-content: center;
		}
		.lightbox-img {
			max-width: 100%%;
			max-height: 82vh;
			border-radius: 12px;
			box-shadow: 0 12px 48px rgba(0,0,0,0.9);
			border: 1px solid rgba(255,255,255,0.1);
			object-fit: contain;
		}
		.lightbox-nav {
			position: absolute;
			top: 50%%;
			transform: translateY(-50%%);
			background: rgba(255,255,255,0.14);
			color: #fff;
			border: none;
			border-radius: 50%%;
			width: 48px;
			height: 48px;
			font-size: 1.5rem;
			display: flex;
			align-items: center;
			justify-content: center;
			cursor: pointer;
			transition: all 0.2s;
		}
		.lightbox-nav:hover { background: var(--primary); transform: translateY(-50%%) scale(1.1); }
		.lightbox-prev { left: -64px; }
		.lightbox-next { right: -64px; }
		@media (max-width: 820px) {
			.lightbox-prev { left: 10px; }
			.lightbox-next { right: 10px; }
		}
		.lightbox-close {
			position: absolute;
			top: 24px;
			right: 28px;
			background: rgba(255,255,255,0.15);
			color: #fff;
			border: none;
			border-radius: 50%%;
			width: 40px;
			height: 40px;
			font-size: 1.3rem;
			cursor: pointer;
			display: flex;
			align-items: center;
			justify-content: center;
			transition: all 0.2s;
		}
		.lightbox-close:hover { background: #ef4444; }
		.lightbox-counter {
			color: var(--text-muted);
			font-size: 0.9rem;
			margin-top: 1rem;
		}

		/* Video Player Modal */
		.player-modal {
			position: fixed;
			inset: 0;
			background: rgba(0, 0, 0, 0.94);
			backdrop-filter: blur(16px);
			z-index: 1000;
			display: none;
			align-items: center;
			justify-content: center;
			padding: 2rem;
		}
		.player-modal.active { display: flex; }
		.player-box {
			position: relative;
			width: 100%%;
			max-width: 1040px;
			background: #000;
			border-radius: 16px;
			overflow: hidden;
			box-shadow: 0 16px 48px rgba(0,0,0,0.85);
			border: 1px solid var(--border);
		}
		.player-box video {
			width: 100%%;
			height: auto;
			max-height: 80vh;
			display: block;
		}
		.player-close {
			position: absolute;
			top: 16px;
			right: 16px;
			background: rgba(0,0,0,0.65);
			color: #fff;
			border: 1px solid rgba(255,255,255,0.25);
			border-radius: 50%%;
			width: 38px;
			height: 38px;
			font-size: 1.2rem;
			cursor: pointer;
			display: flex;
			align-items: center;
			justify-content: center;
			z-index: 10;
		}
		.player-close:hover { background: #ef4444; border-color: #ef4444; }

		/* Toast Notification */
		.toast {
			position: fixed;
			bottom: 24px;
			right: 24px;
			background: #1e1e2e;
			color: #fff;
			padding: 0.85rem 1.4rem;
			border-radius: 10px;
			box-shadow: 0 8px 24px rgba(0,0,0,0.6);
			border: 1px solid var(--primary);
			font-size: 0.9rem;
			display: none;
			align-items: center;
			gap: 0.5rem;
			z-index: 2000;
			animation: slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
		}
		@keyframes slideUp { from { transform: translateY(20px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
	</style>
</head>
<body>
	<!-- Cinematic Backdrop -->
	<div class="backdrop-container">
		<img class="backdrop-img" src="fanart.jpg" alt="" onerror="if(this.src.indexOf('poster.jpg')===-1){this.src='poster.jpg';}else{this.parentElement.style.display='none';}" />
		<div class="backdrop-overlay"></div>
	</div>

	<div class="container">
		<!-- Main Movie Header -->
		<div class="movie-header">
			<div class="poster-wrapper" onclick="openCoverLightbox()" title="Click to view full cover in high resolution">
				<img class="poster-img" src="poster.jpg" alt="%s Poster" onerror="if(this.src.indexOf('folder.jpg')===-1){this.src='folder.jpg';}else{this.src='fanart.jpg';}" />
				<div class="poster-zoom-hint">🔍 Full Cover</div>
			</div>
			<div class="movie-info">
				<div class="badge-row">
					<span class="id-badge" onclick="copyId('%s')" title="Click to copy JAV ID">📋 %s</span>
					%s
					%s
				</div>
				<h1>%s</h1>
				<div class="orig-title">%s</div>

				<!-- Action Bar -->
				<div class="action-bar">
					%s
				</div>

				<!-- Metadata Grid -->
				<div class="meta-grid">
					<div class="meta-item">
						<div class="label">Studio / Maker</div>
						<div class="value">%s</div>
					</div>
					<div class="meta-item">
						<div class="label">Release Date</div>
						<div class="value">%s</div>
					</div>
					<div class="meta-item">
						<div class="label">Duration</div>
						<div class="value">%d mins</div>
					</div>
					<div class="meta-item">
						<div class="label">Director</div>
						<div class="value">%s</div>
					</div>
					%s
					%s
				</div>

				<!-- Genres -->
				<div class="badge-list">
					%s
				</div>
			</div>
		</div>

		<!-- Cast Section -->
		<div class="section-header">
			<div class="section-title">🎭 Featured Cast</div>
		</div>
		<div class="actress-grid">
			%s
		</div>

		<!-- Screenshots Section -->
		<div class="section-header">
			<div class="section-title">📸 Sample Screenshots (%d)</div>
		</div>
		<div class="gallery-grid">
			%s
		</div>
	</div>

	<!-- Lightbox Modal -->
	<div id="lightbox" class="lightbox-modal" onclick="closeLightbox()">
		<button class="lightbox-close" onclick="closeLightbox()" title="Close (Esc)">✕</button>
		<div class="lightbox-content" onclick="event.stopPropagation()">
			<button class="lightbox-nav lightbox-prev" onclick="prevImage(event)" title="Previous (Left Arrow)">‹</button>
			<img id="lightbox-img" class="lightbox-img" src="" alt="Screenshot Fullscreen" />
			<button class="lightbox-nav lightbox-next" onclick="nextImage(event)" title="Next (Right Arrow)">›</button>
		</div>
		<div id="lightbox-counter" class="lightbox-counter">1 / 1</div>
	</div>

	<!-- Video Player Modal -->
	<div id="player-modal" class="player-modal" onclick="closePlayer()">
		<div class="player-box" onclick="event.stopPropagation()">
			<button class="player-close" onclick="closePlayer()" title="Close Player">✕</button>
			<video id="main-video" controls preload="metadata"></video>
		</div>
	</div>

	<!-- Toast -->
	<div id="toast" class="toast"></div>

	<script>
		// Copy ID
		function copyId(text) {
			navigator.clipboard.writeText(text).then(() => {
				showToast('Copied ' + text + ' to clipboard!');
			}).catch(() => {
				showToast('ID: ' + text);
			});
		}

		function showToast(msg) {
			const toast = document.getElementById('toast');
			toast.textContent = msg;
			toast.style.display = 'flex';
			setTimeout(() => { toast.style.display = 'none'; }, 2400);
		}

		// Lightbox Gallery
		const galleryImages = %s;
		let currentGalleryIndex = 0;

		function openCoverLightbox() {
			document.getElementById('lightbox-img').src = 'poster.jpg';
			document.getElementById('lightbox-counter').textContent = 'Cover Jacket (Full Resolution)';
			document.getElementById('lightbox').classList.add('active');
		}

		function openLightbox(index) {
			if (!galleryImages || galleryImages.length === 0) return;
			currentGalleryIndex = index;
			updateLightbox();
			document.getElementById('lightbox').classList.add('active');
		}

		function closeLightbox() {
			document.getElementById('lightbox').classList.remove('active');
		}

		function nextImage(e) {
			if (e) e.stopPropagation();
			if (!galleryImages || !galleryImages.length) return;
			currentGalleryIndex = (currentGalleryIndex + 1) %% galleryImages.length;
			updateLightbox();
		}

		function prevImage(e) {
			if (e) e.stopPropagation();
			if (!galleryImages || !galleryImages.length) return;
			currentGalleryIndex = (currentGalleryIndex - 1 + galleryImages.length) %% galleryImages.length;
			updateLightbox();
		}

		function updateLightbox() {
			document.getElementById('lightbox-img').src = galleryImages[currentGalleryIndex];
			document.getElementById('lightbox-counter').textContent = (currentGalleryIndex + 1) + ' / ' + galleryImages.length;
		}

		// Embedded Video Player
		function openPlayer(src) {
			const pm = document.getElementById('player-modal');
			const vid = document.getElementById('main-video');
			vid.src = src;
			pm.classList.add('active');
			vid.play().catch(() => {});
		}

		function closePlayer() {
			const pm = document.getElementById('player-modal');
			const vid = document.getElementById('main-video');
			vid.pause();
			vid.removeAttribute('src');
			vid.load();
			pm.classList.remove('active');
		}

		// Keyboard Shortcuts
		document.addEventListener('keydown', (e) => {
			const lb = document.getElementById('lightbox');
			if (lb && lb.classList.contains('active')) {
				if (e.key === 'ArrowRight') nextImage();
				if (e.key === 'ArrowLeft') prevImage();
				if (e.key === 'Escape') closeLightbox();
			}
			const pm = document.getElementById('player-modal');
			if (pm && pm.classList.contains('active')) {
				if (e.key === 'Escape') closePlayer();
			}
		});
	</script>
</body>
</html>`,
		idEsc, titleEsc,
		idEsc,
		idEsc, idEsc,
		watchedBadge,
		ratingBadge,
		titleEsc,
		origTitleEsc,
		playButtons.String(),
		makerEsc,
		movie.ReleaseDate,
		movie.RuntimeMinutes,
		directorEsc,
		seriesRow,
		labelRow,
		genreBadges.String(),
		actressCards.String(),
		len(galleryURLs),
		galleryItems.String(),
		string(galleryJSON),
	)
}

// WriteHTML writes the standalone movie.html page to destPath, auto-discovering local assets if needed.
func WriteHTML(movie *scraper.Movie, userState *db.UserState, destPath string, videoFilenames ...string) error {
	if movie == nil {
		return fmt.Errorf("movie is nil")
	}

	movieDir := filepath.Dir(destPath)

	// 1. Auto-discover local extrafanart screenshots if movie.SampleScreenshots is empty
	if len(movie.SampleScreenshots) == 0 {
		extraDir := filepath.Join(movieDir, "extrafanart")
		if entries, err := os.ReadDir(extraDir); err == nil {
			type indexedFile struct {
				idx  int
				path string
			}
			var indexed []indexedFile
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				m := reFanartFile.FindStringSubmatch(entry.Name())
				if len(m) >= 2 {
					if num, err := strconv.Atoi(m[1]); err == nil {
						indexed = append(indexed, indexedFile{idx: num, path: filepath.Join("extrafanart", entry.Name())})
					}
				}
			}
			sort.Slice(indexed, func(i, j int) bool {
				return indexed[i].idx < indexed[j].idx
			})
			for _, item := range indexed {
				movie.SampleScreenshots = append(movie.SampleScreenshots, item.path)
			}
		}
	}

	// 2. Auto-discover local video files if none explicitly provided
	if len(videoFilenames) == 0 || (len(videoFilenames) == 1 && strings.TrimSpace(videoFilenames[0]) == "") {
		if entries, err := os.ReadDir(movieDir); err == nil {
			var foundVideos []string
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".wmv" || ext == ".ts" || ext == ".m4v" {
					foundVideos = append(foundVideos, entry.Name())
				}
			}
			sort.Strings(foundVideos)
			if len(foundVideos) > 0 {
				videoFilenames = foundVideos
			}
		}
	}

	content := GenerateHTML(movie, userState, videoFilenames...)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte(content), 0o644)
}
