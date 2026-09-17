package migrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	colorPurple = lipgloss.Color("#7D56F4")
	colorGreen  = lipgloss.Color("#04B575")
	colorCyan   = lipgloss.Color("#00D2FF")
	colorAmber  = lipgloss.Color("#FFB800")
	colorRed    = lipgloss.Color("#FF4C4C")
	colorGray   = lipgloss.Color("#6B7280")
	colorBg     = lipgloss.Color("#181825")
	colorText   = lipgloss.Color("#CDD6F4")

	tuiTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPurple).
			Padding(0, 2)

	tuiBadge = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	tuiBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(0, 1)

	tuiLogTime = lipgloss.NewStyle().Foreground(colorGray)
	tuiLogText = lipgloss.NewStyle().Foreground(colorText)
)

type logEntry struct {
	time    string
	icon    string
	color   lipgloss.Color
	message string
}

type tuiModel struct {
	ctx               context.Context
	cancel            context.CancelFunc
	cfg               Config
	eventCh           chan ProgressEvent
	confirmCh         chan bool
	isAwaitingConfirm bool
	width             int
	height            int
	phase             string
	spinnerIdx        int
	current           int
	total             int
	currentID         string
	currentAct        string
	currentTitle      string
	currentMsg        string
	logs              []logEntry
	organized         int
	duplicates        int
	skipped           int
	errors            int
	healed            int
	startTime         time.Time
	summary           *Summary
	done              bool
	err               error
}

type eventMsg ProgressEvent
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForEvent(ch <-chan ProgressEvent) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(e)
	}
}

