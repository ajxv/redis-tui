package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// browserDelegate renders key/field list items as single rows (v2 GitHub-dark):
//   - selected item: ❯ marker + accent title + tinted full-width background
//   - type / index / score label: color-coded and right-justified
//   - used for both the key browser and the fields/members browser
type browserDelegate struct{}

func (d browserDelegate) Height() int                             { return 1 }
func (d browserDelegate) Spacing() int                            { return 1 }
func (d browserDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// BrowserDelegate returns the delegate used for key and fields lists.
func BrowserDelegate() browserDelegate { return browserDelegate{} }

// Each item is a single row: the key/field name on the left and a color-coded
// type / index / score label right-justified against the row's right edge.
func (d browserDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li, ok := item.(ListItem)
	if !ok {
		return
	}

	width := m.Width()
	if width < 20 {
		width = 80
	}
	isSelected := index == m.Index()

	// Action rows ("＋ new key…") have no type label; render them in green.
	if li.action != "" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(tnGreen))
		if isSelected {
			fmt.Fprintf(w, "%s%s",
				lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Render(pointerGlyph),
				style.Bold(true).Render(li.title))
		} else {
			fmt.Fprintf(w, "  %s", style.Render(li.title))
		}
		return
	}

	descText, descStyle := fieldDescStyle(li.desc)

	// A TTL badge only ever applies to top-level keys (li.ttl is unset — left
	// at its meaningless zero value — for fields/members), so it's shown
	// alongside the type label rather than replacing it.
	ttlBadge := ""
	if li.ttl >= 0 && isKeyTypeDesc(li.desc) {
		ttlBadge = "  " + formatTTL(li.ttl)
	}
	ttlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnFaint))
	if li.ttl >= 0 && li.ttl < 10 {
		ttlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(tnRed))
	}

	// marker + name on the left, type/index/score label right-justified.
	var marker, title string
	if isSelected {
		marker = lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Render(pointerGlyph)
		title = lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Bold(true).Render(li.title)
	} else {
		marker = "  "
		title = lipgloss.NewStyle().Foreground(lipgloss.Color(tnText)).Render(li.title)
	}

	labelWidth := lipgloss.Width(descText) + lipgloss.Width(ttlBadge)
	gap := width - 2 - lipgloss.Width(li.title) - labelWidth - 2
	if gap < 1 {
		gap = 1
	}
	fmt.Fprintf(w, "%s%s%s%s%s", marker, title, strings.Repeat(" ", gap), descStyle.Render(descText), ttlStyle.Render(ttlBadge))
}

type ListItem struct {
	index  int
	title  string
	desc   string
	group  string // non-empty on the first item of a menu group; renders a section label above it
	action string // non-empty marks a special non-data row (e.g. "newkey")

	// ttl is the key's remaining TTL in seconds (-1 means no expiry), set only
	// by the top-level key scan. Every other ListItem (fields, members, the
	// "+new key" action row...) leaves this at its zero value, but that's
	// harmless: the renderer only ever reads it when desc is a bare type name
	// (hash/list/set/zset/string), which none of those rows ever have.
	ttl int
}

func NewListItem(title, desc string) ListItem {
	return ListItem{title: title, desc: desc}
}

// NewListItemInGroup creates a ListItem that will render a section header label
// above it in the grouped menu delegate.
func NewListItemInGroup(title, desc, group string) ListItem {
	return ListItem{title: title, desc: desc, group: group}
}

// NewActionItem creates a special, non-data list row (e.g. "＋ new key…"). The
// action string identifies it so selection can be routed accordingly.
func NewActionItem(title, action string) ListItem {
	return ListItem{title: title, action: action}
}

func (li ListItem) Title() string       { return li.title }
func (li ListItem) Description() string { return li.desc }
func (li ListItem) FilterValue() string { return li.title }

type BrowserModel struct {
	KeyList    list.Model
	FieldsList list.Model

	ActiveKey   string
	ActiveField string
	ActiveIndex int

	ViewingFields bool

	Cursor  string
	Pattern string
	HasMore bool

	// Field-level pagination (lists, sets, sorted sets)
	FieldCursor   string
	FieldOffset   int
	HasMoreFields bool

	// Window dimensions available to the browser (excludes connection header).
	Width  int
	Height int

	// Type of the key currently being browsed ("hash", "list", "set", "zset", "string").
	ActiveKeyType string

	// bubbles/help for consistent footer keybindings.
	Help help.Model

	// Add-item overlay (hash/set/zset/list). FieldInput holds step 0, ValueInput
	// holds step 1 (only used by the two-step hash/zset forms).
	AddingField  bool
	AddFieldStep int
	FieldInput   textinput.Model
	ValueInput   textinput.Model

	// Key-picker mode: the key list is a chooser of existing keys of PickerType
	// (plus a synthetic "＋ new key…" row) for a menu collection command.
	Picking    bool
	PickerType string
}

func (m BrowserModel) Init() tea.Cmd { return nil }

