package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

type scrapeDoneMsg struct {
	id    string
	movie *scraper.Movie
	err   error
}

type debounceScrapeMsg struct {
	seq int
	id  string
}

type batchItemMsg struct {
	id    string
	movie *scraper.Movie
	err   error
}

type batchCompleteMsg struct {
	total int
}

// Model represents the main TUI application state.
type Model struct {
	targetDir     string
	scanner       *scanner.Scanner
	matcher       *matcher.Matcher
	scraperClient *scraper.Client

	files         []scanner.FileInfo
	matches       []matcher.MatchResult
	metadataCache map[string]*scraper.Movie
	scrapeErrors  map[string]string

	matchedCount   int
	unmatchedCount int

	cursor       int
	cursorSeq    int
	scrollOffset int
	width        int
	height       int

	isScanning      bool
	isScraping      bool
	isBatchScraping bool
	batchTotal      int
	batchDone       int
	batchChan       chan batchItemMsg

	statusMessage string
	keys          KeyMap
	spinner       spinner.Model
	editModal     EditModal
}

// New creates a new TUI model for the given target directory.
func New(targetDir string) (*Model, error) {
	sc := scanner.New(scanner.DefaultConfig())
	mc, err := matcher.New(matcher.DefaultConfig())
	if err != nil {
		return nil, err
	}
	scClient := scraper.NewClient(15 * time.Second)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	return &Model{
		targetDir:     targetDir,
		scanner:       sc,
		matcher:       mc,
		scraperClient: scClient,
		metadataCache: make(map[string]*scraper.Movie),
		scrapeErrors:  make(map[string]string),
		keys:          DefaultKeyMap(),
		spinner:       s,
		editModal:     NewEditModal(),
		isScanning:    true,
	}, nil
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

func debounceCmd(seq int, id string) tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return debounceScrapeMsg{seq: seq, id: id}
	})
}

func listenBatchProgress(ch chan batchItemMsg) tea.Cmd {
	return func() tea.Msg {
		item, ok := <-ch
		if !ok {
			return batchCompleteMsg{}
		}
		return item
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
		if m.isScanning || m.isScraping || m.isBatchScraping {
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
		m.statusMessage = fmt.Sprintf("✅ Scan complete: %d files (%d matched)", len(m.files), m.matchedCount)

		// Auto-fetch metadata for the first item if available
		if len(m.matches) > 0 && m.matches[0].ID != "" {
			m.cursorSeq++
			cmds = append(cmds, debounceCmd(m.cursorSeq, m.matches[0].ID))
		}

	case debounceScrapeMsg:
		if msg.seq == m.cursorSeq && msg.id != "" {
			if _, cached := m.metadataCache[msg.id]; !cached {
				m.isScraping = true
				cmds = append(cmds, m.scrapeMovieCmd(msg.id))
			}
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
		}

	case batchItemMsg:
		m.batchDone++
		if msg.err != nil {
			m.scrapeErrors[msg.id] = msg.err.Error()
		} else {
			m.metadataCache[msg.id] = msg.movie
			delete(m.scrapeErrors, msg.id)
		}
		m.statusMessage = fmt.Sprintf("⚡ Batch scraping: %d/%d completed...", m.batchDone, m.batchTotal)
		// Continue listening for next batch progress item
		if m.batchChan != nil {
			cmds = append(cmds, listenBatchProgress(m.batchChan))
		}

	case batchCompleteMsg:
		m.isBatchScraping = false
		m.batchChan = nil
		m.statusMessage = fmt.Sprintf("🎉 Batch scrape completed (%d/%d items processed)!", m.batchDone, m.batchTotal)

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
					m.isScraping = true
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
				cmds = append(cmds, m.triggerCursorScrape())
			}

		case "down", "j":
			if m.cursor < len(m.matches)-1 {
				m.cursor++
				m.adjustScroll()
				cmds = append(cmds, m.triggerCursorScrape())
			}

		case "pgup", "b":
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.adjustScroll()
			cmds = append(cmds, m.triggerCursorScrape())

		case "pgdown", "f":
			m.cursor += 10
			if m.cursor >= len(m.matches) {
				m.cursor = len(m.matches) - 1
			}
			m.adjustScroll()
			cmds = append(cmds, m.triggerCursorScrape())

		case "g", "home":
			m.cursor = 0
			m.adjustScroll()
			cmds = append(cmds, m.triggerCursorScrape())

		case "G", "end":
			if len(m.matches) > 0 {
				m.cursor = len(m.matches) - 1
				m.adjustScroll()
				cmds = append(cmds, m.triggerCursorScrape())
			}

		case "e":
			if m.cursor >= 0 && m.cursor < len(m.matches) {
				cmds = append(cmds, m.editModal.Open(m.cursor, m.matches[m.cursor].ID))
			}

		case "enter", "space":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				m.isScraping = true
				m.statusMessage = fmt.Sprintf("🔍 Scraping %s from R18.dev...", curMatch.ID)
				cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
			}

		case "s":
			if m.isBatchScraping {
				m.statusMessage = "⚠️ Batch scrape is already in progress..."
				return m, nil
			}
			var queue []string
			for _, match := range m.matches {
				if match.ID != "" {
					if _, cached := m.metadataCache[match.ID]; !cached {
						queue = append(queue, match.ID)
					}
				}
			}
			if len(queue) == 0 {
				m.statusMessage = "✨ All matched items are already scraped!"
				return m, nil
			}

			m.isBatchScraping = true
			m.batchTotal = len(queue)
			m.batchDone = 0
			m.batchChan = make(chan batchItemMsg, len(queue))
			m.statusMessage = fmt.Sprintf("🚀 Starting batch scrape for %d items (3 workers)...", len(queue))

			// Launch background pool
			go m.runBatchScrapeWorkerPool(queue, m.batchChan)
			cmds = append(cmds, listenBatchProgress(m.batchChan))

		case "r":
			m.isScanning = true
			m.statusMessage = "🔄 Rescanning directory..."
			cmds = append(cmds, m.startScanCmd())
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) triggerCursorScrape() tea.Cmd {
	m.cursorSeq++
	curMatch := m.currentMatch()
	if curMatch == nil || curMatch.ID == "" {
		return nil
	}
	// If already in cache, no need to trigger debounce network request
	if _, cached := m.metadataCache[curMatch.ID]; cached {
		return nil
	}
	return debounceCmd(m.cursorSeq, curMatch.ID)
}

func (m Model) runBatchScrapeWorkerPool(queue []string, out chan<- batchItemMsg) {
	numWorkers := 3
	if len(queue) < numWorkers {
		numWorkers = len(queue)
	}

	in := make(chan string, len(queue))
	for _, id := range queue {
		in <- id
	}
	close(in)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range in {
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				movie, err := m.scraperClient.Scrape(ctx, id)
				cancel()
				out <- batchItemMsg{id: id, movie: movie, err: err}
				// Polite rate-limit delay
				time.Sleep(150 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	close(out)
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
