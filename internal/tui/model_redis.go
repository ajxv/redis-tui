package tui

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ajxv/redis-tui/internal/redis"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// A command that waits for 2 seconds, then returns the TickMsg
func waitForNextConnection() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)

		return TickMsg{}
	}
}

func connectToRedis(address string, password string, db int) tea.Cmd {
	return func() tea.Msg {
		// dial the address
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return RedisConnectionMsg{
				Error: err,
			}
		}

		reader := bufio.NewReader(conn)

		if password != "" {
			cmd := redis.RedisCmd{
				Name: "AUTH",
				Args: []string{password},
			}

			_, err = conn.Write(cmd.ToBytes())
			if err != nil {
				return RedisConnectionMsg{
					Error: err,
				}
			}
			_, err = redis.ReadResp(reader)
			if err != nil {
				return RedisConnectionMsg{
					Error: err,
				}
			}
		}

		// select db
		cmd := redis.RedisCmd{
			Name: "SELECT",
			Args: []string{strconv.Itoa(db)},
		}

		_, err = conn.Write(cmd.ToBytes())
		if err != nil {
			return RedisConnectionMsg{
				Error: err,
			}
		}
		_, err = redis.ReadResp(reader)
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

func scanRedisKeys(conn net.Conn, reader *bufio.Reader, pattern string, cursor string) tea.Cmd {
	return func() tea.Msg {
		filter := pattern
		var keys []list.Item

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
				var rawKeys []string
				for _, str := range slice {
					if s, ok := str.(string); ok {
						rawKeys = append(rawKeys, s)
					}
				}

				if len(rawKeys) > 0 {
					// Pipeline TYPE commands
					for _, k := range rawKeys {
						cmd := redis.RedisCmd{Name: "TYPE", Args: []string{k}}
						conn.Write(cmd.ToBytes())
					}

					// Read piplined responses
					for _, k := range rawKeys {
						desc := "key"
						typeResp, err := redis.ReadResp(reader)
						if err == nil {
							if typeStr, ok := typeResp.(string); ok {
								desc = typeStr
							}
						}
						keys = append(keys, ListItem{title: k, desc: desc})
					}
				}
			}
		}

		return RedisResultMsg{
			Result: ScanResult{Cursor: cursor, Keys: keys},
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
