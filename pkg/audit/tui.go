package audit

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dagalp/r19dev-scraper/pkg/db"
)

// Styling constants
var (
	colorBgDark   = lipgloss.Color("#1E1E2E")
	colorBgPanel  = lipgloss.Color("#181825")
	colorPrimary  = lipgloss.Color("#7D56F4") // Purple
	colorSuccess  = lipgloss.Color("#04B575") // Emerald Green
	colorWarning  = lipgloss.Color("#FFB800") // Amber
	colorError    = lipgloss.Color("#FF4C4C") // Red
	colorCyan     = lipgloss.Color("#00D2FF") // Cyan
	colorMuted    = lipgloss.Color("#6B7280") // Gray
	colorText     = lipgloss.Color("#CDD6F4") // Light Text
	colorSelected = lipgloss.Color("#45475A")

	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Padding(0, 2).
				MarginRight(1)

	badgeComplete = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorSuccess).
			Padding(0, 1).
			Bold(true)

	badgeIncomplete = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(colorWarning).
			Padding(0, 1).
			Bold(true)

	badgeNoVideo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorError).
			Padding(0, 1).
			Bold(true)

	leftPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	rightPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	fixBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Background(lipgloss.Color("#181825")).
			Padding(0, 1).
			MarginTop(1)

	footerKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#313244")).
			Padding(0, 1).
			Bold(true)

	footerDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginRight(2)
)

// Messages
type scanDoneMsg struct {
	results []*MovieAudit
	err     error
}

type auditProgressMsg AuditProgressEvent

type fixProgressMsg FixProgressEvent

type fixSingleDoneMsg struct {
	item *MovieAudit
	err  error
}

// TUIModel represents the audit TUI application state.
type TUIModel struct {
	targetDir     string
	auditor       *Auditor
	items         []*MovieAudit
	filtered      []*MovieAudit
	filterMode    string // "all", "incomplete", "no_video"
	searchQuery   string
	searching     bool
	cursor        int
	scrollOffset  int
	width         int
	height        int

	isScanning   bool
	scanProgress AuditProgressEvent
	progressChan chan AuditProgressEvent

	isFixing        bool
	fixingID        string
	fixingStatus    string
	fixProgress     FixProgressEvent
	fixProgressChan chan FixProgressEvent

	statusMessage string

	// Batch fix state
	batchFixActive bool
	batchFixQueue  []*MovieAudit
	batchFixDone   int
	batchFixTotal  int

	spinner spinner.Model
}

// NewTUI creates a new audit TUI model.
func NewTUI(targetDir string, d *db.DB) (*TUIModel, error) {
	aud, err := New(d)
	if err != nil {
		return nil, err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)

	return &TUIModel{
		targetDir:    targetDir,
		auditor:      aud,
		filterMode:   "all",
		isScanning:   true,
		spinner:      s,
		progressChan: make(chan AuditProgressEvent, 200),
	}, nil
}

// Init initializes the tea program and kicks off scanning.
func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startScanStreamCmd(),
		m.waitForProgressCmd(),
	)
}

func (m *TUIModel) startScanStreamCmd() tea.Cmd {
	return func() tea.Msg {
		results, err := m.auditor.AuditDirectoryWithProgress(context.Background(), m.targetDir, m.progressChan)
		close(m.progressChan)
		return scanDoneMsg{results: results, err: err}
	}
}

func (m *TUIModel) waitForProgressCmd() tea.Cmd {
	return func() tea.Msg {
		p, ok := <-m.progressChan
		if !ok {
			return nil
		}
		return auditProgressMsg(p)
	}
}

func (m *TUIModel) startFixItemCmd(item *MovieAudit) tea.Cmd {
	m.fixProgressChan = make(chan FixProgressEvent, 100)
	return tea.Batch(
		m.runFixItemWorkerCmd(item),
		m.waitForFixProgressCmd(),
	)
}

func (m *TUIModel) runFixItemWorkerCmd(item *MovieAudit) tea.Cmd {
	return func() tea.Msg {
		err := m.auditor.FixMovieWithProgress(context.Background(), item, m.fixProgressChan)
		if m.fixProgressChan != nil {
			close(m.fixProgressChan)
		}
		return fixSingleDoneMsg{item: item, err: err}
	}
}

