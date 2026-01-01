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
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AppState int

const (
	StateMenu AppState = iota
	StateInputKey
	StateInputField
	StateFieldSelect
	StateInputValue
	StateOutput
	StateBrowser
	StateLoading
)

type ListItem struct {
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

var statusStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#04B575")). // A nice bright green
	Bold(true)

type RedisResultMsg struct {
	Result any
	Error  error
}

type RedisConnectionMsg struct {
	Conn  net.Conn
	Error error
}

// A message to tell us the wait time is over
type TickMsg struct{}

// A command that waits for 2 seconds, then returns the TickMsg
func waitForNextConnection() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)

		return TickMsg{}
	}
}

func connectToRedis(address string) tea.Cmd {
	return func() tea.Msg {
		// dial the address
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return RedisConnectionMsg{
				Error: err,
			}
		}

		return RedisConnectionMsg{
			Conn: conn,
		}
	}
}

func scanRedisKeys(conn net.Conn, reader *bufio.Reader) tea.Cmd {
	return func() tea.Msg {
		cursor := "0"
		filter := "*"
		var keys []list.Item
		for {
			cmd := redis.RedisCmd{
				Name: "SCAN",
				Args: []string{cursor, "MATCH", filter},
			}
			_, err := conn.Write(cmd.ToBytes())
			if err != nil {
				return RedisResultMsg{
					Error: err,
				}
			}
			response, err := redis.ReadResp(reader)
			if err != nil {
				return RedisResultMsg{
					Error: err,
				}
			}
			if resp, ok := response.([]any); ok {
				if c, ok := resp[0].(string); ok {
					cursor = c
				}

				if slice, ok := resp[1].([]any); ok {
					for _, str := range slice {
						if s, ok := str.(string); ok {
							keys = append(keys, ListItem{title: s, desc: "key"})
						}
					}
				}
			}

			// break if no more records
			if cursor == "0" {
				break
			}
		}

		return RedisResultMsg{
			Result: keys,
		}
	}
}

func sendRedisCmd(conn net.Conn, reader *bufio.Reader, cmd redis.RedisCmd) tea.Cmd {
	return func() tea.Msg {
		// SAFETY CHECK: If there is no connection, return an error immediately
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		// 1. Send the command to Redis (conn.Write)
		// 2. Read the response (redis.ReadResp)
		// 3. Return a RedisResultMsg

		_, err := conn.Write(cmd.ToBytes())
		if err != nil {
			return RedisResultMsg{
				Error: err,
			}
		}
		response, err := redis.ReadResp(reader)
		if err != nil {
			return RedisResultMsg{
				Error: err,
			}
		}

		return RedisResultMsg{
			Result: response,
		}
	}
}

type Model struct {
	CurrentState  AppState
	PreviousState AppState
	MenuList      list.Model
	FieldsList    list.Model
	KeyList       list.Model
	Input         textinput.Model
	Output        string
	ViewPort      viewport.Model
	ActiveKey     string
	ActiveField   string
	ActiveValue   string
	SelectedOp    string
	Conn          net.Conn
	RedisAddress  string
	Reader        *bufio.Reader
}

