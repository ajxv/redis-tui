package tui

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	jsonKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))  // cyan-green
	jsonStrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // yellow
	jsonNumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // pink
	jsonBoolStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	jsonPunctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // gray
)

// tryPrettyJSON returns an indented JSON string if s is valid JSON, otherwise s unchanged.
func tryPrettyJSON(s string) string {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return s
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

// colorizeJSON applies ANSI terminal colors to pre-formatted JSON.
func colorizeJSON(s string) string {
	var out strings.Builder
	runes := []rune(s)
	n := len(runes)
	i := 0

	for i < n {
		ch := runes[i]

		switch {
		case ch == '"':
			start := i
			i++ // skip opening quote
			for i < n {
				if runes[i] == '\\' {
					i += 2
					continue
				}
				if runes[i] == '"' {
					i++ // skip closing quote
					break
				}
				i++
			}
			token := string(runes[start:i])
			// Look ahead past whitespace to check for colon (key indicator)
			j := i
			for j < n && (runes[j] == ' ' || runes[j] == '\t') {
				j++
			}
			if j < n && runes[j] == ':' {
				out.WriteString(jsonKeyStyle.Render(token))
			} else {
				out.WriteString(jsonStrStyle.Render(token))
			}

		case ch == 't' || ch == 'f' || ch == 'n':
			var keyword string
			switch {
			case i+4 <= n && string(runes[i:i+4]) == "true":
				keyword = "true"
			case i+5 <= n && string(runes[i:i+5]) == "false":
				keyword = "false"
			case i+4 <= n && string(runes[i:i+4]) == "null":
				keyword = "null"
			}
			if keyword != "" {
				out.WriteString(jsonBoolStyle.Render(keyword))
				i += len([]rune(keyword))
			} else {
				out.WriteRune(ch)
				i++
			}

		case ch == '-' || (ch >= '0' && ch <= '9'):
			start := i
			for i < n {
				c := runes[i]
				if c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' || (c >= '0' && c <= '9') {
					i++
				} else {
					break
				}
			}
			out.WriteString(jsonNumStyle.Render(string(runes[start:i])))

		case ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ':' || ch == ',':
			out.WriteString(jsonPunctStyle.Render(string(ch)))
			i++

		default:
			out.WriteRune(ch)
			i++
		}
	}

	return out.String()
}
