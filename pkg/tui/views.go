package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	dir := headerSubStyle.Render("📁 " + m.targetDir)

	matchedCount := 0
	for _, match := range m.matches {
		if match.ID != "" {
			matchedCount++
		}
	}
	unmatchedCount := len(m.files) - matchedCount

	stats := lipgloss.JoinHorizontal(
		lipgloss.Center,
		statTotal.Render(fmt.Sprintf("Files: %d", len(m.files))),
		" ",
		statMatched.Render(fmt.Sprintf("Matched: %d", matchedCount)),
		" ",
		statUnmatched.Render(fmt.Sprintf("Unmatched: %d", unmatchedCount)),
	)

	topBar := lipgloss.JoinHorizontal(lipgloss.Center, title, dir)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, topBar, lipgloss.NewStyle().MarginLeft(4).Render(stats)),
		lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", m.width-4)),
	)
}

func (m Model) renderBody() string {
	bodyHeight := m.height - 7
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	tableWidth := int(float64(m.width) * 0.58)
	detailWidth := m.width - tableWidth - 6

	if tableWidth < 40 {
		tableWidth = 40
	}
	if detailWidth < 35 {
		detailWidth = 35
	}

	tableView := m.renderTable(tableWidth, bodyHeight)
	detailView := m.renderDetail(detailWidth, bodyHeight)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panelStyle.Width(tableWidth).Height(bodyHeight).Render(tableView),
		" ",
		detailPanelStyle.Width(detailWidth).Height(bodyHeight).Render(detailView),
	)
}

func (m Model) renderTable(width, height int) string {
	if m.isScanning {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, m.spinner.View(), "", "Scanning directory for videos..."))
	}

	if len(m.matches) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, "No video files found.")
	}

	var rows []string
	headerRow := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(
		fmt.Sprintf("  %-3s %-24s %-12s %-8s %-8s", "ST", "FILE NAME", "JAV ID", "PART", "SIZE"),
	)
	rows = append(rows, headerRow, lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", width-4)))

	visibleRows := height - 4
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

		truncName := truncate(filepath.Base(match.File.Name), 22)
		idStr := match.ID
		if idStr == "" {
			idStr = "<unmatched>"
		}
		partStr := "-"
		if match.IsMultiPart {
			partStr = fmt.Sprintf("Pt %d", match.PartNumber)
		}
		sizeStr := fmt.Sprintf("%.0f MB", match.File.SizeMB())

		rowContent := fmt.Sprintf("%s %-24s %-12s %-8s %-8s", statusIcon, truncName, idStr, partStr, sizeStr)

		if i == m.cursor {
			rows = append(rows, selectedItemStyle.Width(width-4).Render("▶ "+rowContent))
		} else {
			rows = append(rows, normalItemStyle.Width(width-4).Render("  "+rowContent))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderDetail(width, height int) string {
	cur := m.currentMatch()
	if cur == nil {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, "Select a file to view metadata")
	}

	var lines []string
	lines = append(lines, labelStyle.Render("📄 Selected File:"))
	lines = append(lines, valueStyle.Render(filepath.Base(cur.File.Name)))
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Size:"), valueStyle.Render(fmt.Sprintf("%.1f MB", cur.File.SizeMB()))))
	lines = append(lines, fmt.Sprintf("%s %s (%s)", labelStyle.Render("JAV ID:"), lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render(cur.ID), cur.MatchedBy))
	if cur.IsMultiPart {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Multi-part:"), valueStyle.Render(fmt.Sprintf("Part %d (%s)", cur.PartNumber, cur.PartSuffix))))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", width-4)))

	// Check R18.dev metadata
	movie, hasMeta := m.metadataCache[cur.ID]
	if hasMeta && movie != nil {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("🌐 R18.DEV METADATA"))
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Title:"), valueStyle.Render(truncate(movie.Title, width-12))))
		if movie.OriginalTitle != "" {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("JP Title:"), valueStyle.Render(truncate(movie.OriginalTitle, width-14))))
		}
		if movie.Maker != "" {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Studio:"), valueStyle.Render(movie.Maker)))
		}
		if movie.ReleaseDate != "" {
			lines = append(lines, fmt.Sprintf("%s %s (%d mins)", labelStyle.Render("Release:"), valueStyle.Render(movie.ReleaseDate), movie.RuntimeMinutes))
		}

		if len(movie.Actresses) > 0 {
			var actNames []string
			for _, act := range movie.Actresses {
				actNames = append(actNames, act.Name)
			}
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Actresses:"), lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(strings.Join(actNames, ", "))))
		}

		if len(movie.Genres) > 0 {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Genres:"), valueStyle.Render(truncate(strings.Join(movie.Genres, ", "), width-12))))
		}

		if movie.CoverURL != "" {
			lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Cover:"), lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA")).Render(truncate(movie.CoverURL, width-12))))
		}
	} else if errStr, hasErr := m.scrapeErrors[cur.ID]; hasErr {
		lines = append(lines, lipgloss.NewStyle().Foreground(errorColor).Render("❌ Metadata Not Found on R18.dev"))
		lines = append(lines, lipgloss.NewStyle().Foreground(mutedColor).Render(errStr))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(warningColor).Render("Tip: Press 'e' to edit/override the ID"))
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
		footerKeyStyle.Render("Enter"), footerDescStyle.Render("Scrape Detail"),
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
