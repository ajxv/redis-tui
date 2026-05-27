package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type InputType int

const (
	InputNone InputType = iota
	InputKey
	InputField
	InputValue
	InputPattern
	InputFilePath
)

type InputCompleteMsg struct {
	Value string
	Type  InputType
}

type InputModel struct {
	Input  textarea.Model
	Type   InputType
	Hint   string // optional override for the prompt label
	Width  int
	Height int
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "enter":
			return m, func() tea.Msg {
				return InputCompleteMsg{
					Value: m.Input.Value(),
					Type:  m.Type,
				}
			}
		}
	}

	// pass eveything else to textarea handler
	m.Input, cmd = m.Input.Update(msg)

	return m, cmd
}

func (m InputModel) View() string {

	var title string

	switch m.Type {
	case InputKey:
		title = "Input the key:"
	case InputValue:
		if m.Hint != "" {
			title = m.Hint
		} else {
			title = "Input the Value:"
		}
	case InputField:
		if m.Hint != "" {
			title = m.Hint
		} else {
			title = "Input the Field:"
		}
	case InputPattern:
		title = "Input the search pattern:"
	case InputFilePath:
		title = "Input the destination/source file path:"
	default:
		return ""
	}

	// Size the textarea to available window space.
	// Reserve lines for the title, a blank separator, help text, and margin.
	const reservedLines = 5
	h := 3 // sensible default before first WindowSizeMsg
	if m.Height > reservedLines && m.Type == InputValue {
		h = m.Height - reservedLines
	}
	w := 78 // fallback before first WindowSizeMsg
	if m.Width > 2 {
		w = m.Width - 2
	}
	m.Input.SetWidth(w)
	m.Input.SetHeight(h)

	// Return the title + newline + the textarea view
	helpText := helpTextStyle.Render("esc: return • enter: submit")
	return title + "\n" + m.Input.View() + "\n\n" + helpText
}