func (m *TUIModel) waitForFixProgressCmd() tea.Cmd {
	return func() tea.Msg {
		p, ok := <-m.fixProgressChan
		if !ok {
			return nil
		}
		return fixProgressMsg(p)
	}
}

// Update handles tea events and key messages.
func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case auditProgressMsg:
		m.scanProgress = AuditProgressEvent(msg)
		if m.scanProgress.Item != nil {
			m.items = append(m.items, m.scanProgress.Item)
		}
		return m, m.waitForProgressCmd()

	case scanDoneMsg:
		m.isScanning = false
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Scan Error: %v", msg.err)
			return m, nil
		}
		m.items = msg.results
		m.applyFilter()
		m.statusMessage = fmt.Sprintf("✨ Scan complete: %d movies audited", len(m.items))
		return m, nil

	case fixProgressMsg:
		m.fixProgress = FixProgressEvent(msg)
		if m.fixProgress.Message != "" {
			m.fixingStatus = m.fixProgress.Message
		}
		return m, m.waitForFixProgressCmd()

	case fixSingleDoneMsg:
		m.isFixing = false
		m.fixingID = ""
		m.fixProgress = FixProgressEvent{}
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("⚠️ %s: %v", msg.item.MovieID, msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("✅ Verified & 100%% complete for %s (Rechecked from disk)", msg.item.MovieID)
		}
		m.applyFilter()

		// If in batch fix mode, trigger next
		if m.batchFixActive {
			m.batchFixDone++
			if len(m.batchFixQueue) > 0 {
				nextItem := m.batchFixQueue[0]
				m.batchFixQueue = m.batchFixQueue[1:]
				m.isFixing = true
				m.fixingID = nextItem.MovieID
				m.statusMessage = fmt.Sprintf("⏳ Batch repairing (%d/%d): %s...", m.batchFixDone+1, m.batchFixTotal, nextItem.MovieID)
				return m, m.startFixItemCmd(nextItem)
			}
			m.batchFixActive = false
			m.statusMessage = fmt.Sprintf("🎉 Batch repair completed! Rechecked and updated %d movies.", m.batchFixDone)
		}
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyEnter:
				m.searching = false
				return m, nil
			case tea.KeyBackspace:
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.applyFilter()
				}
				return m, nil
			case tea.KeyRunes:
				m.searchQuery += string(msg.Runes)
				m.applyFilter()
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scrollOffset {
					m.scrollOffset = m.cursor
				}
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				maxVisible := m.listHeight()
				if m.cursor >= m.scrollOffset+maxVisible {
					m.scrollOffset = m.cursor - maxVisible + 1
				}
			}
			return m, nil

		case "tab":
			// Cycle filter mode: all -> incomplete -> no_video -> all
			switch m.filterMode {
			case "all":
				m.filterMode = "incomplete"
			case "incomplete":
				m.filterMode = "no_video"
			default:
				m.filterMode = "all"
			}
			m.cursor = 0
			m.scrollOffset = 0
			m.applyFilter()
			return m, nil

		case "/":
			m.searching = true
			return m, nil

		case "c":
			// Clear search
			m.searchQuery = ""
			m.applyFilter()
			return m, nil

		case "r":
			// Rescan
			if !m.isScanning && !m.isFixing {
				m.isScanning = true
				m.items = nil
				m.filtered = nil
				m.progressChan = make(chan AuditProgressEvent, 200)
				m.statusMessage = "Starting library re-scan..."
				return m, tea.Batch(
					m.startScanStreamCmd(),
					m.waitForProgressCmd(),
				)
			}
			return m, nil

		case "o":
			// Open selected folder in Finder
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				sel := m.filtered[m.cursor]
				if runtime.GOOS == "darwin" {
					_ = exec.Command("open", sel.FolderPath).Start()
					m.statusMessage = fmt.Sprintf("📂 Opened in Finder: %s", sel.FolderPath)
				}
			}
			return m, nil

		case "f", "enter":
			// Fix selected
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) && !m.isFixing {
				sel := m.filtered[m.cursor]
				m.isFixing = true
				m.fixingID = sel.MovieID
				m.fixingStatus = fmt.Sprintf("Starting fix for %s...", sel.MovieID)
				m.statusMessage = fmt.Sprintf("⏳ Downloading missing assets and generating metadata for %s...", sel.MovieID)
				return m, m.startFixItemCmd(sel)
			}
			return m, nil

		case "F":
			// Batch Fix all incomplete movies
			if !m.isFixing && !m.batchFixActive {
				incomplete := make([]*MovieAudit, 0)
				for _, it := range m.items {
					if it.Status == StatusIncomplete {
						incomplete = append(incomplete, it)
					}
				}
				if len(incomplete) == 0 {
					m.statusMessage = "✨ All movies in library are already 100% complete!"
					return m, nil
				}
				m.batchFixActive = true
				m.batchFixTotal = len(incomplete)
				m.batchFixDone = 0
				m.batchFixQueue = incomplete[1:]
				m.isFixing = true
				m.fixingID = incomplete[0].MovieID
				m.fixingStatus = fmt.Sprintf("Starting batch fix for %s...", incomplete[0].MovieID)
				m.statusMessage = fmt.Sprintf("⏳ Starting Batch Fix (%d movies): repairing %s...", m.batchFixTotal, incomplete[0].MovieID)
				return m, m.startFixItemCmd(incomplete[0])
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *TUIModel) listHeight() int {
	h := m.height - 10
	if h < 5 {
		return 5
	}
	return h
}