// RunTUI launches the interactive migration TUI.
func RunTUI(ctx context.Context, cfg Config) (*Summary, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventCh := make(chan ProgressEvent, 100)
	confirmCh := make(chan bool, 1)

	m := tuiModel{
		ctx:       ctx,
		cancel:    cancel,
		cfg:       cfg,
		eventCh:   eventCh,
		confirmCh: confirmCh,
		phase:     "SCANNING",
		startTime: time.Now(),
		width:     80,
		height:    24,
	}

	// Launch background migration engine
	var bgSummary *Summary
	var bgErr error
	go func() {
		bgSummary, bgErr = Run(ctx, cfg, eventCh, confirmCh)
		close(eventCh)
	}()

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if fm, ok := finalModel.(tuiModel); ok {
		if fm.err != nil {
			return nil, fm.err
		}
		if fm.summary != nil {
			return fm.summary, nil
		}
	}

	return bgSummary, bgErr
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.eventCh), tick())
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
		if !m.done {
			return m, tick()
		}
		return m, nil

	case tea.KeyMsg:
		if m.isAwaitingConfirm {
			switch strings.ToLower(msg.String()) {
			case "y", "enter":
				m.isAwaitingConfirm = false
				m.phase = "ORGANIZING"
				m.addLog(time.Now().Format("15:04:05"), "🚀", colorGreen, "Confirmation received! Proceeding with migration...")
				if m.confirmCh != nil {
					m.confirmCh <- true
				}
				return m, nil
			case "n", "q", "esc", "ctrl+c":
				m.isAwaitingConfirm = false
				m.done = true
				m.phase = "CANCELLED"
				m.addLog(time.Now().Format("15:04:05"), "🛑", colorAmber, "Migration cancelled by user. No files modified.")
				if m.confirmCh != nil {
					m.confirmCh <- false
				}
				return m, tea.Quit
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.done {
				return m, tea.Quit
			}
			m.cancel()
			m.done = true
			m.err = fmt.Errorf("migration cancelled by user")
			return m, tea.Quit
		case "enter", "esc":
			if m.done {
				return m, tea.Quit
			}
		}

	case eventMsg:
		e := ProgressEvent(msg)
		nowStr := time.Now().Format("15:04:05")

		switch e.Type {
		case EventScanStart:
			m.phase = "SCANNING"
			m.currentMsg = e.Message
			m.addLog(nowStr, "🔍", colorCyan, e.Message)

		case EventScanProgress:
			m.phase = "SCANNING"
			m.current = e.Current
			m.currentMsg = e.Message
			m.currentTitle = e.Detail
			if e.Current <= 10 || e.Current%25 == 0 {
				m.addLog(nowStr, "📂", colorCyan, fmt.Sprintf("Discovered %d video files so far...", e.Current))
			}

		case EventScanDone:
			m.phase = "MAPPING"
			m.total = e.Total
			m.currentMsg = e.Message
			m.addLog(nowStr, "✨", colorGreen, e.Message)

		case EventPlanItem:
			if m.cfg.DryRun {
				m.phase = "DRY-RUN PREVIEW"
			} else {
				m.phase = "PRE-FLIGHT MAPPING"
			}
			m.current = e.Current
			m.total = e.Total
			m.currentID = e.MovieID
			m.currentAct = e.Actress
			m.currentTitle = e.Title
			m.currentMsg = e.Message
			if m.cfg.DryRun {
				m.organized++
			}
			m.addLog(nowStr, "👁️", colorPurple, fmt.Sprintf("%s -> %s", e.MovieID, e.Actress))

		case EventConfirmReady:
			m.phase = "CONFIRMATION"
			m.isAwaitingConfirm = true
			m.current = e.Current
			m.total = e.Total
			m.currentMsg = e.Message
			m.addLog(nowStr, "📋", colorAmber, fmt.Sprintf("Pre-flight plan verified: %d movies ready. Awaiting confirmation...", e.Current))

		case EventMoveStart:
			m.phase = "ORGANIZING"
			m.current = e.Current
			m.total = e.Total
			m.currentID = e.MovieID
			m.currentAct = e.Actress
			m.currentTitle = e.Title
			m.currentMsg = e.Message

		case EventMoveDone:
			m.phase = "ORGANIZING"
			m.current = e.Current
			m.total = e.Total
			m.organized++
			m.addLog(nowStr, "✔", colorGreen, fmt.Sprintf("%s -> %s", e.MovieID, e.Actress))

		case EventMoveDuplicate:
			m.duplicates++
			m.addLog(nowStr, "ℹ", colorCyan, fmt.Sprintf("%s: Destination already exists, merging assets", e.MovieID))

		case EventMoveSkip:
			m.skipped++
			m.addLog(nowStr, "⚠", colorAmber, e.Message)

		case EventMoveError:
			m.errors++
			m.addLog(nowStr, "✖", colorRed, fmt.Sprintf("%s: %s", e.MovieID, e.Message))

		case EventUpdateHTML:
			m.phase = "UPDATING HTML"
			m.currentMsg = e.Message
			if e.Total > 0 {
				m.current = e.Current
				m.total = e.Total
			}
			if e.MovieID != "" {
				m.currentID = e.MovieID
				m.currentAct = e.Actress
				m.currentTitle = e.Title
			}
			if e.Total == 0 || e.Current == 1 || e.Current%20 == 0 || e.Current == e.Total {
				m.addLog(nowStr, "🔄", colorCyan, e.Message)
			}

		case EventAuditStart:
			m.phase = "QUALITY AUDIT & HEAL"
			m.currentMsg = e.Message
			m.addLog(nowStr, "🩺", colorPurple, e.Message)

		case EventAuditProgress:
			m.phase = "QUALITY AUDIT & HEAL"
			m.current = e.Current
			m.total = e.Total
			m.currentID = e.MovieID
			m.currentAct = e.Actress
			m.currentMsg = e.Message
			if e.Current <= 5 || e.Current%20 == 0 || e.Current == e.Total || strings.Contains(e.Message, "Auto-healing") {
				m.addLog(nowStr, "🔍", colorCyan, e.Message)
			}

		case EventAuditHealed:
			m.healed++
			m.addLog(nowStr, "✨", colorGreen, e.Message)

		case EventAuditDone:
			m.phase = "AUDIT COMPLETE"
			m.currentMsg = e.Message
			m.addLog(nowStr, "🏁", colorGreen, e.Message)

		case EventCleanArchive:
			m.phase = "CLEANING UP"
			m.currentMsg = e.Message
			m.addLog(nowStr, "🧹", colorGray, e.Message)

		case EventDone:
			m.phase = "COMPLETED"
			m.done = true
			m.summary = e.SummaryData
			m.addLog(nowStr, "✨", colorGreen, "All operations finished successfully!")
			return m, nil
		}

		return m, waitForEvent(m.eventCh)
	}

	return m, nil
}

