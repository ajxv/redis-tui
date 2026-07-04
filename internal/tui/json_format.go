package tui

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GitHub-dark color palette for JSON highlighting (v2).
var (
	jsonKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#79c0ff")) // light blue
	jsonStrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a8c48b")) // muted green
	jsonNumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffa657")) // orange
	jsonBoolStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")) // red
	jsonPunctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#58626c")) // faint
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

// colorizeInfo styles raw Redis INFO output: lines that start with '#' (section
// headers) are dimmed, every other line is rendered in the subtle text color.
func colorizeInfo(s string) string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle))
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		t := strings.TrimRight(ln, "\r")
		if strings.HasPrefix(strings.TrimSpace(t), "#") {
			lines[i] = dim.Render(t)
		} else {
			lines[i] = sub.Render(t)
		}
	}
	return strings.Join(lines, "\n")
}

// colorizeJSON applies Tokyo Night ANSI colors to pre-formatted JSON.
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
			i++
			for i < n {
				if runes[i] == '\\' {
					i += 2
					continue
				}
				if runes[i] == '"' {
					i++
					break
				}
				i++
			}
			token := string(runes[start:i])
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
