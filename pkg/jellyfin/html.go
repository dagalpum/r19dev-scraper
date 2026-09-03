package jellyfin

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

// GenerateHTML produces a standalone, premium dark-mode HTML summary page for the movie.
func GenerateHTML(movie *scraper.Movie, userState *db.UserState) string {
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

	var actressCards strings.Builder
	for _, act := range movie.Actresses {
		nameEsc := html.EscapeString(act.Name)
		jaNameEsc := html.EscapeString(act.JaName)
		thumb := act.ImageURL
		if thumb == "" {
			thumb = "https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg"
		}
		actressCards.WriteString(fmt.Sprintf(`
			<div class="actress-card">
				<img class="actress-avatar" src="%s" alt="%s" loading="lazy" />
				<div class="actress-name">%s</div>
				<div class="actress-ja">%s</div>
			</div>`, html.EscapeString(thumb), nameEsc, nameEsc, jaNameEsc))
	}

	var genreBadges strings.Builder
	for _, g := range movie.Genres {
		genreBadges.WriteString(fmt.Sprintf(`<span class="badge genre">%s</span>`, html.EscapeString(g)))
	}

	var galleryItems strings.Builder
	for i := range movie.SampleScreenshots {
		// Link to local extrafanart first, fallback to remote
		localPath := fmt.Sprintf("extrafanart/fanart%d.jpg", i+1)
		galleryItems.WriteString(fmt.Sprintf(`
			<a class="gallery-item" href="%s" target="_blank">
				<img src="%s" alt="Screenshot %d" loading="lazy" />
			</a>`, localPath, localPath, i+1))
	}

	ratingStars := ""
	watchedBadge := `<span class="badge unwatched">👁️ Unwatched</span>`
	if userState != nil {
		if userState.IsWatched {
			watchedBadge = `<span class="badge watched">✅ Watched</span>`
		}
		if userState.UserRating > 0 {
			ratingStars = strings.Repeat("⭐", userState.UserRating)
		}
		if userState.IsFavorite {
			ratingStars += " ❤️ Favorite"
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>[%s] %s</title>
	<style>
		:root {
			--bg-dark: #11111b;
			--bg-card: #1e1e2e;
			--bg-card-hover: #313244;
			--text-main: #cdd6f4;
			--text-muted: #a6adc8;
			--primary: #89b4fa;
			--accent: #f5c2e7;
			--success: #a6e3a1;
			--warning: #f9e2af;
			--border: #45475a;
		}
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body {
			background: var(--bg-dark);
			color: var(--text-main);
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
			line-height: 1.6;
			padding: 2rem 1rem;
		}
		.container {
			max-width: 1200px;
			margin: 0 auto;
		}
		.movie-header {
			display: grid;
			grid-template-columns: 320px 1fr;
			gap: 2.5rem;
			background: var(--bg-card);
			border: 1px solid var(--border);
			border-radius: 16px;
			padding: 2rem;
			box-shadow: 0 10px 30px rgba(0,0,0,0.5);
			margin-bottom: 2.5rem;
		}
		@media (max-width: 768px) {
			.movie-header { grid-template-columns: 1fr; }
		}
		.poster-wrapper {
			position: relative;
			border-radius: 12px;
			overflow: hidden;
			box-shadow: 0 8px 24px rgba(0,0,0,0.6);
		}
		.poster-img {
			width: 100%%;
			height: auto;
			display: block;
			transition: transform 0.3s ease;
		}
		.poster-img:hover { transform: scale(1.02); }
		.movie-info h1 {
			font-size: 1.8rem;
			color: #fff;
			margin-bottom: 0.5rem;
			line-height: 1.3;
		}
		.orig-title {
			font-size: 1.1rem;
			color: var(--text-muted);
			margin-bottom: 1.2rem;
		}
		.meta-grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
			gap: 1rem;
			margin: 1.5rem 0;
			background: rgba(0,0,0,0.2);
			padding: 1.2rem;
			border-radius: 10px;
			border: 1px solid var(--border);
		}
		.meta-item .label {
			font-size: 0.8rem;
			text-transform: uppercase;
			color: var(--primary);
			font-weight: 700;
			letter-spacing: 0.5px;
		}
		.meta-item .value {
			font-size: 1rem;
			color: #fff;
			font-weight: 500;
		}
		.badge-list {
			display: flex;
			flex-wrap: wrap;
			gap: 0.5rem;
			margin-top: 1rem;
		}
		.badge {
			padding: 0.35rem 0.8rem;
			border-radius: 20px;
			font-size: 0.85rem;
			font-weight: 600;
		}
		.badge.genre {
			background: #313244;
			color: var(--text-main);
			border: 1px solid var(--border);
		}
		.badge.watched { background: rgba(166, 227, 161, 0.2); color: var(--success); border: 1px solid var(--success); }
		.badge.unwatched { background: rgba(249, 226, 175, 0.2); color: var(--warning); border: 1px solid var(--warning); }
		.badge.id-badge { background: var(--primary); color: #11111b; font-weight: 800; }
		
		/* Cast Section */
		.section-title {
			font-size: 1.4rem;
			color: #fff;
			margin: 2rem 0 1rem 0;
			display: flex;
			align-items: center;
			gap: 0.5rem;
			border-bottom: 2px solid var(--border);
			padding-bottom: 0.5rem;
		}
		.actress-grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
			gap: 1.2rem;
			margin-bottom: 2rem;
		}
		.actress-card {
			background: var(--bg-card);
			border: 1px solid var(--border);
			border-radius: 12px;
			padding: 1rem;
			text-align: center;
			transition: transform 0.2s, background 0.2s;
		}
		.actress-card:hover {
			transform: translateY(-4px);
			background: var(--bg-card-hover);
		}
		.actress-avatar {
			width: 90px;
			height: 90px;
			border-radius: 50%%;
			object-fit: cover;
			margin: 0 auto 0.8rem auto;
			border: 2px solid var(--accent);
		}
		.actress-name { font-weight: 700; color: #fff; font-size: 0.95rem; }
		.actress-ja { font-size: 0.8rem; color: var(--text-muted); }

		/* Gallery Section */
		.gallery-grid {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
			gap: 1rem;
		}
		.gallery-item {
			border-radius: 10px;
			overflow: hidden;
			border: 1px solid var(--border);
			box-shadow: 0 4px 12px rgba(0,0,0,0.4);
			transition: transform 0.2s ease, border-color 0.2s ease;
		}
		.gallery-item:hover {
			transform: scale(1.03);
			border-color: var(--primary);
		}
		.gallery-item img {
			width: 100%%;
			height: 180px;
			object-fit: cover;
			display: block;
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="movie-header">
			<div class="poster-wrapper">
				<img class="poster-img" src="poster.jpg" alt="%s Poster" onerror="this.src='fanart.jpg'" />
			</div>
			<div class="movie-info">
				<div style="display: flex; gap: 0.6rem; align-items: center; margin-bottom: 0.8rem;">
					<span class="badge id-badge">%s</span>
					%s
					<span style="font-size: 1.1rem; color: var(--warning);">%s</span>
				</div>
				<h1>%s</h1>
				<div class="orig-title">%s</div>

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
				</div>

				<div class="badge-list">
					%s
				</div>
			</div>
		</div>

		<div class="section-title">🎭 Featured Cast</div>
		<div class="actress-grid">
			%s
		</div>

		<div class="section-title">📸 Sample Screenshots (%d)</div>
		<div class="gallery-grid">
			%s
		</div>
	</div>
</body>
</html>`,
		idEsc, titleEsc,
		idEsc,
		idEsc,
		watchedBadge,
		ratingStars,
		titleEsc,
		origTitleEsc,
		makerEsc,
		movie.ReleaseDate,
		movie.RuntimeMinutes,
		directorEsc,
		genreBadges.String(),
		actressCards.String(),
		len(movie.SampleScreenshots),
		galleryItems.String(),
	)
}

// WriteHTML writes the standalone movie.html page to destPath.
func WriteHTML(movie *scraper.Movie, userState *db.UserState, destPath string) error {
	content := GenerateHTML(movie, userState)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte(content), 0o644)
}