func (m *TUIModel) applyFilter() {
	var res []*MovieAudit
	q := strings.ToLower(strings.TrimSpace(m.searchQuery))

	for _, it := range m.items {
		// Filter mode
		if m.filterMode == "incomplete" && it.Status != StatusIncomplete {
			continue
		}
		if m.filterMode == "no_video" && it.Status != StatusNoVideo {
			continue
		}

		// Search query
		if q != "" {
			matchID := strings.Contains(strings.ToLower(it.MovieID), q)
			matchTitle := strings.Contains(strings.ToLower(it.FolderTitle), q)
			matchAct := strings.Contains(strings.ToLower(it.ActressName), q)
			if !matchID && !matchTitle && !matchAct {
				continue
			}
		}
		res = append(res, it)
	}

	m.filtered = res
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
		m.scrollOffset = 0
	}
}

// View renders the TUI screen.
func (m *TUIModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var sb strings.Builder

	// Header
	header := m.renderHeader()
	sb.WriteString(header)
	sb.WriteString("\n")

	// If currently scanning, show dedicated Live Scanning Dashboard
	if m.isScanning {
		sb.WriteString(m.renderScanningDashboard())
		return sb.String()
	}

	// Calculate counts
	completeCount := 0
	incompleteCount := 0
	noVideoCount := 0
	for _, it := range m.items {
		switch it.Status {
		case StatusComplete:
			completeCount++
		case StatusIncomplete:
			incompleteCount++
		case StatusNoVideo:
			noVideoCount++
		}
	}

	// Status stats bar
	statsBar := fmt.Sprintf(
		"  %s %s  %s  %s",
		lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(fmt.Sprintf("📁 Target: %s", m.targetDir)),
		badgeComplete.Render(fmt.Sprintf("✓ Complete: %d", completeCount)),
		badgeIncomplete.Render(fmt.Sprintf("⚠ Incomplete: %d", incompleteCount)),
		badgeNoVideo.Render(fmt.Sprintf("✗ No Video: %d", noVideoCount)),
	)
	sb.WriteString(statsBar)
	sb.WriteString("\n\n")

	// Main Split Panes
	leftWidth := (m.width * 42) / 100
	if leftWidth < 38 {
		leftWidth = 38
	}
	rightWidth := m.width - leftWidth - 4
	if rightWidth < 40 {
		rightWidth = 40
	}

	paneHeight := m.height - 9
	if paneHeight < 8 {
		paneHeight = 8
	}

	leftContent := m.renderLeftPane(leftWidth, paneHeight)
	rightContent := m.renderRightPane(rightWidth, paneHeight)

	split := lipgloss.JoinHorizontal(lipgloss.Top, leftContent, " ", rightContent)
	sb.WriteString(split)
	sb.WriteString("\n")

	// Footer status and keybindings
	footer := m.renderFooter()
	sb.WriteString(footer)

	return sb.String()
}

