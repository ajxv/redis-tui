package tui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/ajxv/redis-tui/internal/redis"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	CurrentState           AppState
	StateNavigationHistory []AppState
	MenuList               list.Model
	Input                  InputModel
	Output                 string
	ViewPort               viewport.Model
	ActiveKey              string
	ActiveField            string
	ActiveIndex            int
	ActiveValue            string
	ActiveTTL              string
	CopyStatus             string
	SelectedOp             Op
	Conn                   net.Conn
	RedisAddress           string
	Password               string
	DB                     int
	LastPattern            string
	Reader                 *bufio.Reader
	Browser                BrowserModel
	Spinner                spinner.Model
	WindowWidth            int
	WindowHeight           int
}

func (m *Model) pushState(state AppState) {
	// push state onto stack
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
	// change to loading state
	m.CurrentState = StateLoading

	return m, cmd
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, connectToRedis(m.RedisAddress, m.Password, m.DB), textinput.Blink)
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

	case SelectKeyMsg:
		m.ActiveKey = msg.Key

		m.pushState(m.CurrentState)

		cmd := redis.RedisCmd{
			Name: "TYPE",
			Args: []string{m.ActiveKey},
		}

		m.SelectedOp = OpCheckType
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

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

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case OpSet, OpRPush, OpSAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				// clear previous input
				m.Input.Input.SetValue("")

			case OpHSet, OpZAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputField
				m.Input.Type = InputField
				// clear previous input
				m.Input.Input.SetValue("")

			case OpHGet:
				cmd := redis.RedisCmd{
					Name: "HKEYS",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = OpHKeys

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

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

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			}

		case InputField:
			m.ActiveField = msg.Value

			// decide where to go next
			switch m.SelectedOp {
			case OpHSet, OpZAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
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

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case OpHSet, OpZAdd:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveField, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case OpLSet:
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, strconv.Itoa(m.ActiveIndex), m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case OpRPush, OpSAdd:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case OpRename:
				cmd := redis.RedisCmd{
					Name: "RENAME",
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				m.ActiveKey = m.ActiveValue // keep model in sync
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case OpExpirySet:
				if m.ActiveValue == "0" {
					cmd := redis.RedisCmd{
						Name: "PERSIST",
						Args: []string{m.ActiveKey},
					}
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))
				}
				cmd := redis.RedisCmd{
					Name: "EXPIRE",
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			}

		case InputFilePath:
			filePath := msg.Value

			switch m.SelectedOp {
			case OpExport:
				return m.switchToLoadingAndExecute(exportSingleKey(m.Conn, m.Reader, m.ActiveKey, filePath))
			case OpImport:
				return m.switchToLoadingAndExecute(importSingleKeyOrDB(m.Conn, m.Reader, filePath))
			case OpExportDB:
				return m.switchToLoadingAndExecute(exportFullDB(m.Conn, m.Reader, filePath))
			case OpImportDB:
				return m.switchToLoadingAndExecute(importSingleKeyOrDB(m.Conn, m.Reader, filePath))
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
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case OpExploreList:
			// Simple output for list items
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

		m.ViewPort.Width = msg.Width
		m.ViewPort.Height = msg.Height

	case TickMsg:
		return m, connectToRedis(m.RedisAddress, m.Password, m.DB)

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
		if msg.Error != nil {
			return m, waitForNextConnection()
		}

		conn := msg.Conn
		// create and set reader
		reader := bufio.NewReader(conn)
		m.Reader = reader
		m.Conn = conn

		if m.CurrentState == StateLoading {
			m.CurrentState = m.popState()
		}

	case LoadMoreKeysMsg:
		return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, m.Browser.Pattern, m.Browser.Cursor))

	case RefreshMsg:
		if m.Browser.ViewingFields {
			// Instead of trusting stale state, let's confidently check the key TYPE again. 
			// This automatically drops into OpCheckType, which correctly re-routes to 
			// OpHKeys, OpExploreList, etc., OR catches if the key was deleted in the meantime!
			m.SelectedOp = OpCheckType
			cmd := redis.RedisCmd{Name: "TYPE", Args: []string{m.ActiveKey}}
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))
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
		if msg.Error != nil {
			var netError net.Error
			if msg.Error == io.EOF || errors.As(msg.Error, &netError) {
				// retry connection for connection errors (server restart)
				if m.CurrentState != StateLoading {
					m.pushState(m.CurrentState)
				}
				m.CurrentState = StateLoading
				return m, connectToRedis(m.RedisAddress, m.Password, m.DB)
			}

			m.Output = msg.Error.Error()
			m.CurrentState = StateOutput
			return m, nil
		}

		switch m.SelectedOp {
		case OpGet, OpHGet, OpInfo:
			if result, ok := msg.Result.(string); ok {
				m.Output = result
				m.CurrentState = StateOutput
				if m.SelectedOp != OpInfo {
					m.ActiveTTL = "fetching..."
					return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey)
				}
			}

		case OpHKeys:
			if result, ok := msg.Result.([]any); ok {
				var items []list.Item
				for _, key := range result {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{title: key, desc: "Hash Field"})
					}
				}
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case OpExplore:
			if result, ok := msg.Result.(ScanResult); ok {
				if m.Browser.Cursor == "0" || m.Browser.Cursor == "" {
					// update browsers keylist entirely
					m.Browser.KeyList.SetItems(result.Keys)
				} else {
					// append to the current list
					items := m.Browser.KeyList.Items()
					for _, k := range result.Keys {
						items = append(items, k)
					}
					m.Browser.KeyList.SetItems(items)
				}
				m.Browser.Cursor = result.Cursor
				m.Browser.ViewingFields = false
				m.CurrentState = StateBrowser
			}

		case OpLRange:
			if resp, ok := msg.Result.([]any); ok {
				var items []list.Item
				for index, key := range resp {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{index: index, title: key, desc: "Index: " + strconv.Itoa(index)})
					}
				}
				m.SelectedOp = OpExploreList
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case OpSMembers:
			if resp, ok := msg.Result.([]any); ok {
				var items []list.Item
				for index, key := range resp {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{index: index, title: key, desc: "Index: " + strconv.Itoa(index)})
					}
				}
				m.SelectedOp = OpExploreSet
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case OpZRange:
			if resp, ok := msg.Result.([]any); ok {
				var items []list.Item
				// iterate by steps of 2 to handle scores
				for i := 0; i < len(resp); i += 2 {
					// The member is at 'i'
					member, ok1 := resp[i].(string)

					// The score is at 'i+1'
					score := "unknown"
					if i+1 < len(resp) {
						if s, ok2 := resp[i+1].(string); ok2 {
							score = s
						}
					}

					if ok1 {
						items = append(items, ListItem{title: member, desc: "Score: " + score})
					}
				}
				m.SelectedOp = OpExploreZSet
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case OpCheckType:
			if str, ok := msg.Result.(string); ok {
				switch str {
				case "string":
					cmd := redis.RedisCmd{
						Name: "GET",
						Args: []string{m.ActiveKey},
					}

					m.SelectedOp = OpGet
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "hash":
					cmd := redis.RedisCmd{
						Name: "HKEYS",
						Args: []string{m.ActiveKey},
					}

					m.SelectedOp = OpHKeys
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "list":
					cmd := redis.RedisCmd{
						Name: "LRANGE",
						Args: []string{m.ActiveKey, "0", "-1"},
					}

					m.SelectedOp = OpLRange
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "set":
					cmd := redis.RedisCmd{
						Name: "SMEMBERS",
						Args: []string{m.ActiveKey},
					}

					m.SelectedOp = OpSMembers
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "zset":
					cmd := redis.RedisCmd{
						Name: "ZRANGE",
						Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"},
					}

					m.SelectedOp = OpZRange
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "none":
					m.Output = "Key does not exist or has expired."
					m.CurrentState = StateOutput
					return m, nil
					
				default:
					m.Output = "Unknown key type: " + str
					m.CurrentState = StateOutput
					return m, nil
				}
			} else {
				m.Output = "Unexpected response"
				m.CurrentState = StateOutput
			}

		case OpSet, OpLSet, OpRename, OpExpirySet, OpExport, OpImport, OpExportDB, OpImportDB:
			if str, ok := msg.Result.(string); ok {
				m.Output = str
			} else if num, ok := msg.Result.(int); ok {
				m.Output = strconv.Itoa(num)
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput

			// If it's a rename or expiry set, update the TTL
			if m.SelectedOp == OpExpirySet || m.SelectedOp == OpRename {
				m.ActiveTTL = "fetching..."
				return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey)
			}

		case OpDel:
			m.Output = "Deleted Key: " + m.ActiveKey

			m.SelectedOp = OpExplore
			// refresh the keylist using the last pattern searched
			pattern := m.LastPattern
			if pattern == "" {
				pattern = "*"
			}
			m.Browser.Cursor = "0"
			m.Browser.Pattern = pattern
			return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, pattern, "0"))

		case OpHDel:
			m.Output = "Deleted Hash Key: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "HKEYS",
				Args: []string{m.ActiveKey},
			}

			m.SelectedOp = OpHKeys

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case OpLRem:
			m.Output = "Removed element from list: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "LRANGE",
				Args: []string{m.ActiveKey, "0", "-1"},
			}

			m.SelectedOp = OpLRange

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case OpSRem:
			m.Output = "Removed element from set: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "SMEMBERS",
				Args: []string{m.ActiveKey},
			}

			m.SelectedOp = OpSMembers

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case OpZRem:
			m.Output = "Removed element from sorted set: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "ZRANGE",
				Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"},
			}

			m.SelectedOp = OpZRange

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case OpDelete, OpHSet, OpRPush, OpSAdd, OpZAdd:
			if res, ok := msg.Result.(int); ok {
				m.Output = strconv.Itoa(res)
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput

		}
	}

	switch m.CurrentState {
	case StateMenu:
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "enter":
				selectedItem := m.MenuList.SelectedItem()
				if selectedItem, ok := selectedItem.(ListItem); ok {
					m.SelectedOp = ParseOp(selectedItem.title)

					// save state history
					m.pushState(m.CurrentState)

					switch m.SelectedOp {
					case OpSet, OpGet, OpHSet, OpHGet, OpDelete, OpRPush, OpSAdd, OpZAdd, OpExport:
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
						return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))
					}
				}
			case "esc", "q":
				m.SelectedOp = OpQuit
				m.pushState(m.CurrentState)
				m.CurrentState = StateConfirmation
				return m, nil
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
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok {
			switch keyMsg.String() {
			case "esc":
				m.Input.Input.SetValue("")
				m.Output = ""
				m.ActiveTTL = ""
				m.CopyStatus = ""

				previousState := m.popState()
				// 1. Handle the "Edit Loop" artifact
				// If the previous state was ALSO Output, pop one more time to find the List.
				if previousState == StateOutput {
					previousState = m.popState()
				}

				// 2. Handle the "Creation Flow" (Hard Reset)
				if previousState == StateInputKey || previousState == StateInputField || previousState == StateInputFilePath {
					m.StateNavigationHistory = []AppState{} // Clear history
					m.CurrentState = StateMenu              // Go to Menu
					return m, nil
				}

				// 3. Fallback: Go to the previous state (Browser or List)
				m.CurrentState = previousState

				// 4. GLOBAL CLEANUP: Reset the Op Mode
				// This ensures that 'Enter' works again when we land on the List.
				switch m.SelectedOp {
				case OpLSet:
					m.SelectedOp = OpExploreList
				case OpHSet:
					m.SelectedOp = OpHKeys
				}

				return m, nil

			case "e":
				if m.SelectedOp == OpInfo {
					break
				}
				m.Input.Input.SetValue(m.Output)
				switch m.SelectedOp {
				case OpGet:
					m.SelectedOp = OpSet

				case OpHGet:
					m.SelectedOp = OpHSet

				case OpExploreList:
					m.SelectedOp = OpLSet
				}

				m.Input.Type = InputValue
				m.Input.Input.Focus()
				m.Input.Input.CursorEnd()
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue

			case "c":
				err := clipboard.WriteAll(m.Output)
				if err != nil {
					m.CopyStatus = "Clipboard unavailable (xclip/xsel missing?)"
				} else {
					m.CopyStatus = "Copied to clipboard!"
				}
				return m, func() tea.Msg {
					time.Sleep(2 * time.Second)
					return ClearCopyStatusMsg{}
				}

			case "r":
				if m.SelectedOp == OpInfo {
					break
				}

			case "x":
				if m.SelectedOp == OpInfo {
					break
				}
				m.SelectedOp = OpExpirySet
				m.Input.Input.SetValue("")
				m.Input.Type = InputValue
				m.Input.Input.Focus()
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
			}
		}

	case StateBrowser:
		browserModel, cmd := m.Browser.Update((msg))
		m.Browser = browserModel
		return m, cmd

	case StateConfirmation:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok {
			switch keyMsg.String() {
			case "esc", "n", "N":
				m.CurrentState = m.popState()
				return m, nil

			case "y", "Y":

				m.popState()

				switch m.SelectedOp {
				case OpQuit:
					return m, tea.Quit
				case OpDel:
					cmd := redis.RedisCmd{
						Name: m.SelectedOp.String(),
						Args: []string{m.ActiveKey},
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				case OpHDel:
					cmd := redis.RedisCmd{
						Name: m.SelectedOp.String(),
						Args: []string{m.ActiveKey, m.ActiveField},
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				case OpLRem:
					cmd := redis.RedisCmd{
						Name: m.SelectedOp.String(),
						Args: []string{m.ActiveKey, "1", m.ActiveField}, // removes one instance of element
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				case OpSRem, OpZRem:
					cmd := redis.RedisCmd{
						Name: m.SelectedOp.String(),
						Args: []string{m.ActiveKey, m.ActiveField},
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				}

			}
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
		if m.SelectedOp == OpInfo {
			helpTextStr = "esc: return • c: copy"
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
