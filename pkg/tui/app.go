package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/scanner"
	"github.com/dagalp/r19dev-scraper/pkg/scraper"
)

type scanDoneMsg struct {
	result *scanner.ScanResult
	err    error
}

type matchDoneMsg struct {
	matches []matcher.MatchResult
}

type scrapeDoneMsg struct {
	id    string
	movie *scraper.Movie
	err   error
}

type coverDoneMsg struct {
	id   string
	ansi string
	err  error
}

// Model represents the main TUI application state.
type Model struct {
	targetDir      string
	language       string
	protocol       GraphicProtocol
	scanner        *scanner.Scanner
	matcher        *matcher.Matcher
	scraperClient  *scraper.Client

	files          []scanner.FileInfo
	matches        []matcher.MatchResult
	metadataCache  map[string]*scraper.Movie
	coverCache     map[string]string // ID -> formatted image string
	scrapeErrors   map[string]string

	matchedCount   int
	unmatchedCount int

	cursor         int
	scrollOffset   int
	width          int
	height         int

	isScanning     bool
	isScraping     bool
	isCoverLoading bool
	showCover      bool
	statusMessage  string
	keys           KeyMap
	spinner        spinner.Model
	editModal      EditModal
}

// New creates a new TUI model for the given target directory, language, and graphics protocol preference.
func New(targetDir, lang string, proto GraphicProtocol) (*Model, error) {
	if lang == "" {
		lang = "en"
	}
	if proto == "" {
		proto = ProtocolAuto
	}
	sc := scanner.New(scanner.DefaultConfig())
	mc, err := matcher.New(matcher.DefaultConfig())
	if err != nil {
		return nil, err
	}
	scClient := scraper.NewClient(15 * time.Second)
	scClient.SetLanguage(lang)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	return &Model{
		targetDir:     targetDir,
		language:      lang,
		protocol:      proto,
		scanner:       sc,
		matcher:       mc,
		scraperClient: scClient,
		metadataCache: make(map[string]*scraper.Movie),
		coverCache:    make(map[string]string),
		scrapeErrors:  make(map[string]string),
		showCover:     true,
		keys:          DefaultKeyMap(),
		spinner:       s,
		editModal:     NewEditModal(),
		isScanning:    true,
	}, nil
}

// ActiveProtocol returns the currently active terminal graphics protocol.
func (m Model) ActiveProtocol() GraphicProtocol {
	if m.protocol == ProtocolAuto {
		return DetectTerminalProtocol()
	}
	return m.protocol
}

// ProtocolDisplayString returns a user-friendly label of the current graphics mode.
func (m Model) ProtocolDisplayString() string {
	active := m.ActiveProtocol()
	if m.protocol == ProtocolAuto {
		return fmt.Sprintf("AUTO (%s)", strings.ToUpper(string(active)))
	}
	return strings.ToUpper(string(m.protocol))
}

// Init triggers initial directory scanning and spinner tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startScanCmd(),
	)
}

func (m Model) startScanCmd() tea.Cmd {
	return func() tea.Msg {
		res, err := m.scanner.Scan(m.targetDir)
		return scanDoneMsg{result: res, err: err}
	}
}

func (m Model) scrapeMovieCmd(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		movie, err := m.scraperClient.Scrape(ctx, id)
		return scrapeDoneMsg{id: id, movie: movie, err: err}
	}
}

func (m Model) fetchCoverCmd(id, coverURL string, targetW, targetH int) tea.Cmd {
	return func() tea.Msg {
		ansi, err := FetchAndRenderCover(coverURL, m.protocol, targetW, targetH)
		return coverDoneMsg{id: id, ansi: ansi, err: err}
	}
}

func (m *Model) recomputeStats() {
	matched := 0
	for _, match := range m.matches {
		if match.ID != "" {
			matched++
		}
	}
	m.matchedCount = matched
	m.unmatchedCount = len(m.matches) - matched
}

