package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// View renders the entire TUI interface.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading interface..."
	}

	if m.isFullscreenCover {
		return m.renderFullscreenCover()
	}

	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()

	baseView := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)

	if m.editModal.Active {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.editModal.Render(m.width),
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(bgDarkColor),
		)
	}

	return baseView
}

func (m Model) renderFullscreenCover() string {
	cur := m.currentMatch()
	if cur == nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, "No movie selected. Press 'v' or 'Esc' to return.")
	}

	titleText := cur.ID
	movie, hasMeta := m.metadataCache[cur.ID]
	if hasMeta && movie != nil && movie.Title != "" {
		titleText = fmt.Sprintf("%s - %s", cur.ID, movie.Title)
		if len(movie.Actresses) > 0 {
			var names []string
			for _, a := range movie.Actresses {
				names = append(names, a.Name)
			}
			titleText += fmt.Sprintf(" (%s)", strings.Join(names, ", "))
		}
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(0, 1).
		Width(m.width).
		Render("🖼️ FULLSCREEN COVER: " + truncateDisplay(titleText, m.width-25))

	// Calculate maximum possible resolution for full terminal screen
	fullW := max(20, m.width-4)
	fullH := max(10, m.height-5)
	var coverANSI string

	if hasMeta && movie != nil {
		targetURL := movie.CoverURL
		if targetURL == "" {
			targetURL = movie.PosterURL
		}
		// If cached locally, render instantly in ultra high resolution
		coverANSI, _ = FetchAndRenderCover(cur.ID, targetURL, m.protocol, fullW, fullH)
	} else {
		coverANSI = lipgloss.NewStyle().Foreground(mutedColor).Render("Press Enter on table to scrape metadata & cover first")
	}

	imageBox := lipgloss.Place(
		m.width,
		m.height-3,
		lipgloss.Center,
		lipgloss.Center,
		coverANSI,
	)

	footer := lipgloss.JoinHorizontal(
		lipgloss.Left,
		footerKeyStyle.Render("v / Esc"), footerDescStyle.Render("Back"),
		footerKeyStyle.Render("o"), footerDescStyle.Render("macOS Preview"),
		footerKeyStyle.Render("←/→ or ↑/↓"), footerDescStyle.Render("Prev/Next"),
		footerKeyStyle.Render("q"), footerDescStyle.Render("Quit"),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		imageBox,
		footer,
	)
}

func (m Model) renderHeader() string {
	title := titleStyle.Render("🎬 R19DEV SCRAPER")
	dirStr := truncateDisplay(m.targetDir, max(10, m.width/3))
	dir := headerSubStyle.Render("📁 " + dirStr)

	stats := lipgloss.JoinHorizontal(
		lipgloss.Center,
		statTotal.Render(fmt.Sprintf("Files: %d", len(m.files))),
		" ",
		statMatched.Render(fmt.Sprintf("Matched: %d", m.matchedCount)),
		" ",
		statUnmatched.Render(fmt.Sprintf("Unmatched: %d", m.unmatchedCount)),
		" ",
		statProto.Render(fmt.Sprintf("🎨 %s", m.ProtocolDisplayString())),
	)

	topBar := lipgloss.JoinHorizontal(lipgloss.Center, title, dir)
	dividerWidth := max(10, m.width-2)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, topBar, lipgloss.NewStyle().MarginLeft(2).Render(stats)),
		lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", dividerWidth)),
	)
}

func (m Model) renderBody() string {
	// Frame overhead:
	// Left panel: Border(2) + Padding(2) = 4 cols
	// Right panel: Border(2) + Padding(2) = 4 cols
	// Separator space between panels = 1 col
	// Total horizontal overhead = 9 cols
	totalInnerWidth := m.width - 9
	if totalInnerWidth < 40 {
		totalInnerWidth = 40
	}

	tableInnerWidth := int(float64(totalInnerWidth) * 0.50)
	if tableInnerWidth < 30 {
		tableInnerWidth = 30
	}
	detailInnerWidth := totalInnerWidth - tableInnerWidth
	if detailInnerWidth < 24 {
		detailInnerWidth = 24
	}

	// Height overhead:
	// Header: 3 rows (top margin, content, divider)
	// Footer: 2 rows (status, hints)
	// Panel vertical borders: 2 rows
	// Safety margin: 1 row
	// Total vertical overhead = 8 rows
	bodyInnerHeight := m.height - 8
	if bodyInnerHeight < 5 {
		bodyInnerHeight = 5
	}

	tableView := m.renderTable(tableInnerWidth, bodyInnerHeight)
	detailView := m.renderDetail(detailInnerWidth, bodyInnerHeight)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panelStyle.Width(tableInnerWidth).Height(bodyInnerHeight).Render(tableView),
		" ",
		detailPanelStyle.Width(detailInnerWidth).Height(bodyInnerHeight).Render(detailView),
	)
}

