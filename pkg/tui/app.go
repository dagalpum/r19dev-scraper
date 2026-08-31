package tui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dagalp/r19dev-scraper/pkg/cache"
	"github.com/dagalp/r19dev-scraper/pkg/db"
	"github.com/dagalp/r19dev-scraper/pkg/matcher"
	"github.com/dagalp/r19dev-scraper/pkg/organizer"
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

type scanChunkMsg struct {
	chunk []scanner.FileInfo
}

type coverDoneMsg struct {
	id   string
	ansi string
	err  error
}

// Model represents the main TUI application state.
type Model struct {
	targetDir         string
	language          string
	protocol          GraphicProtocol
	scanner           *scanner.Scanner
	matcher           *matcher.Matcher
	scraperClient     *scraper.Client

	files             []scanner.FileInfo
	matches           []matcher.MatchResult
	metadataCache     map[string]*scraper.Movie
	coverCache        map[string]string // ID -> formatted image string
	scrapeErrors      map[string]string

	matchedCount      int
	unmatchedCount    int

	cursor            int
	scrollOffset      int
	width             int
	height            int

	isScanning        bool
	isScraping        bool
	isCoverLoading    bool
	showCover         bool
	isFullscreenCover bool
	scanChan          chan []scanner.FileInfo
	statusMessage     string
	keys              KeyMap
	spinner           spinner.Model
	editModal         EditModal
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
	scClient.SetCache(cache.Default())

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	model := &Model{
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
	}
	model.scanChan, _ = model.startScanCmd()
	return model, nil
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
	return "Truecolor"
}

// Init triggers initial directory scanning and spinner tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		listenScanChunk(m.scanChan),
	)
}

func (m Model) startScanCmd() (chan []scanner.FileInfo, tea.Cmd) {
	ch := make(chan []scanner.FileInfo, 20)
	go func() {
		defer close(ch)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Stream in small chunks of 5 files for immediate responsiveness
		_, _ = m.scanner.ScanStream(ctx, m.targetDir, 5, ch)
	}()
	return ch, listenScanChunk(ch)
}

