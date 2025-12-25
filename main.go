package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

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

var statusStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#04B575")). // A nice bright green
	Bold(true)

type MenuItem struct {
	title string
	desc  string
}

func (mi MenuItem) Title() string {
	return mi.title
}

func (mi MenuItem) Description() string {
	return mi.desc
}

func (mi MenuItem) FilterValue() string {
	return mi.title
}

type Model struct {
	CurrentState AppState
	MenuList     list.Model
	FieldsList   list.Model
	KeyList      list.Model
	Input        textinput.Model
	Output       string
	ViewPort     viewport.Model
	ActiveKey    string
	ActiveField  string
	ActiveValue  string
	SelectedOp   string
	Conn         net.Conn
	Reader       *bufio.Reader
}

type RedisCmd struct {
	Name string
	Args []string
}

func (cmd RedisCmd) ToBytes() []byte {
	// use a bytes buffer to avoid conversion later
	var buf bytes.Buffer

	// 1. header
	fmt.Fprintf(&buf, "*%d\r\n", len(cmd.Args)+1)

	// 2. command name
	fmt.Fprintf(&buf, "$%d\r\n%s\r\n", len(cmd.Name), cmd.Name)

	// 3. arguments
	for _, arg := range cmd.Args {
		fmt.Fprintf(&buf, "$%d\r\n%s\r\n", len(arg), arg)
	}

	return buf.Bytes()
}

