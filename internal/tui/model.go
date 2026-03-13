package tui

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"

	"github.com/ajxv/redis-tui/internal/redis"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	SelectedOp             string
	Conn                   net.Conn
	RedisAddress           string
	Password               string
	DB                     int
	LastPattern            string
	Reader                 *bufio.Reader
	Browser                BrowserModel
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
	return tea.Batch(connectToRedis(m.RedisAddress, m.Password, m.DB), textinput.Blink)
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

		cmd := redis.RedisCmd{
			Name: "TYPE",
			Args: []string{m.ActiveKey},
		}

		m.SelectedOp = "CHECK_TYPE"
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

	case InputCompleteMsg:
		// Handle the data based on what kind of input it was
		switch msg.Type {
		case InputKey:
			m.ActiveKey = msg.Value

			// decide where to go next
			switch m.SelectedOp {
			case "EXPLORE":
				m.LastPattern = m.ActiveKey
				m.Browser.Cursor = "0"
				m.Browser.Pattern = m.ActiveKey
				return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, m.ActiveKey, "0"))
			case "GET":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "SET", "RPUSH", "SADD":
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				// clear previous input
				m.Input.Input.SetValue("")

			case "HSET", "ZADD":
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputField
				m.Input.Type = InputField
				// clear previous input
				m.Input.Input.SetValue("")

			case "HGET":
				cmd := redis.RedisCmd{
					Name: "HKEYS",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = "HKEYS"

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "DELETE":
				cmd := redis.RedisCmd{
					Name: "DEL",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = "DELETE"

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			}

		case InputField:
			m.ActiveField = msg.Value

			// decide where to go next
			switch m.SelectedOp {
			case "HSET", "ZADD":
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				// clear previous input
				m.Input.Input.SetValue("")
			}

		case InputValue:
			m.ActiveValue = msg.Value

			switch m.SelectedOp {
			case "SET":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "HSET", "ZADD":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveField, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "LSET":
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, strconv.Itoa(m.ActiveIndex), m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "RPUSH", "SADD":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			}
		}

	case SelectFieldMsg:
		m.ActiveField = msg.Field
		m.ActiveIndex = msg.Index

		// Save state so we can go back
		m.pushState(m.CurrentState)

		switch m.SelectedOp {
		case "HGET", "HKEYS", "EXPLORE":
			cmd := redis.RedisCmd{
				Name: "HGET",
				Args: []string{m.ActiveKey, m.ActiveField},
			}
			m.SelectedOp = "HGET"
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case "EXPLORE_LIST":
			// Simple output for list items
			m.Output = m.ActiveField
			m.CurrentState = StateOutput
		}

	case DeleteRequestMsg:
		m.ActiveKey = msg.Key
		m.ActiveField = msg.Field

		// Set the correct Op based on context
		if msg.Field == "" {
			m.SelectedOp = "DEL" // Delete Key
		} else {
			// Delete Field/Member
			switch m.SelectedOp {
			case "EXPLORE_LIST":
				m.SelectedOp = "LREM"
			case "EXPLORE_SET":
				m.SelectedOp = "SREM"
			case "EXPLORE_ZSET":
				m.SelectedOp = "ZREM"
			default:
				m.SelectedOp = "HDEL"
			}
		}

		m.pushState(m.CurrentState)
		m.CurrentState = StateConfirmation

	case tea.WindowSizeMsg:
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
		}

		switch m.SelectedOp {
		case "GET", "HGET":
			if result, ok := msg.Result.(string); ok {
				m.Output = result
				m.CurrentState = StateOutput
			}

		case "HKEYS":
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

		case "EXPLORE":
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

		case "LRANGE":
			if resp, ok := msg.Result.([]any); ok {
				var items []list.Item
				for index, key := range resp {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{index: index, title: key, desc: "Index: " + strconv.Itoa(index)})
					}
				}
				m.SelectedOp = "EXPLORE_LIST"
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case "SMEMBERS":
			if resp, ok := msg.Result.([]any); ok {
				var items []list.Item
				for index, key := range resp {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{index: index, title: key, desc: "Index: " + strconv.Itoa(index)})
					}
				}
				m.SelectedOp = "EXPLORE_SET"
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case "ZRANGE":
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
				m.SelectedOp = "EXPLORE_ZSET"
				// update browsers field list
				m.Browser.FieldsList.SetItems(items)
				// tell browser we are looking at fields
				m.Browser.ViewingFields = true
				// switch to browser state
				m.CurrentState = StateBrowser

			}

		case "CHECK_TYPE":
			if str, ok := msg.Result.(string); ok {
				switch str {
				case "string":
					cmd := redis.RedisCmd{
						Name: "GET",
						Args: []string{m.ActiveKey},
					}

					m.SelectedOp = "GET"
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "hash":
					cmd := redis.RedisCmd{
						Name: "HKEYS",
						Args: []string{m.ActiveKey},
					}

					m.SelectedOp = "HKEYS"
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "list":
					cmd := redis.RedisCmd{
						Name: "LRANGE",
						Args: []string{m.ActiveKey, "0", "-1"},
					}

					m.SelectedOp = "LRANGE"
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "set":
					cmd := redis.RedisCmd{
						Name: "SMEMBERS",
						Args: []string{m.ActiveKey},
					}

					m.SelectedOp = "SMEMBERS"
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "zset":
					cmd := redis.RedisCmd{
						Name: "ZRANGE",
						Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"},
					}

					m.SelectedOp = "ZRANGE"
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				}
			} else {
				m.Output = "Unexpected response"
				m.CurrentState = StateOutput
			}

		case "SET", "LSET":
			if str, ok := msg.Result.(string); ok {
				m.Output = str
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput

		case "DEL":
			m.Output = "Deleted Key: " + m.ActiveKey

			m.SelectedOp = "EXPLORE"
			// refresh the keylist using the last pattern searched
			pattern := m.LastPattern
			if pattern == "" {
				pattern = "*"
			}
			m.Browser.Cursor = "0"
			m.Browser.Pattern = pattern
			return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, pattern, "0"))

		case "HDEL":
			m.Output = "Deleted Hash Key: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "HKEYS",
				Args: []string{m.ActiveKey},
			}

			m.SelectedOp = "HKEYS"

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case "LREM":
			m.Output = "Removed element from list: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "LRANGE",
				Args: []string{m.ActiveKey, "0", "-1"},
			}

			m.SelectedOp = "LRANGE"

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case "SREM":
			m.Output = "Removed element from set: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "SMEMBERS",
				Args: []string{m.ActiveKey},
			}

			m.SelectedOp = "SMEMBERS"

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case "ZREM":
			m.Output = "Removed element from sorted set: " + m.ActiveKey

			cmd := redis.RedisCmd{
				Name: "ZRANGE",
				Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"},
			}

			m.SelectedOp = "ZRANGE"

			// refresh the keylist
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

		case "DELETE", "HSET", "RPUSH", "SADD", "ZADD":
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
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			selectedItem := m.MenuList.SelectedItem()
			if selectedItem, ok := selectedItem.(ListItem); ok {
				m.SelectedOp = selectedItem.title

				// save state history
				m.pushState(m.CurrentState)

				switch m.SelectedOp {
				case "SET", "GET", "HSET", "HGET", "DELETE", "RPUSH", "SADD", "ZADD":
					m.Input.Input.Focus()
					m.CurrentState = StateInputKey
					m.Input.Type = InputKey
				case "EXPLORE":
					m.Input.Input.Focus()
					m.Input.Input.SetValue("*") // Default search is everything
					m.CurrentState = StateInputKey
					m.Input.Type = InputKey
				}
			}
		}

		updatedModel, cmd := m.MenuList.Update(msg)
		m.MenuList = updatedModel
		return m, cmd

	case StateInputKey, StateInputField, StateInputValue:
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

				previousState := m.popState()
				// 1. Handle the "Edit Loop" artifact
				// If the previous state was ALSO Output, pop one more time to find the List.
				if previousState == StateOutput {
					previousState = m.popState()
				}

				// 2. Handle the "Creation Flow" (Hard Reset)
				if previousState == StateInputKey || previousState == StateInputField {
					m.StateNavigationHistory = []AppState{} // Clear history
					m.CurrentState = StateMenu              // Go to Menu
					return m, nil
				}

				// 3. Fallback: Go to the previous state (Browser or List)
				m.CurrentState = previousState

				// 4. GLOBAL CLEANUP: Reset the Op Mode
				// This ensures that 'Enter' works again when we land on the List.
				switch m.SelectedOp {
				case "LSET":
					m.SelectedOp = "EXPLORE_LIST"
				case "HSET":
					m.SelectedOp = "HKEYS"
				}

				return m, nil

			case "e":
				m.Input.Input.SetValue(m.Output)
				switch m.SelectedOp {
				case "GET":
					m.SelectedOp = "SET"

				case "HGET":
					m.SelectedOp = "HSET"

				case "EXPLORE_LIST":
					m.SelectedOp = "LSET"
				}

				m.Input.Input.Focus()
				m.Input.Input.CursorEnd()
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
				case "DEL":
					cmd := redis.RedisCmd{
						Name: m.SelectedOp,
						Args: []string{m.ActiveKey},
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				case "HDEL":
					cmd := redis.RedisCmd{
						Name: m.SelectedOp,
						Args: []string{m.ActiveKey, m.ActiveField},
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				case "LREM":
					cmd := redis.RedisCmd{
						Name: m.SelectedOp,
						Args: []string{m.ActiveKey, "1", m.ActiveField}, // removes one instance of element
					}

					// MANUAL LOADING (Preserve History)
					m.CurrentState = StateLoading
					return m, sendRedisCmd(m.Conn, m.Reader, cmd)

				case "SREM", "ZREM":
					cmd := redis.RedisCmd{
						Name: m.SelectedOp,
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
	case StateInputKey, StateInputField, StateInputValue:
		return m.Input.View()
	case StateOutput:
		helpText := helpTextStyle.Render("Esc: Return • e: Edit")
		return "\nOutput: " + statusTextStyle.Render(m.Output) + "\n\n" + helpText
	case StateBrowser:
		return m.Browser.View()
	case StateLoading:
		return "Loading.."
	case StateConfirmation:
		switch m.SelectedOp {
		case "DEL":
			return "Are you sure you want to delete the key: " + (m.ActiveKey) + "? (y/n)"

		case "HDEL":
			return "Are you sure you want to delete the field: " + (m.ActiveField) + "? (y/n)"

		case "LREM":
			return "Remove one instance of value: " + (m.ActiveField) + "? (y/n)"

		case "SREM", "ZREM":
			return "Are you sure you want to delete the set member: " + (m.ActiveField) + "? (y/n)"

		default:
			return "Are you sure you want to perform this action: " + (m.SelectedOp) + "? (y/n)"
		}
	default:
		return ""
	}
}
