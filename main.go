package main

import (
	"bufio"
	"fmt"
	"io"
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
	Reader *bufio.Reader
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
			response, err := ReadResp(m.Reader)
			if err != nil {
				m.Output = "Error reading: " + err.Error()
			} else {
				m.Output = response
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

func ReadResp(r *bufio.Reader) (string, error) {
	// 1. Read the first byte to know the type
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	// Remove the trailing \r\n to look at the prefix
	line = strings.TrimSuffix(line, "\r\n")
	if len(line) == 0 {
		return "", nil
	}

	prefix := line[0]   // e.g., '*' or '$'
	content := line[1:] // e.g., "2" or "3"

	switch prefix {
	case '+', '-', ':':
		// Simple strings, Errors, Integers: just return the content
		return content, nil

	case '$':
		// Bulk String ($3\r\nfoo\r\n)
		// 'content' tells us the length (e.g. "3")
		var length int
		fmt.Sscanf(content, "%d", &length)

		if length == -1 {
			return "(nil)", nil // Handle NULL response
		}

		// Read the exact number of bytes for the data
		data := make([]byte, length)
		_, err := io.ReadFull(r, data)
		if err != nil {
			return "", err
		}

		// Read the trailing \r\n that comes after the data
		r.ReadString('\n')

		return string(data), nil

	case '*':
		// Array (*2\r\n...)
		// 'content' tells us how many items are in the array
		var count int
		fmt.Sscanf(content, "%d", &count)

		// Loop that many times and call THIS function recursively!
		var items []string
		for i := 0; i < count; i++ {
			item, err := ReadResp(r)
			if err != nil {
				return "", err
			}
			items = append(items, fmt.Sprintf("%d) %s", i+1, item))
		}

		// Join them with newlines to look like a list
		return strings.Join(items, "\n"), nil
	}

	return line, nil
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

	// wrap connection in reader
	reader := bufio.NewReader(conn)

	// start BubbleTea program
	p := tea.NewProgram(Model{
		Conn:   conn,
		Reader: reader,
	})

	if _, err := p.Run(); err != nil {
		fmt.Println("Error starting TUI: ", err)
		os.Exit(1)
	}

}