func ReadResp(reader *bufio.Reader) (any, error) {
	// 1. Read the prefix byte
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	switch prefix {
	case '+', '-':
		// Simple String or Error: Read until newline
		msg, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(msg), nil // Clean up the result

	case '$':
		// Bulk String: Read length first
		lengthStr, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		// Convert length to integer
		lengthNum, err := strconv.Atoi(strings.TrimSpace(lengthStr))
		if err != nil {
			return "", err
		}

		if lengthNum == -1 {
			return "(nil)", nil // Handle NULL response
		}

		// Read the exact data bytes
		data := make([]byte, lengthNum)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			return "", err
		}

		// read the trailing clrf
		_, err = reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return string(data), nil
	case '*':
		// Bulk String: Read length first
		lengthStr, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		// Convert length to integer
		lengthNum, err := strconv.Atoi(strings.TrimSpace(lengthStr))
		if err != nil {
			return "", err
		}

		if lengthNum == -1 {
			return "(nil)", nil // Handle NULL response
		}

		var items []any

		for range lengthNum {
			item, err := ReadResp(reader)
			if err != nil {
				return "", err
			}

			items = append(items, item)

		}
		return items, nil

	}

	return "", nil
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
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
	}

	switch m.CurrentState {
	case StateMenu:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			selectedItem := m.MenuList.SelectedItem()
			if selectedItem, ok := selectedItem.(MenuItem); ok {
				m.SelectedOp = selectedItem.title

				switch m.SelectedOp {
				case "SET", "GET", "HSET", "HGET":
					m.Input.Focus()
					m.CurrentState = StateInputKey
				case "EXPLORE":
					cursor := "0"
					filter := "*"
					var keys []list.Item
					for {
						cmd := RedisCmd{
							Name: "SCAN",
							Args: []string{cursor, "MATCH", filter},
						}
						_, err := m.Conn.Write(cmd.ToBytes())
						if err != nil {
							m.Output = "Error sending: " + err.Error()
							return m, nil
						}
						response, err := ReadResp(m.Reader)
						if err != nil {
							m.Output = "Error reading: " + err.Error()
							return m, nil
						}
						if resp, ok := response.([]any); ok {
							if c, ok := resp[0].(string); ok {
								cursor = c
							}

							if slice, ok := resp[1].([]any); ok {
								for _, str := range slice {
									if s, ok := str.(string); ok {
										keys = append(keys, MenuItem{title: s, desc: "key"})
									}
								}
							}
						}

						// break if no more records
						if cursor == "0" {
							break
						}
					}

					// populate viewport
					m.KeyList.SetItems(keys)

					m.CurrentState = StateBrowser
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
				cmd := RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey},
				}

				_, err := m.Conn.Write(cmd.ToBytes())
				if err != nil {
					m.Output = "Error sending: " + err.Error()
					return m, nil
				}
				response, err := ReadResp(m.Reader)
				if err != nil {
					m.Output = "Error reading: " + err.Error()
					return m, nil
				}
				if str, ok := response.(string); ok {
					m.Output = str
				} else {
					m.Output = "Unexpected response"
				}
				m.CurrentState = StateOutput

			case "SET":
				m.CurrentState = StateInputValue

			case "HSET":
				m.CurrentState = StateInputField

			case "HGET":
				cmd := RedisCmd{
					Name: "HKEYS",
					Args: []string{m.ActiveKey},
				}

				_, err := m.Conn.Write(cmd.ToBytes())
				if err != nil {
					m.Output = "Error sending: " + err.Error()
					return m, nil
				}
				response, err := ReadResp(m.Reader)
				if err != nil {
					m.Output = "Error reading: " + err.Error()
					return m, nil
				}
				if resp, ok := response.([]any); ok {
					var items []list.Item
					for _, key := range resp {
						if key, ok := key.(string); ok {
							items = append(items, MenuItem{title: key, desc: "Hash Field"})
						}
					}
					m.FieldsList.SetItems(items)
					// change state
					m.CurrentState = StateFieldSelect
				}
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
				cmd := RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				_, err := m.Conn.Write(cmd.ToBytes())
				if err != nil {
					m.Output = "Error sending: " + err.Error()
					return m, nil
				}
				response, err := ReadResp(m.Reader)
				if err != nil {
					m.Output = "Error reading: " + err.Error()
					return m, nil
				}
				if str, ok := response.(string); ok {
					m.Output = str
				} else {
					m.Output = "Unexpected response"
				}
				m.CurrentState = StateOutput

			case "HSET":
				// send command
				cmd := RedisCmd{
					Name: m.SelectedOp,
					Args: []string{m.ActiveKey, m.ActiveField, m.ActiveValue},
				}
				_, err := m.Conn.Write(cmd.ToBytes())
				if err != nil {
					m.Output = "Error sending: " + err.Error()
					return m, nil
				}
				response, err := ReadResp(m.Reader)
				if err != nil {
					m.Output = "Error reading: " + err.Error()
					return m, nil
				}
				if str, ok := response.(string); ok {
					m.Output = str
				} else {
					m.Output = "Unexpected response"
				}
				m.CurrentState = StateOutput
			}
		}

		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd

	case StateFieldSelect:
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			selectedField := m.FieldsList.SelectedItem()
			if selectedField, ok := selectedField.(MenuItem); ok {
				m.ActiveField = selectedField.title

				switch m.SelectedOp {
				case "HGET", "EXPLORE":
					cmd := RedisCmd{
						Name: "HGET",
						Args: []string{m.ActiveKey, m.ActiveField},
					}

					_, err := m.Conn.Write(cmd.ToBytes())
					if err != nil {
						m.Output = "Error sending: " + err.Error()
						return m, nil
					}
					response, err := ReadResp(m.Reader)
					if err != nil {
						m.Output = "Error reading: " + err.Error()
						return m, nil
					}
					if str, ok := response.(string); ok {
						m.Output = str
					} else {
						m.Output = "Unexpected response"
					}
					m.CurrentState = StateOutput

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
		if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
			selectedKey := m.KeyList.SelectedItem()
			if selectedKey, ok := selectedKey.(MenuItem); ok {
				m.ActiveKey = selectedKey.title

				cmd := RedisCmd{
					Name: "TYPE",
					Args: []string{m.ActiveKey},
				}

				_, err := m.Conn.Write(cmd.ToBytes())
				if err != nil {
					m.Output = "Error sending: " + err.Error()
					return m, nil
				}
				response, err := ReadResp(m.Reader)
				if err != nil {
					m.Output = "Error reading: " + err.Error()
					return m, nil
				}
				if str, ok := response.(string); ok {
					switch str {
					case "string":
						cmd := RedisCmd{
							Name: "GET",
							Args: []string{m.ActiveKey},
						}

						_, err := m.Conn.Write(cmd.ToBytes())
						if err != nil {
							m.Output = "Error sending: " + err.Error()
							return m, nil
						}
						response, err := ReadResp(m.Reader)
						if err != nil {
							m.Output = "Error reading: " + err.Error()
							return m, nil
						}
						if str, ok := response.(string); ok {
							m.Output = str
						} else {
							m.Output = "Unexpected response"
						}
						m.CurrentState = StateOutput

					case "hash":
						cmd := RedisCmd{
							Name: "HKEYS",
							Args: []string{m.ActiveKey},
						}

						_, err := m.Conn.Write(cmd.ToBytes())
						if err != nil {
							m.Output = "Error sending: " + err.Error()
							return m, nil
						}
						response, err := ReadResp(m.Reader)
						if err != nil {
							m.Output = "Error reading: " + err.Error()
							return m, nil
						}
						if resp, ok := response.([]any); ok {
							var items []list.Item
							for _, key := range resp {
								if key, ok := key.(string); ok {
									items = append(items, MenuItem{title: key, desc: "Hash Field"})
								}
							}
							m.FieldsList.SetItems(items)
							// change state
							m.CurrentState = StateFieldSelect
						}

					case "list":
						cmd := RedisCmd{
							Name: "LRANGE",
							Args: []string{m.ActiveKey, "0", "-1"},
						}

						_, err := m.Conn.Write(cmd.ToBytes())
						if err != nil {
							m.Output = "Error sending: " + err.Error()
							return m, nil
						}
						response, err := ReadResp(m.Reader)
						if err != nil {
							m.Output = "Error reading: " + err.Error()
							return m, nil
						}
						if resp, ok := response.([]any); ok {
							var items []list.Item
							for index, key := range resp {
								if key, ok := key.(string); ok {
									items = append(items, MenuItem{title: key, desc: "Index: " + strconv.Itoa(index)})
								}
							}
							m.FieldsList.SetItems(items)
							// change state
							m.SelectedOp = "EXPLORE_LIST"
							m.CurrentState = StateFieldSelect
						}

					case "set":
						cmd := RedisCmd{
							Name: "SMEMBERS",
							Args: []string{m.ActiveKey},
						}

						_, err := m.Conn.Write(cmd.ToBytes())
						if err != nil {
							m.Output = "Error sending: " + err.Error()
							return m, nil
						}
						response, err := ReadResp(m.Reader)
						if err != nil {
							m.Output = "Error reading: " + err.Error()
							return m, nil
						}
						if resp, ok := response.([]any); ok {
							var items []list.Item
							for index, key := range resp {
								if key, ok := key.(string); ok {
									items = append(items, MenuItem{title: key, desc: "Index: " + strconv.Itoa(index)})
								}
							}
							m.FieldsList.SetItems(items)
							// change state
							m.SelectedOp = "EXPLORE_LIST"
							m.CurrentState = StateFieldSelect
						}

					case "zset":
						cmd := RedisCmd{
							Name: "ZRANGE",
							Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"},
						}

						_, err := m.Conn.Write(cmd.ToBytes())
						if err != nil {
							m.Output = "Error sending: " + err.Error()
							return m, nil
						}
						response, err := ReadResp(m.Reader)
						if err != nil {
							m.Output = "Error reading: " + err.Error()
							return m, nil
						}
						if resp, ok := response.([]any); ok {
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
									items = append(items, MenuItem{title: member, desc: "Score: " + score})
								}
							}
							m.FieldsList.SetItems(items)
							// change state
							m.SelectedOp = "EXPLORE_LIST"
							m.CurrentState = StateFieldSelect
						}

					}
				} else {
					m.Output = "Unexpected response"
				}
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

func run() error {
	// define menu items
	items := []list.Item{
		MenuItem{title: "SET", desc: "Set a key-value pair"},
		MenuItem{title: "GET", desc: "Get the value of a key"},
		MenuItem{title: "HSET", desc: "Set a hash field"},
		MenuItem{title: "HGET", desc: "Get the value of a hash field"},
		MenuItem{title: "EXPLORE", desc: "Browse keys and values"},
	}

	// initialize the menu list
	menuList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menuList.Title = "Redis TUI"

	// initialize fields list
	fieldsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	fieldsList.Title = "Select a field"

	// intialize key list
	keylist := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	keylist.Title = "Select a key"

	// initialize the input
	input := textinput.New()

	// conncet to redis
	conn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		fmt.Println("Error connecting to Redis: ", err)
		return err
	}
	defer conn.Close()

	// wrap connection in reader
	reader := bufio.NewReader(conn)

	// initialize viewport
	vp := viewport.New(0, 0)

	// define initialModel
	initialModel := Model{
		CurrentState: StateMenu,
		MenuList:     menuList,
		FieldsList:   fieldsList,
		KeyList:      keylist,
		Input:        input,
		ViewPort:     vp,
		Conn:         conn,
		Reader:       reader,
	}

	// start BubbleTea program
	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Printf("An error occured: %v", err)
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