func (m *tuiModel) addLog(timeStr, icon string, col lipgloss.Color, msg string) {
	entry := logEntry{
		time:    timeStr,
		icon:    icon,
		color:   col,
		message: msg,
	}
	m.logs = append(m.logs, entry)
	if len(m.logs) > 1000 {
		m.logs = m.logs[len(m.logs)-1000:]
	}
}

func (m tuiModel) View() string {
	var b strings.Builder

	spin := spinnerFrames[m.spinnerIdx]

	modeTag := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorGreen).Padding(0, 1).Render("LIVE MIGRATION")
	if m.cfg.DryRun {
		modeTag = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorAmber).Padding(0, 1).Render("DRY-RUN PREVIEW")
	}

	phaseTag := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(fmt.Sprintf("[%s %s]", spin, m.phase))
	if m.isAwaitingConfirm {
		phaseTag = lipgloss.NewStyle().Foreground(colorAmber).Bold(true).Render("[? AWAITING CONFIRMATION]")
	} else if m.done {
		phaseTag = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("[✔ COMPLETED]")
	}

	// 1. Header
	header := fmt.Sprintf("%s %s %s",
		tuiTitle.Render("R19DEV CINEMATIC MIGRATOR"),
		modeTag,
		phaseTag,
	)
	b.WriteString(header + "\n\n")

	// 2. Paths
	b.WriteString(fmt.Sprintf("  %s %s\n", lipgloss.NewStyle().Foreground(colorGray).Render("Source:"), m.cfg.SourceDir))
	b.WriteString(fmt.Sprintf("  %s %s\n\n", lipgloss.NewStyle().Foreground(colorGray).Render("Target:"), m.cfg.DestRoot))

	// 3. Progress Bar
	pct := 0.0
	if m.total > 0 {
		pct = float64(m.current) / float64(m.total)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	barWidth := m.width - 26
	if barWidth < 20 {
		barWidth = 35
	}
	if barWidth > 60 {
		barWidth = 60
	}

	filled := int(float64(barWidth) * pct)
	empty := barWidth - filled
	if empty < 0 {
		empty = 0
	}

	bar := lipgloss.NewStyle().Foreground(colorPurple).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#313244")).Render(strings.Repeat("░", empty))

	elapsed := time.Since(m.startTime).Round(time.Second)
	speed := 0.0
	if elapsed.Seconds() > 0 && m.current > 0 {
		speed = float64(m.current) / elapsed.Seconds()
	}

	if m.phase == "SCANNING" {
		b.WriteString(fmt.Sprintf("  [%s] Scanning...  Discovered: %d files   ⏱ %s\n\n",
			lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(spin),
			m.current,
			elapsed,
		))
	} else if m.isAwaitingConfirm {
		b.WriteString(fmt.Sprintf("  [%s] Pre-Flight Complete: %d movies verified   ⏱ %s\n\n",
			lipgloss.NewStyle().Foreground(colorAmber).Bold(true).Render("✔"),
			m.current,
			elapsed,
		))
	} else {
		b.WriteString(fmt.Sprintf("  [%s] %5.1f%%  [%d / %d]  ⏱ %s (%.1f/s)\n\n",
			bar, pct*100, m.current, m.total, elapsed, speed))
	}

	// 4. Metrics Counters
	statsList := []string{
		tuiBadge.Foreground(lipgloss.Color("#FFFFFF")).Background(colorGreen).Render(fmt.Sprintf("✔ Organized: %d", m.organized)),
		tuiBadge.Foreground(lipgloss.Color("#FFFFFF")).Background(colorCyan).Render(fmt.Sprintf("ℹ Duplicates: %d", m.duplicates)),
		tuiBadge.Foreground(lipgloss.Color("#FFFFFF")).Background(colorAmber).Render(fmt.Sprintf("⚠ Skipped: %d", m.skipped)),
		tuiBadge.Foreground(lipgloss.Color("#FFFFFF")).Background(colorRed).Render(fmt.Sprintf("✖ Errors: %d", m.errors)),
	}
	if m.cfg.AuditAfter || m.healed > 0 {
		statsList = append(statsList, tuiBadge.Foreground(lipgloss.Color("#FFFFFF")).Background(colorPurple).Render(fmt.Sprintf("✨ Healed: %d", m.healed)))
	}
	stats := "  " + strings.Join(statsList, " ")
	b.WriteString(stats + "\n\n")

	// 5. Active Movie Box or Confirmation Card
	activeContent := ""
	if m.isAwaitingConfirm {
		promptBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorAmber).Padding(0, 1).Render("CONFIRM MIGRATION")
		yesKey := lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Render("[Enter] / [y] PROCEED")
		noKey := lipgloss.NewStyle().Bold(true).Foreground(colorRed).Render("[q] / [Esc] CANCEL")
		activeContent = fmt.Sprintf("%s  Pre-flight check passed: %d movies ready to organize into %s\n\n  👉 Press %s to start moving files & generating metadata\n  👉 Press %s to abort safely without modifying any files",
			promptBadge, m.current, m.cfg.DestRoot, yesKey, noKey)
	} else if m.phase == "SCANNING" {
		activeContent = fmt.Sprintf("%s Scanning directory tree...\n%s",
			lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("🔍 Scanning:"),
			lipgloss.NewStyle().Foreground(colorText).Render(fmt.Sprintf("Latest item: %s", m.currentTitle)),
		)
	} else if m.currentID != "" {
		idBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorPurple).Padding(0, 1).Render(m.currentID)
		actBadge := lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render(m.currentAct)
		titleStr := m.currentTitle
		maxTitleLen := m.width - 25
		if maxTitleLen > 20 && len(titleStr) > maxTitleLen {
			titleStr = titleStr[:maxTitleLen-3] + "..."
		}
		activeContent = fmt.Sprintf("%s %s • %s\n%s",
			idBadge, actBadge, titleStr,
			lipgloss.NewStyle().Foreground(colorGray).Render(m.currentMsg))
	} else {
		activeContent = lipgloss.NewStyle().Foreground(colorGray).Render(m.currentMsg)
	}

	b.WriteString(tuiBox.Width(m.width-4).Render(activeContent) + "\n\n")

	// 6. Event Log (Dynamically scales to fill available terminal height)
	availableLogLines := m.height - 16
	if availableLogLines < 6 {
		availableLogLines = 6
	}

	displayLogs := m.logs
	if len(displayLogs) > availableLogLines {
		displayLogs = displayLogs[len(displayLogs)-availableLogLines:]
	}

	logHeader := fmt.Sprintf("  Recent Activity (%d events):", len(m.logs))
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(logHeader) + "\n")
	if len(displayLogs) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render("    (Waiting for events...)") + "\n")
	} else {
		maxMsgLen := m.width - 22
		for _, l := range displayLogs {
			iconColored := lipgloss.NewStyle().Foreground(l.color).Bold(true).Render(l.icon)
			msgText := l.message
			if maxMsgLen > 15 && len(msgText) > maxMsgLen {
				msgText = msgText[:maxMsgLen-3] + "..."
			}
			b.WriteString(fmt.Sprintf("    %s %s %s\n",
				tuiLogTime.Render(l.time),
				iconColored,
				tuiLogText.Render(msgText),
			))
		}
	}

	// 7. Footer
	b.WriteString("\n")
	if m.isAwaitingConfirm {
		b.WriteString(lipgloss.NewStyle().Foreground(colorAmber).Bold(true).Render("  👉 [Enter / y] Confirm & Proceed   •   [q / Esc] Cancel safely") + "\n")
	} else if m.done {
		b.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("  ✔ Migration completed! Press [Enter] or [q] to exit.") + "\n")
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(colorGray).Render("  Press [q] or [Ctrl+C] to abort migration safely.") + "\n")
	}

	return b.String()
}