func (m *TUIModel) renderScanningDashboard() string {
	var sb strings.Builder
	p := m.scanProgress

	cardWidth := m.width - 6
	if cardWidth > 90 {
		cardWidth = 90
	}
	if cardWidth < 40 {
		cardWidth = 40
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("  %s %s\n\n",
		m.spinner.View(),
		lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render("Scanning & Verifying Library Completeness..."),
	))

	content.WriteString(fmt.Sprintf("📁 Directory: %s\n", lipgloss.NewStyle().Foreground(colorText).Render(m.targetDir)))
	content.WriteString(strings.Repeat("─", cardWidth-4))
	content.WriteString("\n\n")

	// 1. Actress Progress Line
	if p.TotalActresses > 0 {
		actressPct := float64(p.CurrentActressIdx) / float64(p.TotalActresses) * 100
		content.WriteString(fmt.Sprintf("👩 %s: %s %s\n",
			lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("Actress"),
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(fmt.Sprintf("%s", p.CurrentActress)),
			lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("(%d of %d • %.0f%% • %d remaining)", p.CurrentActressIdx, p.TotalActresses, actressPct, p.ActressesRemaining)),
		))

		if p.ActressTotalMovies > 0 {
			content.WriteString(fmt.Sprintf("   └── 📂 Filmography: %s\n\n",
				lipgloss.NewStyle().Foreground(colorText).Render(fmt.Sprintf("Movie %d of %d (%d remaining)", p.ActressCurrentMovieIdx, p.ActressTotalMovies, p.ActressMoviesRemaining)),
			))
		} else {
			content.WriteString("\n")
		}
	}

	// 2. Movie Progress Line & Progress Bar
	if p.TotalMovies > 0 {
		moviePct := float64(p.CurrentMovieIdx) / float64(p.TotalMovies)
		if moviePct > 1.0 {
			moviePct = 1.0
		}
		bar := renderProgressBar(moviePct, cardWidth-8)

		content.WriteString(fmt.Sprintf("🎬 %s: %s\n",
			lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render("Total Movies"),
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(fmt.Sprintf("%d / %d movies (%.1f%% • %d remaining)", p.CurrentMovieIdx, p.TotalMovies, moviePct*100, p.MoviesRemaining)),
		))
		content.WriteString(fmt.Sprintf("   %s\n\n", bar))

		if p.CurrentMovieID != "" {
			titleText := p.CurrentTitle
			if len(titleText) > 40 {
				titleText = titleText[:39] + "…"
			}
			content.WriteString(fmt.Sprintf("   👉 Current Title: %s %s\n\n",
				lipgloss.NewStyle().Bold(true).Foreground(colorWarning).Render(p.CurrentMovieID),
				lipgloss.NewStyle().Foreground(colorText).Render(titleText),
			))
		}
	}

	content.WriteString(strings.Repeat("─", cardWidth-4))
	content.WriteString("\n")

	// 3. Live Statistics Summary
	statsLine := fmt.Sprintf("📊 Live Stats: %s  %s  %s",
		badgeComplete.Render(fmt.Sprintf("✓ Complete: %d", p.CompleteCount)),
		badgeIncomplete.Render(fmt.Sprintf("⚠ Incomplete: %d", p.IncompleteCount)),
		badgeNoVideo.Render(fmt.Sprintf("✗ No Video: %d", p.NoVideoCount)),
	)
	content.WriteString(statsLine)
	content.WriteString("\n\n")

	// 4. Speed & ETA
	if p.Speed > 0 {
		etaStr := p.ETA.Round(time.Second).String()
		if p.ETA <= 0 {
			etaStr = "almost done"
		}
		content.WriteString(fmt.Sprintf("⚡ Speed: %.1f movies/sec  •  ⏱️ Elapsed: %s  •  ⏳ ETA: %s\n",
			p.Speed,
			p.Elapsed.Round(time.Second),
			etaStr,
		))
	} else if p.Message != "" {
		content.WriteString(fmt.Sprintf("ℹ️ Status: %s\n", p.Message))
	}

	dashboard := cardStyle.Width(cardWidth).Render(content.String())
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().MarginLeft(2).Render(dashboard))
	sb.WriteString("\n\n  Press [q] or [Ctrl+C] to abort\n")

	return sb.String()
}