// Update handles incoming messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		if m.isScanning || m.isScraping || m.isCoverLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case scanDoneMsg:
		m.isScanning = false
		if msg.err != nil {
			m.statusMessage = "❌ Scan error: " + msg.err.Error()
			return m, nil
		}
		m.files = msg.result.Files
		m.matches = m.matcher.Match(m.files)
		m.recomputeStats()
		m.statusMessage = fmt.Sprintf("✅ Scan complete: %d files found (%d matched)", len(m.files), m.matchedCount)

		// Auto-fetch metadata for the first item if available
		if len(m.matches) > 0 && m.matches[0].ID != "" {
			cmds = append(cmds, m.scrapeMovieCmd(m.matches[0].ID))
		}

	case scrapeDoneMsg:
		m.isScraping = false
		if msg.err != nil {
			m.scrapeErrors[msg.id] = msg.err.Error()
			m.statusMessage = fmt.Sprintf("⚠️ Scrape failed for %s: %v", msg.id, msg.err)
		} else {
			m.metadataCache[msg.id] = msg.movie
			delete(m.scrapeErrors, msg.id)
			m.statusMessage = fmt.Sprintf("🎉 Loaded metadata for %s", msg.id)

			// Fetch cover preview if available and not cached yet
			targetURL := msg.movie.PosterURL
			if targetURL == "" {
				targetURL = msg.movie.CoverURL
			}
			if targetURL != "" && m.showCover {
				if _, ok := m.coverCache[msg.id]; !ok {
					m.isCoverLoading = true
					coverW := 22
					coverH := 8
					cmds = append(cmds, m.fetchCoverCmd(msg.id, targetURL, coverW, coverH))
				}
			}
		}

	case coverDoneMsg:
		m.isCoverLoading = false
		if msg.err == nil && msg.ansi != "" {
			m.coverCache[msg.id] = msg.ansi
		}

	case tea.KeyMsg:
		// Modal active
		if m.editModal.Active {
			switch msg.String() {
			case "enter":
				newID := strings.TrimSpace(m.editModal.Input.Value())
				if newID != "" && m.editModal.FileIndex < len(m.matches) {
					m.matches[m.editModal.FileIndex].ID = strings.ToUpper(newID)
					m.matches[m.editModal.FileIndex].MatchedBy = "manual"
					m.recomputeStats()
					m.statusMessage = fmt.Sprintf("✏️ Updated ID to %s", m.matches[m.editModal.FileIndex].ID)
					cmds = append(cmds, m.scrapeMovieCmd(m.matches[m.editModal.FileIndex].ID))
				}
				m.editModal.Close()
				return m, nil
			case "esc":
				m.editModal.Close()
				return m, nil
			default:
				var cmd tea.Cmd
				m.editModal.Input, cmd = m.editModal.Input.Update(msg)
				return m, cmd
			}
		}

		// Normal navigation
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
				if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
					if _, ok := m.metadataCache[curMatch.ID]; !ok {
						m.isScraping = true
						cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
					}
				}
			}

		case "down", "j":
			if m.cursor < len(m.matches)-1 {
				m.cursor++
				m.adjustScroll()
				if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
					if _, ok := m.metadataCache[curMatch.ID]; !ok {
						m.isScraping = true
						cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
					}
				}
			}

		case "pgup", "b":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.adjustScroll()

		case "pgdown", "f":
			m.cursor += 10
			if m.cursor >= len(m.matches) {
				m.cursor = len(m.matches) - 1
			}
			m.adjustScroll()

		case "g", "home":
			m.cursor = 0
			m.adjustScroll()

		case "G", "end":
			if len(m.matches) > 0 {
				m.cursor = len(m.matches) - 1
				m.adjustScroll()
			}

		case "e":
			if m.cursor >= 0 && m.cursor < len(m.matches) {
				cmds = append(cmds, m.editModal.Open(m.cursor, m.matches[m.cursor].ID))
			}

		case "c":
			m.showCover = !m.showCover
			if m.showCover {
				m.statusMessage = "🖼️ Cover view enabled"
				if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
					if mov, ok := m.metadataCache[curMatch.ID]; ok {
						targetURL := mov.PosterURL
						if targetURL == "" {
							targetURL = mov.CoverURL
						}
						if targetURL != "" {
							cmds = append(cmds, m.fetchCoverCmd(curMatch.ID, targetURL, 22, 8))
						}
					}
				}
			} else {
				m.statusMessage = "🖼️ Cover view hidden"
			}

		case "p":
			// Cycle graphics protocol: auto -> halfblock -> kitty -> iterm2 -> sixel -> auto
			switch m.protocol {
			case ProtocolAuto:
				m.protocol = ProtocolHalfBlock
			case ProtocolHalfBlock:
				m.protocol = ProtocolKitty
			case ProtocolKitty:
				m.protocol = ProtocolITerm2
			case ProtocolITerm2:
				m.protocol = ProtocolSixel
			default:
				m.protocol = ProtocolAuto
			}
			m.coverCache = make(map[string]string) // Invalidate cache on protocol switch
			m.statusMessage = fmt.Sprintf("🎨 Switched graphics mode to: %s", m.ProtocolDisplayString())
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				if mov, ok := m.metadataCache[curMatch.ID]; ok {
					targetURL := mov.PosterURL
					if targetURL == "" {
						targetURL = mov.CoverURL
					}
					if targetURL != "" {
						m.isCoverLoading = true
						cmds = append(cmds, m.fetchCoverCmd(curMatch.ID, targetURL, 22, 8))
					}
				}
			}

		case "enter", "space":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				m.isScraping = true
				m.statusMessage = fmt.Sprintf("🔍 Scraping %s from R18.dev...", curMatch.ID)
				cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
			}

		case "r":
			m.isScanning = true
			m.statusMessage = "🔄 Rescanning directory..."
			cmds = append(cmds, m.startScanCmd())
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) adjustScroll() {
	visibleRows := m.height - 10
	if visibleRows < 5 {
		visibleRows = 5
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.cursor - visibleRows + 1
	}
}

func (m Model) currentMatch() *matcher.MatchResult {
	if m.cursor >= 0 && m.cursor < len(m.matches) {
		return &m.matches[m.cursor]
	}
	return nil
}