// Messages sent to the parent Model.
type BackMsg struct{}

type SelectKeyMsg struct{ Key string }

type SelectFieldMsg struct {
	Key   string
	Field string
	Index int
}

type DeleteRequestMsg struct {
	Key   string
	Field string
}

type LoadMoreKeysMsg struct{}
type LoadMoreFieldsMsg struct{}

type RenameRequestMsg struct{ Key string }

type RefreshMsg struct{}

// NewKeyRequestMsg is emitted when the "＋ new key…" picker row is chosen.
type NewKeyRequestMsg struct{}

// FieldExportRequestMsg exports the selected field/member to a JSON file.
type FieldExportRequestMsg struct {
	Field string
	Index int
}

// FieldImportRequestMsg imports a single field/member from a JSON file.
type FieldImportRequestMsg struct{}

// AddItemMsg commits the add-item overlay. A is the field/member/value; B is the
// value/score for the two-step hash and zset forms.
type AddItemMsg struct {
	Key  string
	Type string
	A    string
	B    string
}

// addStepLabels returns the input-step labels for adding to a key of keyType.
// A length of 2 means a two-step form, 1 means a single field.
func addStepLabels(keyType string) []string {
	switch keyType {
	case "hash":
		return []string{"field name", "value"}
	case "zset":
		return []string{"member", "score"}
	case "set":
		return []string{"member"}
	case "list":
		return []string{"value"}
	}
	return nil
}

// addOpLabel returns the Redis command shown in the add overlay header.
func addOpLabel(keyType string) string {
	switch keyType {
	case "hash":
		return "HSET"
	case "zset":
		return "ZADD"
	case "set":
		return "SADD"
	case "list":
		return "RPUSH"
	}
	return "ADD"
}

func (m BrowserModel) Update(msg tea.Msg) (BrowserModel, tea.Cmd) {
	// Route all input to the overlay when it is active.
	if m.AddingField {
		return m.updateAddField(msg)
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		activeList := &m.KeyList
		if m.ViewingFields {
			activeList = &m.FieldsList
		}
		filterState := activeList.FilterState()

		// While the filter text box is being typed into, every key here (esc,
		// enter, and the single-letter shortcuts below) must go to the
		// underlying list instead of being hijacked as a command — otherwise
		// esc would exit the browser instead of cancelling the filter, enter
		// would select whatever's highlighted instead of committing the
		// filter text, and typing e.g. "d" or "r" into a search query would
		// trigger delete/rename instead of appending to it.
		if filterState == list.Filtering {
			break
		}

		switch msg.String() {
		case "ctrl+r", "f5":
			return m, func() tea.Msg { return RefreshMsg{} }

		case "esc":
			// A committed (non-empty) filter is still applied here, even
			// though the user isn't actively typing into it — let the list's
			// own ClearFilter binding clear it first. Only once there's no
			// filter left does esc fall back to this browser's own
			// navigation, or clearing a filter would double as "go back".
			if filterState == list.FilterApplied {
				break
			}
			if m.ViewingFields {
				m.ViewingFields = false
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			if m.ViewingFields {
				if selected, ok := m.FieldsList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg {
						return SelectFieldMsg{Key: m.ActiveKey, Field: selected.Title(), Index: selected.index}
					}
				}
			} else {
				if selected, ok := m.KeyList.SelectedItem().(ListItem); ok {
					if selected.action == "newkey" {
						return m, func() tea.Msg { return NewKeyRequestMsg{} }
					}
					m.ActiveKey = selected.Title()
					return m, func() tea.Msg { return SelectKeyMsg{Key: selected.Title()} }
				}
			}

		case "n":
			if m.ViewingFields && m.HasMoreFields {
				return m, func() tea.Msg { return LoadMoreFieldsMsg{} }
			}
			if !m.ViewingFields && m.HasMore {
				return m, func() tea.Msg { return LoadMoreKeysMsg{} }
			}

		case "d":
			if m.ViewingFields {
				if item, ok := m.FieldsList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg { return DeleteRequestMsg{Key: m.ActiveKey, Field: item.Title()} }
				}
			} else {
				if item, ok := m.KeyList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg { return DeleteRequestMsg{Key: item.Title()} }
				}
			}

		case "r":
			if !m.ViewingFields {
				if item, ok := m.KeyList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg { return RenameRequestMsg{Key: item.Title()} }
				}
			}

		case "a":
			// Add an item — hash/set/zset/list (each has its own step layout).
			if m.ViewingFields && len(addStepLabels(m.ActiveKeyType)) > 0 {
				cmd := m.StartAdd()
				return m, cmd
			}

		case "x":
			// Export the selected field/member to a self-describing JSON file.
			if m.ViewingFields {
				if item, ok := m.FieldsList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg {
						return FieldExportRequestMsg{Field: item.Title(), Index: item.index}
					}
				}
			}

		case "i":
			// Import a single field/member from a JSON file.
			if m.ViewingFields {
				return m, func() tea.Msg { return FieldImportRequestMsg{} }
			}
		}
	}

	if m.ViewingFields {
		m.FieldsList, cmd = m.FieldsList.Update(msg)
	} else {
		m.KeyList, cmd = m.KeyList.Update(msg)
	}

	return m, cmd
}