func (m *TUIModel) renderHeader() string {
	title := headerTitleStyle.Render("🔍 R19DEV AUDITOR & DOCTOR")
	subtitle := lipgloss.NewStyle().Foreground(colorMuted).Render("— Movie Completeness Checklist & Repair Tool")
	return lipgloss.JoinHorizontal(lipgloss.Center, title, subtitle)
}

func (m *TUIModel) renderLeftPane(width, height int) string {
	var sb strings.Builder

	// Filter Tabs & Search Header
	tabAll := "[ All ]"
	tabInc := "[ Incomplete ]"
	tabNoVid := "[ No Video ]"

	switch m.filterMode {
	case "all":
		tabAll = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorPrimary).Render(" All ")
	case "incomplete":
		tabInc = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(colorWarning).Render(" Incomplete ")
	case "no_video":
		tabNoVid = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorError).Render(" No Video ")
	}

	filterTabs := fmt.Sprintf("%s %s %s", tabAll, tabInc, tabNoVid)
	sb.WriteString(filterTabs)
	sb.WriteString("\n")

	if m.searching {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(fmt.Sprintf("🔍 Search: %s█\n", m.searchQuery)))
	} else if m.searchQuery != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("Filter: '%s' (press 'c' to clear)\n", m.searchQuery)))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("Press '/' to search, [Tab] to switch filter\n"))
	}
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteString("\n")

	if len(m.filtered) == 0 {
		sb.WriteString("\n  (No matching movies found)")
		return leftPanelStyle.Width(width).Height(height).Render(sb.String())
	}

	maxVisible := height - 4
	visibleItems := m.filtered[m.scrollOffset:]
	if len(visibleItems) > maxVisible {
		visibleItems = visibleItems[:maxVisible]
	}

	for i, it := range visibleItems {
		idx := m.scrollOffset + i
		isSelected := idx == m.cursor

		// Badge
		badgeStr := ""
		switch it.Status {
		case StatusComplete:
			badgeStr = lipgloss.NewStyle().Foreground(colorSuccess).Render("✓")
		case StatusIncomplete:
			badgeStr = lipgloss.NewStyle().Foreground(colorWarning).Render("⚠")
		case StatusNoVideo:
			badgeStr = lipgloss.NewStyle().Foreground(colorError).Render("✗")
		}

		// Text layout
		actressPart := it.ActressName
		if actressPart == "" {
			actressPart = "-"
		}
		if len(actressPart) > 12 {
			actressPart = actressPart[:11] + "…"
		}

		idPart := it.MovieID
		if idPart == "" {
			idPart = it.FolderTitle
		}
		if len(idPart) > 14 {
			idPart = idPart[:13] + "…"
		}

		line := fmt.Sprintf(" %s %-12s │ %-14s", badgeStr, idPart, actressPart)
		if len(line) > width-4 {
			line = line[:width-4]
		}

		if isSelected {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorSelected).
				Width(width - 4).
				Render(line)
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return leftPanelStyle.Width(width).Height(height).Render(sb.String())
}

