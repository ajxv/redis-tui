package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type InputType int

const (
	InputNone InputType = iota
	InputKey
	InputField
	InputValue
	InputScore
	InputMember
)

type InputCompleteMsg struct {
	Value string
	Type  InputType
}

type InputModel struct {
	Input textinput.Model
	Type  InputType
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

	// pass eveything else to text input handler
	m.Input, cmd = m.Input.Update(msg)

	return m, cmd
}

func (m InputModel) View() string {

	var title string

	switch m.Type {
	case InputKey:
		title = "Input the key:"
	case InputValue:
		title = "Input the Value:"
	case InputField:
		title = "Input the Field:"
	case InputScore:
		title = "Input the Score:"
	case InputMember:
		title = "Input the Member:"
	default:
		return ""
	}

	// Return the title + newline + the text input view
	helpText := helpTextStyle.Render("esc: return • enter: submit")
	return title + "\n" + m.Input.View() + "\n\n" + helpText
}
