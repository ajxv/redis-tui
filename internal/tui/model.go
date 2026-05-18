package tui

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ajxv/redis-tui/internal/redis"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	CurrentState           AppState
	StateNavigationHistory []AppState
	MenuList               list.Model
	Input                  InputModel
	Output                 string
	ActiveKey              string
	ActiveField            string
	ActiveIndex            int
	ActiveValue            string
	ActiveTTL              string
	PreservedTTL           int
	CopyStatus             string
	SelectedOp             Op
	Conn                   net.Conn
	RedisAddress           string
	Password               string
	Username               string
	DB                     int
	TLSConfig              *tls.Config
	DialTimeout            time.Duration
	ReadTimeout            time.Duration
	ReconnectAttempts      int
	LastPattern            string
	Reader                 *bufio.Reader
	Browser                BrowserModel
	Spinner                spinner.Model
	WindowWidth            int
	WindowHeight           int
}

func (m *Model) pushState(state AppState) {
	n := len(m.StateNavigationHistory)
	if n > 0 && m.StateNavigationHistory[n-1] == state {
		return
	}
	m.StateNavigationHistory = append(m.StateNavigationHistory, state)
}

func (m *Model) popState() AppState {
	if len(m.StateNavigationHistory) == 0 {
		return StateMenu
	}

	// get last state
	lastIndex := len(m.StateNavigationHistory) - 1
	lastState := m.StateNavigationHistory[lastIndex]

	// remove last state from stack
	m.StateNavigationHistory = m.StateNavigationHistory[:lastIndex]

	return lastState
}