func (m *TUIModel) renderRightPane(width, height int) string {
	var sb strings.Builder

	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		sb.WriteString("\n  No movie selected")
		return rightPanelStyle.Width(width).Height(height).Render(sb.String())
	}

	sel := m.filtered[m.cursor]

	// Title Card
	badge := badgeComplete.Render("✓ COMPLETE")
	if sel.Status == StatusIncomplete {
		badge = badgeIncomplete.Render("⚠ INCOMPLETE")
	} else if sel.Status == StatusNoVideo {
		badge = badgeNoVideo.Render("✗ NO VIDEO")
	}

	headerLine := fmt.Sprintf("%s %s",
		lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render(sel.MovieID),
		badge,
	)
	sb.WriteString(headerLine)
	sb.WriteString("\n")

	if sel.FolderTitle != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorText).Render(fmt.Sprintf("Title: %s\n", sel.FolderTitle)))
	}
	if sel.ActressName != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("Actress: %s\n", sel.ActressName)))
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("Path: %s\n", sel.FolderPath)))
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteString("\n")

	// Checklist Table
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("📋 Standard Asset Checklist:\n"))

	// 1. Video
	if sel.HasVideo {
		vidSize := FormatBytes(sel.VideoTotalBytes)
		extra := ""
		if sel.IsMultipart {
			extra = fmt.Sprintf(" [Multi-part %d CDs]", sel.MultiPartCount)
		} else if len(sel.VideoFiles) > 1 {
			extra = fmt.Sprintf(" [%d files]", len(sel.VideoFiles))
		}
		sb.WriteString(fmt.Sprintf("  %s Video File: %s%s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			vidSize,
			extra,
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Video File: %s\n",
			lipgloss.NewStyle().Foreground(colorError).Render("✗"),
			lipgloss.NewStyle().Foreground(colorError).Render("Missing / No Video Found"),
		))
	}

	// 2. NFO
	if sel.HasNFO {
		sb.WriteString(fmt.Sprintf("  %s Jellyfin NFO: %s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			lipgloss.NewStyle().Foreground(colorText).Render("Present (.nfo)"),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Jellyfin NFO: %s\n",
			lipgloss.NewStyle().Foreground(colorWarning).Render("✗"),
			lipgloss.NewStyle().Foreground(colorWarning).Render("Missing NFO Metadata"),
		))
	}

	// 3. HTML Viewer
	if sel.HasHTML {
		sb.WriteString(fmt.Sprintf("  %s HTML Gallery: %s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			lipgloss.NewStyle().Foreground(colorText).Render("movie.html Ready"),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s HTML Gallery: %s\n",
			lipgloss.NewStyle().Foreground(colorWarning).Render("✗"),
			lipgloss.NewStyle().Foreground(colorWarning).Render("Missing movie.html"),
		))
	}

	// 4. Poster
	if sel.HasPoster {
		posterDetail := "poster.jpg Ready"
		if sel.PosterDimensions != "" && sel.PosterBytes > 0 {
			posterDetail = fmt.Sprintf("poster.jpg (%s, %s)", sel.PosterDimensions, FormatBytes(sel.PosterBytes))
		}
		sb.WriteString(fmt.Sprintf("  %s Poster (Cover): %s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			lipgloss.NewStyle().Foreground(colorText).Render(posterDetail),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Poster (Cover): %s\n",
			lipgloss.NewStyle().Foreground(colorWarning).Render("✗"),
			lipgloss.NewStyle().Foreground(colorWarning).Render("Missing poster.jpg"),
		))
	}

	// 5. Fanart
	if sel.HasFanart {
		fanartDetail := "fanart.jpg Ready"
		if sel.FanartDimensions != "" && sel.FanartBytes > 0 {
			fanartDetail = fmt.Sprintf("fanart.jpg (%s, %s)", sel.FanartDimensions, FormatBytes(sel.FanartBytes))
		}
		sb.WriteString(fmt.Sprintf("  %s Fanart (Backdrop): %s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			lipgloss.NewStyle().Foreground(colorText).Render(fanartDetail),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Fanart (Backdrop): %s\n",
			lipgloss.NewStyle().Foreground(colorWarning).Render("✗"),
			lipgloss.NewStyle().Foreground(colorWarning).Render("Missing fanart.jpg"),
		))
	}

	// 6. Sample Screenshots
	if sel.ScreenshotsCount > 0 {
		sb.WriteString(fmt.Sprintf("  %s Screenshots: %s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			lipgloss.NewStyle().Foreground(colorText).Render(fmt.Sprintf("%d sample screenshots in extrafanart/", sel.ScreenshotsCount)),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s Screenshots: %s\n",
			lipgloss.NewStyle().Foreground(colorWarning).Render("✗"),
			lipgloss.NewStyle().Foreground(colorWarning).Render("Missing sample screenshots (extrafanart/ empty)"),
		))
	}

	// 7. Database Sync
	if sel.InDB {
		sb.WriteString(fmt.Sprintf("  %s SQLite Database: %s\n",
			lipgloss.NewStyle().Foreground(colorSuccess).Render("✓"),
			lipgloss.NewStyle().Foreground(colorText).Render("Indexed in Library & Organized DB"),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s SQLite Database: %s\n",
			lipgloss.NewStyle().Foreground(colorMuted).Render("○"),
			lipgloss.NewStyle().Foreground(colorMuted).Render("Not yet indexed in local DB"),
		))
	}

	// Health & Quality Warnings
	if len(sel.Warnings) > 0 {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorWarning).Render("⚠️ Health & Quality Notices:\n"))
		for _, w := range sel.Warnings {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				lipgloss.NewStyle().Foreground(colorWarning).Render("•"),
				lipgloss.NewStyle().Foreground(colorWarning).Render(w),
			))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteString("\n")

	// Live Fix Status Banner with Detailed Step Progress
	if m.isFixing {
		var fixBox strings.Builder
		if m.batchFixActive {
			fixBox.WriteString(fmt.Sprintf("⚡ %s (%d/%d movies)\n",
				lipgloss.NewStyle().Bold(true).Foreground(colorWarning).Render("BATCH REPAIR IN PROGRESS"),
				m.batchFixDone+1, m.batchFixTotal,
			))
			batchPct := float64(m.batchFixDone) / float64(m.batchFixTotal)
			fixBox.WriteString(fmt.Sprintf("   %s\n\n", renderProgressBar(batchPct, width-8)))
		}

		if m.fixProgress.StepIndex > 0 {
			fixBox.WriteString(fmt.Sprintf("⚙️ %s %s [%d/%d]:\n",
				m.spinner.View(),
				lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render(fmt.Sprintf("Fixing %s", m.fixingID)),
				m.fixProgress.StepIndex, m.fixProgress.TotalSteps,
			))
			fixBox.WriteString(fmt.Sprintf("   👉 %s\n", lipgloss.NewStyle().Foreground(colorText).Render(m.fixProgress.Message)))
			if m.fixProgress.TotalFiles > 0 && m.fixProgress.CurrentFile > 0 {
				filePct := float64(m.fixProgress.CurrentFile) / float64(m.fixProgress.TotalFiles)
				fixBox.WriteString(fmt.Sprintf("   %s (file %d/%d)\n",
					renderProgressBar(filePct, 24),
					m.fixProgress.CurrentFile, m.fixProgress.TotalFiles,
				))
			}
		} else {
			fixBox.WriteString(fmt.Sprintf("⏳ %s %s...\n", m.spinner.View(), m.fixingStatus))
		}

		sb.WriteString(fixBoxStyle.Width(width - 4).Render(fixBox.String()))
		sb.WriteString("\n")
	} else {
		// Missing Summary / Action Guidance
		if len(sel.MissingItems) > 0 {
			missingStr := strings.Join(sel.MissingItems, ", ")
			sb.WriteString(lipgloss.NewStyle().Foreground(colorWarning).Bold(true).Render(fmt.Sprintf("⚠️ Missing Assets: %s\n", missingStr)))
			sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Render("👉 Press [f] or [Enter] to download & generate missing assets automatically\n"))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(colorSuccess).Bold(true).Render("🎉 All standard assets and metadata for this title are 100% complete!\n"))
		}
	}

	return rightPanelStyle.Width(width).Height(height).Render(sb.String())
}

