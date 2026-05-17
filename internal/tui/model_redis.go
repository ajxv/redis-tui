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
	tea "github.com/charmbracelet/bubbletea"
)

const defaultDialTimeout = 5 * time.Second
const defaultReadTimeout = 10 * time.Second

// BackoffDuration returns an exponentially increasing wait time capped at 30s.
// Attempt is clamped to 7 to prevent integer overflow (1<<7 * 200ms = 25.6s).
func BackoffDuration(attempt int) time.Duration {
	const maxWait = 30 * time.Second
	if attempt > 7 {
		attempt = 7
	}
	d := time.Duration(1<<attempt) * 200 * time.Millisecond
	if d > maxWait {
		d = maxWait
	}
	return d
}

// waitForNextConnection waits the backoff duration then signals a reconnect.
func waitForNextConnection(attempt int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(BackoffDuration(attempt))
		return TickMsg{}
	}
}

// connectToRedis dials Redis using the connection settings stored in m,
// performs TLS wrapping when configured, authenticates, and selects the DB.
func connectToRedis(m Model) tea.Cmd {
	return func() tea.Msg {
		dialTimeout := m.DialTimeout
		if dialTimeout == 0 {
			dialTimeout = defaultDialTimeout
		}

		// 1. Dial raw TCP
		rawConn, err := net.DialTimeout("tcp", m.RedisAddress, dialTimeout)
		if err != nil {
			return RedisConnectionMsg{Error: err}
		}

		// 2. Wrap with TLS when configured
		var conn net.Conn = rawConn
		if m.TLSConfig != nil {
			tlsConn := tls.Client(rawConn, m.TLSConfig)
			if err := tlsConn.Handshake(); err != nil {
				rawConn.Close()
				return RedisConnectionMsg{Error: fmt.Errorf("TLS handshake failed: %w", err)}
			}
			conn = tlsConn
		}

		reader := bufio.NewReader(conn)

		// 3. AUTH — ACL format (username + password) or legacy (password only)
		if err := sendAuth(conn, reader, m.Username, m.Password); err != nil {
			conn.Close()
			return RedisConnectionMsg{Error: err}
		}

		// 4. SELECT database
		cmd := redis.RedisCmd{
			Name: "SELECT",
			Args: []string{strconv.Itoa(m.DB)},
		}
		_, err = conn.Write(cmd.ToBytes())
		if err != nil {
			conn.Close()
			return RedisConnectionMsg{Error: err}
		}
		_, err = redis.ReadResp(reader)
		if err != nil {
			conn.Close()
			return RedisConnectionMsg{Error: err}
		}

		return RedisConnectionMsg{Conn: conn}
	}
}

// sendAuth sends the appropriate AUTH command based on whether a username is
// provided. Sends nothing when both username and password are empty.
func sendAuth(conn net.Conn, reader *bufio.Reader, username, password string) error {
	var cmd redis.RedisCmd
	if username != "" && password != "" {
		cmd = redis.RedisCmd{Name: "AUTH", Args: []string{username, password}}
	} else if password != "" {
		cmd = redis.RedisCmd{Name: "AUTH", Args: []string{password}}
	} else {
		return nil
	}

	if _, err := conn.Write(cmd.ToBytes()); err != nil {
		return fmt.Errorf("AUTH write failed: %w", err)
	}
	resp, err := redis.ReadResp(reader)
	if err != nil {
		return fmt.Errorf("AUTH read failed: %w", err)
	}
	// ReadResp returns plain strings for both +OK and -ERR... responses.
	if str, ok := resp.(string); ok && str != "OK" {
		return fmt.Errorf("AUTH rejected: %s", str)
	}
	return nil
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
						if _, err := conn.Write(cmd.ToBytes()); err != nil {
							return RedisResultMsg{Error: err}
						}
					}

					// Read pipelined responses
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

func sendRedisCmd(conn net.Conn, reader *bufio.Reader, cmd redis.RedisCmd, readTimeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		if readTimeout == 0 {
			readTimeout = defaultReadTimeout
		}

		_, err := conn.Write(cmd.ToBytes())
		if err != nil {
			return RedisResultMsg{Error: err}
		}
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		response, err := redis.ReadResp(reader)
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		return RedisResultMsg{Result: response}
	}
}

func fetchTTL(conn net.Conn, reader *bufio.Reader, key string, readTimeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisTTLResultMsg{TTL: -2}
		}
		if readTimeout == 0 {
			readTimeout = defaultReadTimeout
		}
		cmd := redis.RedisCmd{
			Name: "TTL",
			Args: []string{key},
		}
		_, err := conn.Write(cmd.ToBytes())
		if err != nil {
			return RedisTTLResultMsg{TTL: -2}
		}
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		response, err := redis.ReadResp(reader)
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			return RedisTTLResultMsg{TTL: -2}
		}
		if ttl, ok := response.(int); ok {
			return RedisTTLResultMsg{TTL: ttl}
		}
		return RedisTTLResultMsg{TTL: -2}
	}
}