func (m Model) switchToLoadingAndExecute(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.CurrentState = StateLoading
	// Re-seed the spinner tick so it animates on every loading entry.
	// Without this, the tick chain dies after the first time we leave StateLoading,
	// and the spinner freezes on all subsequent loads.
	return m, tea.Batch(m.Spinner.Tick, cmd)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, connectToRedis(m), textinput.Blink)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// handle keyboard events
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	case BackMsg:
		m.CurrentState = m.popState()
		// Sync Input.Type and Hint with the recovered state.
		// Without this, esc from a nested input (e.g. ZAdd member → score) leaves
		// Input.Type pointing at the wrong step, causing the form title to be wrong
		// and InputCompleteMsg to carry the wrong Type when the user submits.
		switch m.CurrentState {
		case StateInputKey:
			m.Input.Type = InputKey
			m.Input.Hint = ""
		case StateInputField:
			m.Input.Type = InputField
			if m.SelectedOp == OpZAdd {
				m.Input.Hint = "Input the score:"
			} else {
				m.Input.Hint = ""
			}
		case StateInputValue:
			m.Input.Type = InputValue
			switch m.SelectedOp {
			case OpZAdd:
				m.Input.Hint = "Input the member:"
			case OpExpirySet:
				m.Input.Hint = "TTL in seconds (enter 0 to remove expiry / PERSIST):"
			default:
				m.Input.Hint = ""
			}
		case StateInputFilePath:
			m.Input.Type = InputFilePath
			m.Input.Hint = ""
		default:
			m.Input.Hint = ""
		}

	case SelectKeyMsg:
		m.ActiveKey = msg.Key

		m.pushState(m.CurrentState)

		cmd := redis.RedisCmd{
			Name: "TYPE",
			Args: []string{m.ActiveKey},
		}

		m.SelectedOp = OpCheckType
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

	case InputCompleteMsg:
		// Handle the data based on what kind of input it was
		switch msg.Type {
		case InputPattern:
			m.ActiveKey = msg.Value
			switch m.SelectedOp {
			case OpExplore:
				m.LastPattern = m.ActiveKey
				m.Browser.Cursor = "0"
				m.Browser.Pattern = m.ActiveKey
				return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, m.ActiveKey, "0"))
			}

		case InputKey:
			m.ActiveKey = msg.Value

			// decide where to go next
			switch m.SelectedOp {
			case OpGet:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpSet, OpRPush, OpLPush, OpSAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				// clear previous input
				m.Input.Input.SetValue("")

			case OpHSet, OpZAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputField
				m.Input.Type = InputField
				if m.SelectedOp == OpZAdd {
					m.Input.Hint = "Input the score:"
				} else {
					m.Input.Hint = ""
				}
				// clear previous input
				m.Input.Input.SetValue("")

			case OpHGet:
				cmd := redis.RedisCmd{
					Name: "HKEYS",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = OpHKeys

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpExport:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputFilePath
				m.Input.Type = InputFilePath
				// clear previous input
				m.Input.Input.SetValue("")

			case OpDelete:
				cmd := redis.RedisCmd{
					Name: "DEL",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = OpDelete

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			}

		case InputField:
			m.ActiveField = msg.Value

			// decide where to go next
			switch m.SelectedOp {
			case OpHSet, OpZAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				if m.SelectedOp == OpZAdd {
					m.Input.Hint = "Input the member:"
				} else {
					m.Input.Hint = ""
				}
				// clear previous input
				m.Input.Input.SetValue("")
			}

		case InputValue:
			m.ActiveValue = msg.Value

			switch m.SelectedOp {
			case OpSet:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpHSet, OpZAdd:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveField, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpLSet:
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, strconv.Itoa(m.ActiveIndex), m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpRPush, OpLPush, OpSAdd:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpRename:
				cmd := redis.RedisCmd{
					Name: "RENAME",
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				m.ActiveKey = m.ActiveValue // keep model in sync
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpExpirySet:
				if m.ActiveValue == "0" {
					cmd := redis.RedisCmd{
						Name: "PERSIST",
						Args: []string{m.ActiveKey},
					}
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
				}
				cmd := redis.RedisCmd{
					Name: "EXPIRE",
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			}

		case InputFilePath:
			filePath := msg.Value

			switch m.SelectedOp {
			case OpExport:
				return m.switchToLoadingAndExecute(ExportSingleKey(m.Conn, m.Reader, m.ActiveKey, filePath))
			case OpImport:
				return m.switchToLoadingAndExecute(ImportKeys(m.Conn, m.Reader, filePath))
			case OpExportDB:
				return m.switchToLoadingAndExecute(ExportFullDB(m.Conn, m.Reader, filePath))
			case OpImportDB:
				return m.switchToLoadingAndExecute(ImportKeys(m.Conn, m.Reader, filePath))
			}
		}

	case SelectFieldMsg:
		m.ActiveField = msg.Field
		m.ActiveIndex = msg.Index

		// Save state so we can go back
		m.pushState(m.CurrentState)

		switch m.SelectedOp {
		case OpHGet, OpHKeys, OpExplore:
			cmd := redis.RedisCmd{
				Name: "HGET",
				Args: []string{m.ActiveKey, m.ActiveField},
			}
			m.SelectedOp = OpHGet
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

		case OpExploreList:
			m.Output = m.ActiveField
			m.CurrentState = StateOutput

		case OpExploreSet, OpExploreZSet:
			// Set members and ZSet members are their own values; display directly.
			// The score for ZSet is visible in the list description but not surfaced
			// here — the user can copy the member name with 'c'.
			m.Output = m.ActiveField
			m.CurrentState = StateOutput
		}

	case DeleteRequestMsg:
		m.ActiveKey = msg.Key
		m.ActiveField = msg.Field

		// Set the correct Op based on context
		if msg.Field == "" {
			m.SelectedOp = OpDel // Delete Key
		} else {
			// Delete Field/Member
			switch m.SelectedOp {
			case OpExploreList:
				m.SelectedOp = OpLRem
			case OpExploreSet:
				m.SelectedOp = OpSRem
			case OpExploreZSet:
				m.SelectedOp = OpZRem
			default:
				m.SelectedOp = OpHDel
			}
		}

		m.pushState(m.CurrentState)
		m.CurrentState = StateConfirmation

	case RenameRequestMsg:
		m.ActiveKey = msg.Key
		m.SelectedOp = OpRename
		m.Input.Input.SetValue(msg.Key)
		m.Input.Type = InputValue
		m.Input.Hint = ""
		m.Input.Input.Focus()
		m.Input.Input.CursorEnd()
		m.pushState(m.CurrentState)
		m.CurrentState = StateInputValue

	case tea.WindowSizeMsg:
		m.WindowWidth = msg.Width
		m.WindowHeight = msg.Height

		// handle resizing events
		m.MenuList.SetWidth(msg.Width)
		m.MenuList.SetHeight(msg.Height)

		m.Browser.FieldsList.SetWidth(msg.Width)
		m.Browser.FieldsList.SetHeight(msg.Height)

		m.Browser.KeyList.SetHeight(msg.Height)
		m.Browser.KeyList.SetWidth(msg.Width)

	case TickMsg:
		return m, connectToRedis(m)

	case spinner.TickMsg:
		if m.CurrentState == StateLoading {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}

	case RedisTTLResultMsg:
		if msg.TTL == -1 || msg.TTL == -2 {
			m.ActiveTTL = "no expiry"
		} else {
			m.ActiveTTL = strconv.Itoa(msg.TTL) + "s"
		}
		return m, nil

	case ClearCopyStatusMsg:
		m.CopyStatus = ""
		return m, nil

	case RedisConnectionMsg:
		return handleRedisConnection(m, msg)

	case LoadMoreKeysMsg:
		return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, m.Browser.Pattern, m.Browser.Cursor))

	case LoadMoreFieldsMsg:
		switch m.SelectedOp {
		case OpExploreList:
			end := m.Browser.FieldOffset + fieldPageSize - 1
			cmd := redis.RedisCmd{Name: "LRANGE", Args: []string{m.ActiveKey, strconv.Itoa(m.Browser.FieldOffset), strconv.Itoa(end)}}
			m.SelectedOp = OpLRange
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		case OpExploreSet:
			cmd := redis.RedisCmd{Name: "SSCAN", Args: []string{m.ActiveKey, m.Browser.FieldCursor, "COUNT", strconv.Itoa(fieldPageSize)}}
			m.SelectedOp = OpSMembers
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		case OpExploreZSet:
			end := m.Browser.FieldOffset + fieldPageSize - 1
			cmd := redis.RedisCmd{Name: "ZRANGE", Args: []string{m.ActiveKey, strconv.Itoa(m.Browser.FieldOffset), strconv.Itoa(end), "WITHSCORES"}}
			m.SelectedOp = OpZRange
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		}

	case RefreshMsg:
		if m.Browser.ViewingFields {
			// Instead of trusting stale state, let's confidently check the key TYPE again.
			// This automatically drops into OpCheckType, which correctly re-routes to
			// OpHKeys, OpExploreList, etc., OR catches if the key was deleted in the meantime!
			m.SelectedOp = OpCheckType
			cmd := redis.RedisCmd{Name: "TYPE", Args: []string{m.ActiveKey}}
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		} else {
			// Top-level key scan
			m.SelectedOp = OpExplore
			pattern := m.Browser.Pattern
			if pattern == "" {
				pattern = "*"
			}
			m.Browser.Cursor = "0"
			return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, pattern, "0"))
		}
	case RedisResultMsg:
		return handleRedisResult(m, msg)
	}

	switch m.CurrentState {
	case StateMenu:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			isFiltering := m.MenuList.FilterState() == list.Filtering

			switch keyMsg.String() {
			case "enter":
				// While the filter input is active, let the list handle Enter
				// (it confirms the filter and moves to FilterApplied state).
				if !isFiltering {
					selectedItem := m.MenuList.SelectedItem()
					if selectedItem, ok := selectedItem.(ListItem); ok {
						m.SelectedOp = ParseOp(selectedItem.title)

						// save state history
						m.pushState(m.CurrentState)

						switch m.SelectedOp {
						case OpSet, OpGet, OpHSet, OpHGet, OpDelete, OpRPush, OpLPush, OpSAdd, OpZAdd, OpExport:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("") // Clear previous input
							m.CurrentState = StateInputKey
							m.Input.Type = InputKey
						case OpImport, OpExportDB, OpImportDB:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("") // Clear previous input
							m.CurrentState = StateInputFilePath
							m.Input.Type = InputFilePath
						case OpExplore:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("*") // Default search is everything
							m.CurrentState = StateInputKey
							m.Input.Type = InputPattern
						case OpInfo:
							cmd := redis.RedisCmd{
								Name: "INFO",
							}
							return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
						}
					}
				}
			case "esc", "q":
				// While filtering, let the list handle Esc (cancels filter).
				// Only show the quit dialog when the menu is idle.
				if !isFiltering {
					m.SelectedOp = OpQuit
					m.pushState(m.CurrentState)
					m.CurrentState = StateConfirmation
					return m, nil
				}
			default:
				// Any printable character while the filter is not yet active
				// automatically opens the filter input, then the character is
				// forwarded so it appears immediately in the search box.
				if !isFiltering && len(keyMsg.Runes) == 1 {
					m.MenuList, _ = m.MenuList.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
				}
			}
		}

		updatedModel, cmd := m.MenuList.Update(msg)
		m.MenuList = updatedModel
		return m, cmd

	case StateInputKey, StateInputField, StateInputValue, StateInputFilePath:
		inputModel, cmd := m.Input.Update(msg)
		m.Input = inputModel
		return m, cmd

	case StateOutput:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return handleStateOutputKey(m, keyMsg)
		}

	case StateBrowser:
		browserModel, cmd := m.Browser.Update((msg))
		m.Browser = browserModel
		return m, cmd

	case StateConfirmation:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return handleStateConfirmationKey(m, keyMsg)
		}

	}

	return m, nil
}

