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
	// Exact frame math:
	// panelStyle: Border(2) + Padding(2) = 4 cols
	// detailPanelStyle: Border(2) + Padding(2) = 4 cols
	// Space between panels = 1 col
	// Total horizontal overhead = 4 + 4 + 1 = 9 cols
	totalInnerWidth := m.width - 9
	if totalInnerWidth < 40 {
		totalInnerWidth = 40
	}

	tableInnerWidth := int(float64(totalInnerWidth) * 0.52)
	if tableInnerWidth < 28 {
		tableInnerWidth = 28
	}
	detailInnerWidth := totalInnerWidth - tableInnerWidth
	if detailInnerWidth < 24 {
		detailInnerWidth = 24
	}

	// Height math:
	// Header: 1 blank + 1 topBar + 1 divider = 3 rows
	// Footer: 1 status + 1 hints = 2 rows
	// Panel vertical borders: 2 rows
	// Safety margin: 1 row
	// Total vertical overhead = 3 + 2 + 2 + 1 = 8 rows
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
	if m.isScanning {
		return lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, m.spinner.View(), "", "Scanning directory for videos..."))
	}

	if len(m.matches) == 0 {
		return lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, "No video files found.")
	}

	var rows []string

	// Calculate column widths safely
	// ST (4: "🟢 ") + JAV ID (12) + PART (5) + SIZE (6) + spaces (5) = 32 cols
	nameColWidth := max(8, innerWidth-32)
	headerRow := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(
		fmt.Sprintf("  %-3s %-*s %-12s %-5s %-6s", "ST", nameColWidth, "FILE NAME", "JAV ID", "PART", "SIZE"),
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
		partPadded := runewidth.FillRight(partStr, 5)

		sizeStr := fmt.Sprintf("%.0fM", match.File.SizeMB())
		sizePadded := runewidth.FillRight(sizeStr, 6)

		rowContent := fmt.Sprintf("%s %s %s %s %s", statusIcon, paddedName, idPadded, partPadded, sizePadded)

		if i == m.cursor {
			rows = append(rows, selectedItemStyle.Width(innerWidth).Render("▶ "+rowContent))
		} else {
			rows = append(rows, normalItemStyle.Width(innerWidth).Render("  "+rowContent))
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

	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("📄 File:"), valueStyle.Render(truncateDisplay(filepath.Base(cur.File.Name), innerWidth-10))))
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

		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Title:"), valueStyle.Render(truncateDisplay(movie.Title, innerWidth-8))))
		if movie.Maker != "" {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Studio:"), valueStyle.Render(truncateDisplay(movie.Maker, innerWidth-9))))
		}
		if movie.ReleaseDate != "" {
			lines = append(lines, fmt.Sprintf("%s %s (%d min)", labelStyle.Render("Release:"), valueStyle.Render(movie.ReleaseDate), movie.RuntimeMinutes))
		}

		if len(movie.Actresses) > 0 {
			var actNames []string
			for _, act := range movie.Actresses {
				actNames = append(actNames, act.Name)
			}
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Cast:"), lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(truncateDisplay(strings.Join(actNames, ", "), innerWidth-7))))
		}

		if len(movie.Genres) > 0 {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Genres:"), valueStyle.Render(truncateDisplay(strings.Join(movie.Genres, ", "), innerWidth-9))))
		}
	} else if errStr, hasErr := m.scrapeErrors[cur.ID]; hasErr {
		lines = append(lines, lipgloss.NewStyle().Foreground(errorColor).Render("❌ Not Found on R18.dev"))
		lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render(truncateDisplay(errStr, innerWidth)))
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
		footerKeyStyle.Render("c"), footerDescStyle.Render("Cover"),
		footerKeyStyle.Render("p"), footerDescStyle.Render("Proto ["+m.ProtocolDisplayString()+"]"),
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