func (m Model) renderTable(innerWidth, innerHeight int) string {
	if m.isScanning && len(m.matches) == 0 {
		return lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, m.spinner.View(), "", "Scanning directory for videos..."))
	}

	if len(m.matches) == 0 {
		return lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, "No video files found.")
	}

	var rows []string

	// Calculate name column width with a comfortable buffer
	// Columns: cursor(2) + icon(2) + space(1) + name(W) + space(1) + ID(12) + space(1) + Part(4) + space(1) + Size(6) = W + 28
	// We set W = innerWidth - 32 so total line width is innerWidth - 4 (never wraps)
	nameColWidth := max(8, innerWidth-32)
	headerRow := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(
		fmt.Sprintf("  %-3s %-*s %-12s %-4s %-6s", "ST", nameColWidth, "FILE NAME", "JAV ID", "PART", "SIZE"),
	)
	rows = append(rows, headerRow, lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", innerWidth)))

	visibleRows := innerHeight - 2
	if visibleRows < 1 {
		visibleRows = 1
	}

	endIdx := m.scrollOffset + visibleRows
	if endIdx > len(m.matches) {
		endIdx = len(m.matches)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		match := m.matches[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "▶ "
		}

		statusIcon := "🟢"
		if match.ID == "" {
			statusIcon = "🔴"
		} else if match.IsMultiPart {
			statusIcon = "🟡"
		}

		truncName := truncateDisplay(filepath.Base(match.File.Name), nameColWidth)
		paddedName := runewidth.FillRight(truncName, nameColWidth)

		idStr := match.ID
		if idStr == "" {
			idStr = "<unmatched>"
		}
		idPadded := runewidth.FillRight(truncateDisplay(idStr, 12), 12)

		partStr := "-"
		if match.IsMultiPart {
			partStr = fmt.Sprintf("P%d", match.PartNumber)
		}
		partPadded := runewidth.FillRight(partStr, 4)

		sizeStr := fmt.Sprintf("%.0fM", match.File.SizeMB())
		sizePadded := runewidth.FillRight(sizeStr, 6)

		rowLine := fmt.Sprintf("%s%s %s %s %s %s", prefix, statusIcon, paddedName, idPadded, partPadded, sizePadded)

		if i == m.cursor {
			rows = append(rows, selectedItemStyle.Width(innerWidth).Render(rowLine))
		} else {
			rows = append(rows, normalItemStyle.Width(innerWidth).Render(rowLine))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderDetail(innerWidth, innerHeight int) string {
	cur := m.currentMatch()
	if cur == nil {
		return lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, "Select a file to view metadata")
	}

	var lines []string

	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("📄 File:"), valueStyle.Render(truncateDisplay(filepath.Base(cur.File.Name), innerWidth-12))))
	lines = append(lines, fmt.Sprintf("%s %s (%.0f MB, %s)", labelStyle.Render("JAV ID:"), lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(cur.ID), cur.File.SizeMB(), cur.MatchedBy))
	if cur.IsMultiPart {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Multi-part:"), valueStyle.Render(fmt.Sprintf("Part %d (%s)", cur.PartNumber, cur.PartSuffix))))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", innerWidth)))

	// Check R18.dev metadata
	movie, hasMeta := m.metadataCache[cur.ID]
	if hasMeta && movie != nil {
		// Display cover art if enabled
		if m.showCover {
			if coverANSI, hasCover := m.coverCache[cur.ID]; hasCover && coverANSI != "" {
				lines = append(lines, coverANSI)
				lines = append(lines, "")
			} else if m.isCoverLoading {
				lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render("   [Loading cover preview...]"), "")
			}
		}

		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Title:"), valueStyle.Render(truncateDisplay(movie.Title, innerWidth-10))))
		if movie.Maker != "" {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Studio:"), valueStyle.Render(truncateDisplay(movie.Maker, innerWidth-11))))
		}
		if movie.ReleaseDate != "" {
			lines = append(lines, fmt.Sprintf("%s %s (%d min)", labelStyle.Render("Release:"), valueStyle.Render(movie.ReleaseDate), movie.RuntimeMinutes))
		}

		if len(movie.Actresses) > 0 {
			var actNames []string
			for _, act := range movie.Actresses {
				actNames = append(actNames, act.Name)
			}
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Cast:"), lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(truncateDisplay(strings.Join(actNames, ", "), innerWidth-9))))
		}

		if len(movie.Genres) > 0 {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Genres:"), valueStyle.Render(truncateDisplay(strings.Join(movie.Genres, ", "), innerWidth-11))))
		}
	} else if errStr, hasErr := m.scrapeErrors[cur.ID]; hasErr {
		lines = append(lines, lipgloss.NewStyle().Foreground(errorColor).Render("❌ Not Found on R18.dev"))
		lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render(truncateDisplay(errStr, innerWidth-2)))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(warningColor).Render("Tip: Press 'e' to edit/override ID"))
	} else if m.isScraping {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " Fetching metadata from R18.dev..."))
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render("Press Enter to fetch R18.dev metadata"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderFooter() string {
	status := m.statusMessage
	if status == "" {
		status = "Ready"
	}

	statusLine := lipgloss.NewStyle().
		Foreground(accentColor).
		Padding(0, 1).
		Render("STATUS: " + status)

	hints := lipgloss.JoinHorizontal(
		lipgloss.Left,
		footerKeyStyle.Render("↑/↓"), footerDescStyle.Render("Navigate"),
		footerKeyStyle.Render("Enter"), footerDescStyle.Render("Scrape"),
		footerKeyStyle.Render("v"), footerDescStyle.Render("Fullscreen"),
		footerKeyStyle.Render("o"), footerDescStyle.Render("Preview"),
		footerKeyStyle.Render("s"), footerDescStyle.Render("Scrape All"),
		footerKeyStyle.Render("c"), footerDescStyle.Render("Cover"),
		footerKeyStyle.Render("e"), footerDescStyle.Render("Edit ID"),
		footerKeyStyle.Render("r"), footerDescStyle.Render("Rescan"),
		footerKeyStyle.Render("q"), footerDescStyle.Render("Quit"),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusLine,
		hints,
	)
}

// truncateDisplay safely truncates a string to fit within a given terminal column width.
func truncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	return runewidth.Truncate(s, maxWidth, "...")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