func (m Model) View() string {
	switch m.CurrentState {
	case StateMenu:
		return m.MenuList.View()
	case StateInputKey, StateInputField, StateInputValue, StateInputFilePath:
		return m.Input.View()
	case StateOutput:
		helpTextStr := "esc: return • e: edit • c: copy • x: ttl"
		switch m.SelectedOp {
		case OpInfo:
			helpTextStr = "esc: return • c: copy"
		case OpExploreSet, OpExploreZSet:
			helpTextStr = "esc: return • c: copy • x: ttl"
		}
		helpText := helpTextStyle.Render(helpTextStr)
		ttlText := ""
		if m.ActiveTTL != "" {
			ttlText = "\n\nTTL: " + m.ActiveTTL
		}
		copyText := ""
		if m.CopyStatus != "" {
			copyText = "\n\n" + statusTextStyle.Render(m.CopyStatus)
		}
		return "\nOutput: " + statusTextStyle.Render(m.Output) + ttlText + copyText + "\n\n" + helpText
	case StateBrowser:
		return m.Browser.View()
	case StateLoading:
		return "\n  " + m.Spinner.View() + " Loading..."
	case StateConfirmation:
		msg := ""
		switch m.SelectedOp {
		case OpDel:
			msg = "Are you sure you want to delete the key:\n" + (m.ActiveKey)

		case OpHDel:
			msg = "Are you sure you want to delete the field:\n" + (m.ActiveField)

		case OpLRem:
			msg = "Remove one instance of value:\n" + (m.ActiveField)

		case OpSRem, OpZRem:
			msg = "Are you sure you want to delete the set member:\n" + (m.ActiveField)

		case OpQuit:
			msg = "Are you sure you want to exit Redis TUI?"

		default:
			msg = "Are you sure you want to perform this action:\n" + m.SelectedOp.String()
		}
		prompt := fmt.Sprintf("\n%s\n\n[y] Confirm   [n / esc] Cancel\n", msg)
		styledPrompt := warningStyle.Render(prompt)
		return lipgloss.Place(m.WindowWidth, m.WindowHeight, lipgloss.Center, lipgloss.Center, styledPrompt)
	default:
		return ""
	}
}