func (m *TUIModel) renderFooter() string {
	var sb strings.Builder

	// Status message
	if m.statusMessage != "" {
		sb.WriteString("  " + m.statusMessage + "\n")
	}

	// Keybindings
	keys := []struct{ key, desc string }{
		{"↑/↓", "Select"},
		{"Tab", "Filter (" + m.filterMode + ")"},
		{"/", "Search"},
		{"f / Enter", "Fix Selected"},
		{"F", "Batch Fix All"},
		{"o", "Open Finder"},
		{"r", "Rescan"},
		{"q", "Quit"},
	}

	var keyParts []string
	for _, k := range keys {
		keyParts = append(keyParts, fmt.Sprintf("%s %s", footerKeyStyle.Render(k.key), footerDescStyle.Render(k.desc)))
	}

	sb.WriteString("  " + strings.Join(keyParts, " "))
	return sb.String()
}

func renderProgressBar(pct float64, width int) string {
	if width < 10 {
		width = 10
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	fillStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", empty)
	return lipgloss.NewStyle().Foreground(colorPrimary).Render(fillStr) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#313244")).Render(emptyStr)
}

// RunAuditTUI launches the Bubbletea audit program.
func RunAuditTUI(targetDir string, d *db.DB) error {
	model, err := NewTUI(targetDir, d)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
