package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// --- The Redis Protocol ---

type RedisCmd struct {
	Name string
	Args []string
}

func (cmd *RedisCmd) ToBytes() []byte {
	// 1. header
	result := fmt.Sprintf("*%d\r\n", len(cmd.Args)+1)

	// 2. command name
	result += fmt.Sprintf("$%d\r\n%s\r\n", len(cmd.Name), cmd.Name)

	// 3. arguments
	for _, arg := range cmd.Args {
		result += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	return []byte(result)
}

// --- The TUI model ---

type Model struct {
	Input  string
	Output string
	Conn   net.Conn
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// handler keyboard input
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			// DEBUG: Set this first so we know we hit the enter key
			m.Output = "Sending to Redis..."
			//  parse the input (split by spaces)
			parts := strings.Fields(m.Input)
			if len(parts) == 0 {
				return m, nil
			}

			// create the command struct
			cmd := RedisCmd{
				Name: parts[0],
				Args: parts[1:],
			}

			// send command to redis
			_, err := m.Conn.Write(cmd.ToBytes())
			if err != nil {
				m.Output = "Error sending: " + err.Error()
			}

			// read response
			buffer := make([]byte, 1024)
			n, err := m.Conn.Read(buffer)
			if err != nil {
				m.Output = "Error reading: " + err.Error()
			} else {
				// raw data might contain \r\n which messes up the TUI
				raw := string(buffer[:n])

				// Clean it up!
				m.Output = ParseResponse(raw)
			}

			// clear input
			m.Input = ""

		case "backspace":
			if len(m.Input) > 0 {
				m.Input = m.Input[:len(m.Input)-1]
			}

		default:
			// add typed character to input string
			m.Input += msg.String()
		}
	}
	return m, nil
}

func (m Model) View() string {
	return fmt.Sprintf("REDIS TUI\n\nResponse:\n%q\n\n> %s\n\n(Press Ctrl+C to quit)", m.Output, m.Input)
}

func ParseResponse(raw string) string {
	// 1. If empty, return nothing
	if len(raw) == 0 {
		return ""
	}

	// 2. Identify the type by the first byte
	prefix := raw[0]
	content := strings.TrimSpace(raw[1:]) // Remove the first char and trailing \r\n

	switch prefix {
	case '+', '-':
		// Simple String (+) or Error (-): content is already clean
		return content

	case ':':
		// Integer (:): content is just the number
		return "(int) " + content

	case '$':
		// Bulk String ($): Format is "$Length\r\nValue"
		// We need to split by the first \r\n to separate the length from the value
		parts := strings.SplitN(content, "\r\n", 2)
		if len(parts) == 2 {
			return parts[1] // Return just the value part
		}
	}

	return raw // Fallback: return raw if we don't recognize it
}

// -- Main Function--

func main() {
	// connect to redis
	conn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		fmt.Println("Could not connect to Redis: ", err)
		os.Exit(1)
	}
	defer conn.Close()

	// start BubbleTea program
	p := tea.NewProgram(Model{
		Conn: conn, // pass connection to model
	})

	if _, err := p.Run(); err != nil {
		fmt.Println("Error starting TUI: ", err)
		os.Exit(1)
	}

}
