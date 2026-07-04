package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Input          textarea.Model
	Type           InputType
	Hint           string // optional override for the prompt label
	Width          int
	Height         int
	RecentPatterns []string // populated by the parent model; shown in scan pattern view
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }
		case "enter":
			return m, func() tea.Msg {
				return InputCompleteMsg{Value: m.Input.Value(), Type: m.Type}
			}
		case "tab":
			// Filesystem path completion on the import/export prompts.
			if m.Type == InputFilePath {
				m.Input.SetValue(completePath(m.Input.Value()))
				m.Input.CursorEnd()
				return m, nil
			}
		}
	}

	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}

// completePath completes a partially-typed filesystem path to the longest
// common prefix of the matching entries in its directory. A sole match that is
// a directory gets a trailing "/". Returns the input unchanged on no match.
func completePath(in string) string {
	dir, base := filepath.Split(in)
	readDir := dir
	if readDir == "" {
		readDir = "."
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return in
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		if e.IsDir() {
			name += "/"
		}
		matches = append(matches, name)
	}
	switch len(matches) {
	case 0:
		return in
	case 1:
		return dir + matches[0]
	}

	lcp := matches[0]
	for _, m := range matches[1:] {
		lcp = commonPrefix(lcp, m)
	}
	return dir + lcp
}

// commonPrefix returns the longest shared leading substring of a and b.
func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
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
		title = "Scan keys by pattern:"
	case InputFilePath:
		if m.Hint != "" {
			title = m.Hint
		} else {
			title = "Input the destination/source file path:"
		}
	default:
		return ""
	}

	// Size the textarea to available window space. Key/field/pattern/file-path
	// prompts are single-line (enter submits rather than inserting a newline),
	// so only the value prompt grows to fill the window.
	const reservedLines = 5
	h := 1
	if m.Type == InputValue {
		h = 3
		if m.Height > reservedLines {
			h = m.Height - reservedLines
		}
	}
	w := 78
	if m.Width > 2 {
		w = m.Width - 2
	}

	// Match the "› " prompt + accent cursor used by the hash/set/zset/list
	// add-item overlay so every input screen reads the same way. The prompt
	// only appears on the first visual row — otherwise every wrapped line and
	// every blank filler row down to the bottom of a tall textarea (the value
	// prompt) would repeat it. SetPromptFunc must run before SetWidth: SetWidth
	// reserves room for the prompt using the width SetPromptFunc records, so
	// calling it first left the reserved width at 0 and the textarea's
	// internal viewport a couple columns narrower than what it actually drew,
	// clipping content out of view.
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent))
	blank := strings.Repeat(" ", lipgloss.Width(pointerGlyph))
	m.Input.SetPromptFunc(lipgloss.Width(pointerGlyph), func(lineIdx int) string {
		if lineIdx == 0 {
			return pointerGlyph
		}
		return blank
	})
	m.Input.FocusedStyle.Prompt = accent
	m.Input.BlurredStyle.Prompt = accent
	m.Input.Cursor.Style = accent

	m.Input.SetWidth(w)
	m.Input.SetHeight(h)

	hm := NewHelp()
	hm.Width = m.Width
	keys := help.KeyMap(inputKeys)
	if m.Type == InputFilePath {
		keys = filePathKeys
	}
	foot := footerSep(m.Width) + "\n  " + hm.View(keys)

	titleView := "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle)).Render(title)

	var body string
	if m.Type == InputPattern {
		body = m.patternBody(titleView)
	} else {
		body = titleView + "\n" + indentLines(m.Input.View(), 2)
	}
	// m.Height is the full window; subtract the 2-line connection header.
	return bottomFooter(body, foot, m.Height-2)
}

// indentLines prefixes every line of s with n spaces.
func indentLines(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

var (
	patternExampleKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Bold(true)
	patternSectionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	patternChipStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle))
)

func (m InputModel) patternBody(title string) string {
	// Compact glob hints (v2): a single row of pattern tokens.
	hints := []struct{ tok, desc string }{
		{"*", "any"},
		{"?", "one char"},
		{"[dp]", "class"},
	}
	var hintRow strings.Builder
	for i, hnt := range hints {
		if i > 0 {
			hintRow.WriteString("   ")
		}
		hintRow.WriteString(patternExampleKeyStyle.Render(hnt.tok) + " " + helpTextStyle.Render(hnt.desc))
	}

	body := title + "\n" + indentLines(m.Input.View(), 2) + "\n\n  " + hintRow.String()

	if len(m.RecentPatterns) > 0 {
		var chips strings.Builder
		for i, p := range m.RecentPatterns {
			if i > 0 {
				chips.WriteString("   ")
			}
			chips.WriteString(patternChipStyle.Render(p))
		}
		body += "\n\n  " + patternSectionStyle.Render("recent:") + "  " + chips.String()
	}

	return body
}

// prependPattern adds p to the front of the recent-patterns list, deduplicates,
// and keeps at most 5 entries.
func prependPattern(patterns []string, p string) []string {
	out := make([]string, 0, len(patterns)+1)
	for _, existing := range patterns {
		if existing != p {
			out = append(out, existing)
		}
	}
	out = append([]string{p}, out...)
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}