func (m Model) switchToLoadingAndExecute(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	// save current state
	if m.CurrentState != StateLoading {
		m.PreviousState = m.CurrentState
	}
	// change to loading state
	m.CurrentState = StateLoading

	return m, cmd
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(connectToRedis(m.RedisAddress), textinput.Blink)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// handle keyboard events
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		// handle resizing events
		m.MenuList.SetWidth(msg.Width)
		m.MenuList.SetHeight(msg.Height)

		m.FieldsList.SetWidth(msg.Width)
		m.FieldsList.SetHeight(msg.Height)

		m.KeyList.SetHeight(msg.Height)
		m.KeyList.SetWidth(msg.Width)

		m.ViewPort.Width = msg.Width
		m.ViewPort.Height = msg.Height

	case TickMsg:
		return m, connectToRedis(m.RedisAddress)

	case RedisConnectionMsg:
		if msg.Error != nil {
			return m, waitForNextConnection()
		}

		conn := msg.Conn
		// create and set reader
		reader := bufio.NewReader(conn)
		m.Reader = reader
		m.Conn = conn
		m.CurrentState = m.PreviousState

	case RedisResultMsg:
		if msg.Error != nil {
			var netError net.Error
			if msg.Error == io.EOF || errors.As(msg.Error, &netError) {
				// retry connection for connection errors (server restart)
				if m.CurrentState != StateLoading {
					m.PreviousState = m.CurrentState
				}
				m.CurrentState = StateLoading
				return m, connectToRedis(m.RedisAddress)
			}

			m.Output = msg.Error.Error()
			m.CurrentState = StateOutput
		}

		switch m.SelectedOp {
		case "GET":
			if result, ok := msg.Result.(string); ok {
				m.Output = result
				m.CurrentState = StateOutput
			}

		case "HGET", "HKEYS":
			if result, ok := msg.Result.([]any); ok {
				var items []list.Item
				for _, key := range result {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{title: key, desc: "Hash Field"})
					}
				}
				m.FieldsList.SetItems(items)
				// change state
				m.CurrentState = StateFieldSelect

			}

		case "EXPLORE":
			if result, ok := msg.Result.([]list.Item); ok {
				m.KeyList.SetItems(result)
				m.CurrentState = StateBrowser
			}

		case "LRANGE", "SMEMBERS":
			if resp, ok := msg.Result.([]any); ok {
				var items []list.Item
				for index, key := range resp {
					if key, ok := key.(string); ok {
						items = append(items, ListItem{title: key, desc: "Index: " + strconv.Itoa(index)})
					}
				}
				m.FieldsList.SetItems(items)
				// change state
				m.SelectedOp = "EXPLORE_LIST"
				m.CurrentState = StateFieldSelect
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
				m.FieldsList.SetItems(items)
				// change state
				m.SelectedOp = "EXPLORE_LIST"
				m.CurrentState = StateFieldSelect
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
			}

		case "SET", "HSET":
			if str, ok := msg.Result.(string); ok {
				m.Output = str
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

				switch m.SelectedOp {
				case "SET", "GET", "HSET", "HGET":
					m.Input.Focus()
					m.CurrentState = StateInputKey
				case "EXPLORE":
					return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader))
				}
			}
		}

		updatedModel, cmd := m.MenuList.Update(msg)
		m.MenuList = updatedModel
		return m, cmd

	case StateInputKey:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.String() == "esc" {
			m.Input.SetValue("")
			m.CurrentState = StateMenu
			m.Output = ""
			return m, nil
		}

		if ok && keyMsg.String() == "enter" {
			m.ActiveKey = m.Input.Value()
			// reset input
			m.Input.SetValue("")

			// decide where to go next
			switch m.SelectedOp {
			case "GET":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "SET":
				m.CurrentState = StateInputValue

			case "HSET":
				m.CurrentState = StateInputField

			case "HGET":
				cmd := redis.RedisCmd{
					Name: "HKEYS",
					Args: []string{m.ActiveKey},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			}

			return m, nil
		}

		// if any other key pressed
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd

	case StateInputField:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.String() == "esc" {
			m.Input.SetValue("")
			m.CurrentState = StateMenu
			m.Output = ""
			return m, nil
		}

		if ok && keyMsg.String() == "enter" {
			m.ActiveField = m.Input.Value()
			// reset input
			m.Input.SetValue("")

			// decide where to go next
			switch m.SelectedOp {
			case "HSET":
				m.CurrentState = StateInputValue
			}

			return m, nil
		}

		// if any other key pressed
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd

	case StateInputValue:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.String() == "esc" {
			m.Input.SetValue("")
			m.CurrentState = StateMenu
			m.Output = ""
			return m, nil
		}

		if ok && keyMsg.String() == "enter" {
			m.ActiveValue = m.Input.Value()

			// reset input
			m.Input.SetValue("")

			switch m.SelectedOp {
			case "SET":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			case "HSET":
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveField, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

			}
		}

		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd

	case StateFieldSelect:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.String() == "esc" {
			m.Input.SetValue("")
			m.CurrentState = StateMenu
			m.Output = ""
			return m, nil
		}

		if ok && keyMsg.String() == "enter" {
			selectedField := m.FieldsList.SelectedItem()
			if selectedField, ok := selectedField.(ListItem); ok {
				m.ActiveField = selectedField.title

				switch m.SelectedOp {
				case "HGET", "HKEYS", "EXPLORE":
					cmd := redis.RedisCmd{
						Name: "HGET",
						Args: []string{m.ActiveKey, m.ActiveField},
					}

					m.SelectedOp = "GET" // get already handles the expected response strucuture
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))

				case "EXPLORE_LIST":
					m.Output = m.ActiveField
					m.CurrentState = StateOutput
				}
			}
		}

		updatedModel, cmd := m.FieldsList.Update(msg)
		m.FieldsList = updatedModel
		return m, cmd

	case StateOutput:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.String() == "esc" {
			m.Input.SetValue("")
			m.CurrentState = StateMenu
			m.Output = ""
			return m, nil
		}

	case StateBrowser:
		keyMsg, ok := msg.(tea.KeyMsg)
		if ok && keyMsg.String() == "esc" {
			m.Input.SetValue("")
			m.CurrentState = StateMenu
			m.Output = ""
			return m, nil
		}

		if ok && keyMsg.String() == "enter" {
			selectedKey := m.KeyList.SelectedItem()
			if selectedKey, ok := selectedKey.(ListItem); ok {
				m.ActiveKey = selectedKey.title

				cmd := redis.RedisCmd{
					Name: "TYPE",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = "CHECK_TYPE"
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd))
			}
		}

		updatedModel, cmd := m.KeyList.Update(msg)
		m.KeyList = updatedModel
		return m, cmd

	}

	return m, nil
}

func (m Model) View() string {
	switch m.CurrentState {
	case StateMenu:
		return m.MenuList.View()
	case StateInputKey:
		return "Input the key: \n" + m.Input.View()
	case StateInputField:
		return "Input the field: \n" + m.Input.View()
	case StateFieldSelect:
		return m.FieldsList.View()
	case StateInputValue:
		return "Input the value: \n" + m.Input.View()
	case StateOutput:
		return "\nOutput: " + statusStyle.Render(m.Output) + "\n\nPress 'Esc' to return."
	case StateBrowser:
		return m.KeyList.View()
	case StateLoading:
		return "Loading.."
	default:
		return ""
	}
}
