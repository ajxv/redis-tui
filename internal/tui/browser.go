package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type ListItem struct {
	index int
	title string
	desc  string
}

func NewListItem(title, desc string) ListItem {
	return ListItem{
		title: title,
		desc:  desc,
	}
}

func (li ListItem) Title() string {
	return li.title
}

func (li ListItem) Description() string {
	return li.desc
}

func (li ListItem) FilterValue() string {
	return li.title
}

type BrowserModel struct {
	KeyList    list.Model
	FieldsList list.Model

	ActiveKey   string
	ActiveField string
	ActiveIndex int

	// Helper to track which list we are looking at
	ViewingFields bool

	Cursor  string
	Pattern string
	HasMore bool

	// Field-level pagination (lists, sets, sorted sets)
	FieldCursor   string // SSCAN cursor for set pages; "" = first page
	FieldOffset   int    // item offset for list/zset pages
	HasMoreFields bool   // shows "n: load more" hint in field view
}

func (m BrowserModel) Init() tea.Cmd {
	return nil
}

// A message to signal the Main Model to go back to the previous screen
type BackMsg struct {
}

type SelectKeyMsg struct {
	Key string // The payload: which key was selected?
}

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

type RenameRequestMsg struct {
	Key string
}

type RefreshMsg struct{}

func (m BrowserModel) Update(msg tea.Msg) (BrowserModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+r", "f5":
			return m, func() tea.Msg {
				return RefreshMsg{}
			}

		case "esc":
			if m.ViewingFields {
				m.ViewingFields = false
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "enter":
			if m.ViewingFields {
				// handle field selection
				if selectedItem, ok := m.FieldsList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg {
						return SelectFieldMsg{
							Key:   m.ActiveKey,
							Field: selectedItem.Title(),
							Index: selectedItem.index,
						}
					}
				}
			} else {
				selectedKey := m.KeyList.SelectedItem()

				if selectedKey, ok := selectedKey.(ListItem); ok {
					m.ActiveKey = selectedKey.Title()

					return m, func() tea.Msg {
						return SelectKeyMsg{
							Key: selectedKey.Title(),
						}
					}
				}
			}

		case "n":
			if m.ViewingFields && m.HasMoreFields {
				return m, func() tea.Msg {
					return LoadMoreFieldsMsg{}
				}
			}
			if !m.ViewingFields && m.HasMore {
				return m, func() tea.Msg {
					return LoadMoreKeysMsg{}
				}
			}

		case "d":
			// handle delete request
			if m.ViewingFields {
				if item, ok := m.FieldsList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg {
						return DeleteRequestMsg{
							Key:   m.ActiveKey,
							Field: item.Title(),
						}
					}
				}
			} else {
				if item, ok := m.KeyList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg {
						return DeleteRequestMsg{
							Key: item.Title(),
						}
					}
				}
			}

		case "r":
			// only allow rename from the key list (not field list)
			if !m.ViewingFields {
				if item, ok := m.KeyList.SelectedItem().(ListItem); ok {
					return m, func() tea.Msg {
						return RenameRequestMsg{
							Key: item.Title(),
						}
					}
				}
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

func (m BrowserModel) View() string {
	var listView string
	var helpText string

	if m.ViewingFields {
		listView = m.FieldsList.View()
		fieldMoreHint := ""
		if m.HasMoreFields {
			fieldMoreHint = " • n: load more"
		}
		helpText = helpTextStyle.Render("esc: return • enter: select • d: delete • ctrl+r: refresh" + fieldMoreHint)
	} else {
		listView = m.KeyList.View()
		moreHint := ""
		if m.HasMore {
			moreHint = " • n: load more"
		}
		helpText = helpTextStyle.Render("esc: return • enter: select • d: delete • r: rename" + moreHint + " • ctrl+r: refresh")
	}
	return listView + "\n" + helpText
}
