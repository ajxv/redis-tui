package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"net"
)

type AppState int

const (
	StateMenu AppState = iota
	StateInputKey
	StateInputField
	StateFieldSelect
	StateInputValue
	StateOutput
	StateBrowser
	StateLoading
	StateConfirmation
)

var statusTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#04B575")). // A nice bright green
	Bold(true)

var helpTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("241")) // A subtle gray

type ScanResult struct {
	Cursor string
	Keys   []list.Item
}

type RedisResultMsg struct {
	Result any
	Error  error
}

type RedisConnectionMsg struct {
	Conn  net.Conn
	Error error
}

// A message to tell us the wait time is over
type TickMsg struct{}