func listenScanChunk(ch <-chan []scanner.FileInfo) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return scanDoneMsg{}
		}
		chunk, ok := <-ch
		if !ok {
			return scanDoneMsg{}
		}
		return scanChunkMsg{chunk: chunk}
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
		ansi, err := FetchAndRenderCover(id, coverURL, m.protocol, targetW, targetH)
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
		m.coverCache = make(map[string]string) // Invalidate in-memory ANSI on resize to re-scale from disk images

	case spinner.TickMsg:
		if m.isScanning || m.isScraping || m.isCoverLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case scanChunkMsg:
		if len(msg.chunk) > 0 {
			isFirstChunk := len(m.files) == 0
			newMatches := m.matcher.Match(msg.chunk)
			m.files = append(m.files, msg.chunk...)
			m.matches = append(m.matches, newMatches...)
			m.recomputeStats()
			m.statusMessage = fmt.Sprintf("🔍 Discovered %d files (%d matched)...", len(m.files), m.matchedCount)

			// Auto-fetch metadata for the first item as soon as it appears
			if isFirstChunk && len(m.matches) > 0 && m.matches[0].ID != "" {
				cmds = append(cmds, m.scrapeMovieCmd(m.matches[0].ID))
			}
		}
		if m.scanChan != nil {
			cmds = append(cmds, listenScanChunk(m.scanChan))
		}

	case scanDoneMsg:
		m.isScanning = false
		m.scanChan = nil
		// Re-run matching on full slice for directory-wide multi-part sibling validation
		if len(m.files) > 0 {
			m.matches = m.matcher.Match(m.files)
			m.recomputeStats()
		}
		m.statusMessage = fmt.Sprintf("✅ Scan complete: %d files found (%d matched)", len(m.files), m.matchedCount)

	case scrapeDoneMsg:
		m.isScraping = false
		if msg.err != nil {
			m.scrapeErrors[msg.id] = msg.err.Error()
			m.statusMessage = fmt.Sprintf("⚠️ Scrape failed for %s: %v", msg.id, msg.err)
		} else {
			m.metadataCache[msg.id] = msg.movie
			delete(m.scrapeErrors, msg.id)
			m.statusMessage = fmt.Sprintf("🎉 Loaded metadata for %s", msg.id)

			// Persist to SQLite DB
			if d, dErr := db.Default(); dErr == nil && d != nil {
				_ = d.SaveMovie(msg.movie)
			}

			// Fetch high-res cover preview if available and not cached yet
			targetURL := msg.movie.CoverURL
			if targetURL == "" {
				targetURL = msg.movie.PosterURL
			}
			if targetURL != "" && m.showCover {
				if _, ok := m.coverCache[msg.id]; !ok {
					m.isCoverLoading = true
					coverW, coverH := m.getCoverTargetDims()
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

		// Fullscreen Cover Mode
		if m.isFullscreenCover {
			switch msg.String() {
			case "v", "esc", "enter", "backspace":
				m.isFullscreenCover = false
				return m, clearKittyCmd()

			case "o":
				if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
					if imgPath, found := cache.Default().GetImagePath(curMatch.ID); found && imgPath != "" {
						_ = openFileInDefaultApp(imgPath)
						m.statusMessage = fmt.Sprintf("🖼️ Opened %s image in macOS Preview", curMatch.ID)
					}
				}
				return m, nil

			case "left", "h", "up", "k":
				if m.cursor > 0 {
					m.cursor--
					m.adjustScroll()
					cmds = append(cmds, clearKittyCmd())
					if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
						if _, ok := m.metadataCache[curMatch.ID]; !ok {
							cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
						}
					}
				}
				return m, tea.Batch(cmds...)

			case "right", "l", "down", "j":
				if m.cursor < len(m.matches)-1 {
					m.cursor++
					m.adjustScroll()
					cmds = append(cmds, clearKittyCmd())
					if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
						if _, ok := m.metadataCache[curMatch.ID]; !ok {
							cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
						}
					}
				}
				return m, tea.Batch(cmds...)

			case "q", "ctrl+c":
				return m, tea.Quit

			default:
				return m, nil
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

		case "pgdown", "ctrl+f", "d":
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

		case "v":
			m.isFullscreenCover = true

		case "a":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				if mov, ok := m.metadataCache[curMatch.ID]; ok && len(mov.Actresses) > 0 {
					act := mov.Actresses[0]
					d, _ := db.Default()
					if d != nil {
						followed, _ := d.IsActressFollowed(act.Name)
						if followed {
							_ = d.UnfollowActress(act.Name)
							m.statusMessage = fmt.Sprintf("🗑️ Unfollowed %s", act.Name)
						} else {
							_ = d.FollowActress(act.Name, act.JaName, act.ImageURL)
							m.statusMessage = fmt.Sprintf("⭐ Followed %s (%s)", act.Name, act.JaName)
						}
					}
				} else {
					m.statusMessage = "⚠️ Scrape metadata first (press Enter) to follow actress"
				}
			}

		case "t":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				d, _ := db.Default()
				if d != nil {
					isWatched, _ := d.ToggleWatched(curMatch.ID)
					if isWatched {
						m.statusMessage = fmt.Sprintf("👁️ Marked %s as Watched", curMatch.ID)
					} else {
						m.statusMessage = fmt.Sprintf("👓 Marked %s as Unwatched", curMatch.ID)
					}
				}
			}

		case "1", "2", "3", "4", "5":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				rating := int(msg.String()[0] - '0')
				d, _ := db.Default()
				if d != nil {
					_ = d.SetRating(curMatch.ID, rating)
					m.statusMessage = fmt.Sprintf("⭐ Rated %s: %s (%d/5)", curMatch.ID, strings.Repeat("⭐", rating), rating)
				}
			}

		case "f":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				d, _ := db.Default()
				if d != nil {
					isFav, _ := d.ToggleFavorite(curMatch.ID)
					if isFav {
						m.statusMessage = fmt.Sprintf("❤️ Added %s to Favorites", curMatch.ID)
					} else {
						m.statusMessage = fmt.Sprintf("🤍 Removed %s from Favorites", curMatch.ID)
					}
				}
			}

		case "w":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				if mov, ok := m.metadataCache[curMatch.ID]; ok && mov != nil {
					d, _ := db.Default()
					var userState *db.UserState
					if d != nil {
						userState, _ = d.GetUserState(curMatch.ID)
					}
					destRoot := filepath.Dir(m.targetDir)
					if destRoot == "" || destRoot == "." {
						destRoot = m.targetDir
					}
					res, err := organizer.OrganizeMatch(context.Background(), curMatch, mov, userState, destRoot, false)
					if err != nil {
						m.statusMessage = fmt.Sprintf("❌ Organize error: %v", err)
					} else {
						m.statusMessage = fmt.Sprintf("✅ Organized %s into %s", curMatch.ID, filepath.Base(res.TargetFolder))
					}
				} else {
					m.statusMessage = "⚠️ Scrape metadata first (press Enter) before organizing"
				}
			}

		case "o":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				if imgPath, found := cache.Default().GetImagePath(curMatch.ID); found && imgPath != "" {
					_ = openFileInDefaultApp(imgPath)
					m.statusMessage = fmt.Sprintf("🖼️ Opened %s image in macOS Preview", curMatch.ID)
				} else if mov, ok := m.metadataCache[curMatch.ID]; ok {
					targetURL := mov.CoverURL
					if targetURL == "" {
						targetURL = mov.PosterURL
					}
					if targetURL != "" {
						go func() {
							_, _ = FetchAndRenderCover(curMatch.ID, targetURL, m.protocol, 40, 20)
							if p, ok := cache.Default().GetImagePath(curMatch.ID); ok {
								_ = openFileInDefaultApp(p)
							}
						}()
						m.statusMessage = fmt.Sprintf("📥 Downloading and opening %s in macOS Preview...", curMatch.ID)
					}
				} else {
					m.statusMessage = "⚠️ Scrape metadata first (press Enter) to download high-res cover"
				}
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
						targetURL := mov.CoverURL
						if targetURL == "" {
							targetURL = mov.PosterURL
						}
						if targetURL != "" {
							coverW, coverH := m.getCoverTargetDims()
							cmds = append(cmds, m.fetchCoverCmd(curMatch.ID, targetURL, coverW, coverH))
						}
					}
				}
			} else {
				m.statusMessage = "🖼️ Cover view hidden"
			}

		case "enter", "space":
			if curMatch := m.currentMatch(); curMatch != nil && curMatch.ID != "" {
				m.isScraping = true
				m.statusMessage = fmt.Sprintf("🔍 Scraping %s from R18.dev...", curMatch.ID)
				cmds = append(cmds, m.scrapeMovieCmd(curMatch.ID))
			}

		case "r":
			m.files = nil
			m.matches = nil
			m.matchedCount = 0
			m.unmatchedCount = 0
			m.cursor = 0
			m.scrollOffset = 0
			m.isScanning = true
			m.statusMessage = "🔄 Rescanning directory..."
			var cmd tea.Cmd
			m.scanChan, cmd = m.startScanCmd()
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) getCoverTargetDims() (int, int) {
	// Total available width for detail panel:
	// detailInnerWidth is ~ 48% of (m.width - 9)
	detailInnerWidth := (m.width - 9) * 48 / 100
	targetW := detailInnerWidth - 4
	if targetW < 24 {
		targetW = 24
	}
	if targetW > 44 {
		targetW = 44
	}

	targetH := (m.height - 12) / 2
	if targetH < 10 {
		targetH = 10
	}
	if targetH > 18 {
		targetH = 18
	}

	return targetW, targetH
}

func clearKittyCmd() tea.Cmd {
	return func() tea.Msg {
		fmt.Print("\x1b_Ga=d,d=a\x1b\\")
		return nil
	}
}

func openFileInDefaultApp(filePath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", filePath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	default: // linux, bsd
		cmd = exec.Command("xdg-open", filePath)
	}
	return cmd.Start()
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