// StartAdd opens the add-item overlay for the current ActiveKey/ActiveKeyType
// and returns the cursor-blink command.
func (m *BrowserModel) StartAdd() tea.Cmd {
	m.AddingField = true
	m.AddFieldStep = 0
	m.FieldInput.SetValue("")
	m.FieldInput.Focus()
	m.ValueInput.SetValue("")
	m.ValueInput.Blur()
	return textinput.Blink
}

// updateAddField handles all input while the add-item overlay is shown.
func (m BrowserModel) updateAddField(msg tea.Msg) (BrowserModel, tea.Cmd) {
	twoStep := len(addStepLabels(m.ActiveKeyType)) == 2

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.AddingField = false
			m.AddFieldStep = 0
			m.FieldInput.SetValue("")
			m.ValueInput.SetValue("")
			return m, nil

		case "enter":
			// Two-step forms advance from step 0 to step 1 (blank step 0 blocks).
			if twoStep && m.AddFieldStep == 0 {
				if strings.TrimSpace(m.FieldInput.Value()) == "" {
					return m, nil
				}
				m.AddFieldStep = 1
				m.FieldInput.Blur()
				m.ValueInput.Focus()
				return m, textinput.Blink
			}
			// Commit (single-step at step 0, or two-step at step 1).
			a := strings.TrimSpace(m.FieldInput.Value())
			if a == "" {
				return m, nil
			}
			key, keyType, b := m.ActiveKey, m.ActiveKeyType, m.ValueInput.Value()
			m.AddingField = false
			m.AddFieldStep = 0
			return m, func() tea.Msg {
				return AddItemMsg{Key: key, Type: keyType, A: a, B: b}
			}
		}
	}

	var cmd tea.Cmd
	if m.AddFieldStep == 0 {
		m.FieldInput, cmd = m.FieldInput.Update(msg)
	} else {
		m.ValueInput, cmd = m.ValueInput.Update(msg)
	}
	return m, cmd
}

func (m BrowserModel) View() string {
	h := m.Help
	h.Width = m.Width

	if m.AddingField {
		foot := footerSep(m.Width) + "\n  " + h.View(inputKeys)
		// m.Height is the full window; subtract the 2-line connection header.
		return bottomFooter(m.addFieldOverlayView(), foot, m.Height-2)
	}

	var listView, helpView string
	if m.ViewingFields {
		listView = m.FieldsList.View()
		if m.ActiveKeyType == "hash" {
			helpView = h.View(hashFieldsKeys)
		} else {
			helpView = h.View(otherFieldsKeys)
		}
	} else {
		listView = m.KeyList.View()
		helpView = h.View(browserKeys)
	}
	return listView + "\n" + footerSep(m.Width) + "\n  " + helpView
}

// addFieldOverlayView renders the add-item flow inline (v2): a header, then one
// or two left-aligned step labels + inputs depending on the key type.
func (m BrowserModel) addFieldOverlayView() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	subtle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(tnText))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color(tnGreen))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent))
	dimmer := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDimmer))

	labels := addStepLabels(m.ActiveKeyType)
	if len(labels) == 0 {
		return ""
	}
	twoStep := len(labels) == 2
	step0Done := m.AddFieldStep > 0

	head := dim.Render(addOpLabel(m.ActiveKeyType)+" · ") + subtle.Render(m.ActiveKey)

	// Step 0.
	var s0Label, s0Row string
	if twoStep && step0Done {
		s0Label = green.Render(labels[0] + "  ✓")
		s0Row = dimmer.Render(pointerGlyph) + subtle.Render(m.FieldInput.Value())
	} else {
		s0Label = accent.Render(labels[0])
		fi := m.FieldInput
		fi.Width = 60
		fi.Prompt = pointerGlyph
		fi.PromptStyle = accent
		fi.TextStyle = text
		// The step label above already names this field ("member", "value"...);
		// the input's own placeholder is configured once in main.go for the
		// hash case ("field name") and would otherwise show that same fixed
		// text under a "member"/"score" label for zset/set/list adds.
		fi.Placeholder = ""
		s0Row = fi.View()
	}

	content := head + "\n\n" + s0Label + "\n" + s0Row

	// Step 1 (two-step forms only).
	if twoStep {
		var s1Label string
		vi := m.ValueInput
		vi.Width = 60
		vi.Prompt = pointerGlyph
		vi.Placeholder = ""
		if step0Done {
			s1Label = accent.Render(labels[1])
			vi.PromptStyle = accent
			vi.TextStyle = text
		} else {
			s1Label = dim.Render(labels[1])
			vi.PromptStyle = dimmer
			vi.TextStyle = subtle
		}
		content += "\n\n" + s1Label + "\n" + vi.View()
	}

	return indentLines(content, 2)
}
