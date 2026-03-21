package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// alertLevel controls the visual style of an alert.
type alertLevel int

const (
	alertSuccess alertLevel = iota
	alertError
	alertInfo
)

const alertDuration = 2 * time.Second

// alertDismissMsg is sent when an alert should be hidden.
type alertDismissMsg struct{}

// alertModel manages a temporary overlay notification.
type alertModel struct {
	message string
	level   alertLevel
	visible bool
}

// Show displays an alert overlay.
func (m *alertModel) Show(message string, level alertLevel) tea.Cmd {
	m.message = message
	m.level = level
	m.visible = true
	return tea.Tick(alertDuration, func(_ time.Time) tea.Msg {
		return alertDismissMsg{}
	})
}

// Dismiss hides the alert.
func (m *alertModel) Dismiss() {
	m.visible = false
	m.message = ""
}

// Visible returns whether the alert is currently showing.
func (m alertModel) Visible() bool {
	return m.visible
}

// Render returns the styled alert box for overlay compositing.
func (m alertModel) Render() string {
	if !m.visible || m.message == "" {
		return ""
	}

	var icon string
	var style lipgloss.Style

	switch m.level {
	case alertSuccess:
		icon = "✓  "
		style = styleAlertSuccess
	case alertError:
		icon = "✗  "
		style = styleAlertError
	default:
		icon = "●  "
		style = styleAlertInfo
	}

	return style.Render(icon + m.message)
}
