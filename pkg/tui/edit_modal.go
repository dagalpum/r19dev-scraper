package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditModal represents a text input popup for manually overriding a JAV ID.
type EditModal struct {
	Active    bool
	FileIndex int
	Input     textinput.Model
}

// NewEditModal initializes the text input modal.
func NewEditModal() EditModal {
	ti := textinput.New()
	ti.Placeholder = "e.g. MIDA-517, KAVR-428"
	ti.CharLimit = 32
	ti.Width = 30
	return EditModal{
		Input: ti,
	}
}

// Open activates the modal for the given file index and pre-fills current ID.
func (m *EditModal) Open(index int, currentID string) tea.Cmd {
	m.Active = true
	m.FileIndex = index
	m.Input.SetValue(currentID)
	m.Input.Focus()
	return textinput.Blink
}

// Close deactivates the modal.
func (m *EditModal) Close() {
	m.Active = false
	m.Input.Blur()
}

// Render draws the modal dialog overlay.
func (m *EditModal) Render(width int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(accentColor).
		Background(bgDarkColor).
		Padding(1, 2).
		Width(45)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		labelStyle.Render("✏️  Edit / Override JAV ID:"),
		"",
		m.Input.View(),
		"",
		lipgloss.NewStyle().Foreground(mutedColor).Render("Press Enter to save, Esc to cancel"),
	)

	return box.Render(content)
}
