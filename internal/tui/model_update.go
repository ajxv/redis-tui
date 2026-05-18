package tui

import (
	"bufio"
	"errors"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ajxv/redis-tui/internal/redis"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func handleRedisResult(m Model, msg RedisResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		var netError net.Error
		if msg.Error == io.EOF || errors.As(msg.Error, &netError) {
			if m.CurrentState != StateLoading {
				m.pushState(m.CurrentState)
			}
			m.CurrentState = StateLoading
			return m, connectToRedis(m)
		}

		m.Output = msg.Error.Error()
		m.ActiveTTL = ""
		m.CopyStatus = ""
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
				return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey, m.ReadTimeout)
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
			m.Browser.FieldsList.SetItems(items)
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
		} else {
			// ReadResp returns Redis error strings (e.g. WRONGTYPE) as plain string,
			// not as Go errors. Without this branch the model is stuck in StateLoading.
			if s, ok := msg.Result.(string); ok && s != "" {
				m.Output = s
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput
		}

	case OpExplore:
		if result, ok := msg.Result.(ScanResult); ok {
			if m.Browser.Cursor == "0" || m.Browser.Cursor == "" {
				m.Browser.KeyList.SetItems(result.Keys)
			} else {
				items := m.Browser.KeyList.Items()
				for _, k := range result.Keys {
					items = append(items, k)
				}
				m.Browser.KeyList.SetItems(items)
			}
			m.Browser.Cursor = result.Cursor
			m.Browser.HasMore = result.Cursor != "0"
			m.Browser.ViewingFields = false
			m.CurrentState = StateBrowser
		}

	case OpLRange:
		if resp, ok := msg.Result.([]any); ok {
			baseIndex := m.Browser.FieldOffset
			var newItems []list.Item
			for i, v := range resp {
				if s, ok := v.(string); ok {
					idx := baseIndex + i
					newItems = append(newItems, ListItem{index: idx, title: s, desc: "Index: " + strconv.Itoa(idx)})
				}
			}
			if baseIndex == 0 {
				m.Browser.FieldsList.SetItems(newItems)
			} else {
				existing := m.Browser.FieldsList.Items()
				m.Browser.FieldsList.SetItems(append(existing, newItems...))
			}
			if len(resp) >= fieldPageSize {
				m.Browser.HasMoreFields = true
				m.Browser.FieldOffset += len(resp)
			} else {
				m.Browser.HasMoreFields = false
			}
			m.SelectedOp = OpExploreList
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
		} else {
			if s, ok := msg.Result.(string); ok && s != "" {
				m.Output = s
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput
		}

	case OpSMembers:
		// SSCAN response: []any{cursor_string, []any{member1, member2, ...}}
		if resp, ok := msg.Result.([]any); ok && len(resp) == 2 {
			newCursor, _ := resp[0].(string)
			members, _ := resp[1].([]any)
			isFirstPage := m.Browser.FieldCursor == "" || m.Browser.FieldCursor == "0"
			baseIndex := 0
			if !isFirstPage {
				baseIndex = len(m.Browser.FieldsList.Items())
			}
			var newItems []list.Item
			for i, v := range members {
				if s, ok := v.(string); ok {
					newItems = append(newItems, ListItem{index: baseIndex + i, title: s, desc: "Index: " + strconv.Itoa(baseIndex+i)})
				}
			}
			if isFirstPage {
				m.Browser.FieldsList.SetItems(newItems)
			} else {
				existing := m.Browser.FieldsList.Items()
				m.Browser.FieldsList.SetItems(append(existing, newItems...))
			}
			m.Browser.FieldCursor = newCursor
			m.Browser.HasMoreFields = newCursor != "0"
			m.SelectedOp = OpExploreSet
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
		} else {
			// Covers: Redis error strings (WRONGTYPE), null arrays returned as
			// "(nil)" string, and malformed SSCAN responses with len != 2.
			if s, ok := msg.Result.(string); ok && s != "" {
				m.Output = s
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput
		}

	case OpZRange:
		if resp, ok := msg.Result.([]any); ok {
			baseOffset := m.Browser.FieldOffset
			var newItems []list.Item
			for i := 0; i+1 < len(resp); i += 2 {
				member, ok1 := resp[i].(string)
				score := "unknown"
				if s, ok2 := resp[i+1].(string); ok2 {
					score = s
				}
				if ok1 {
					newItems = append(newItems, ListItem{title: member, desc: "Score: " + score})
				}
			}
			memberCount := len(resp) / 2
			if baseOffset == 0 {
				m.Browser.FieldsList.SetItems(newItems)
			} else {
				existing := m.Browser.FieldsList.Items()
				m.Browser.FieldsList.SetItems(append(existing, newItems...))
			}
			if memberCount >= fieldPageSize {
				m.Browser.HasMoreFields = true
				m.Browser.FieldOffset += memberCount
			} else {
				m.Browser.HasMoreFields = false
			}
			m.SelectedOp = OpExploreZSet
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
		} else {
			if s, ok := msg.Result.(string); ok && s != "" {
				m.Output = s
			} else {
				m.Output = "Unexpected response"
			}
			m.CurrentState = StateOutput
		}

	case OpCheckType:
		// Reset field pagination state whenever we open a key fresh.
		m.Browser.FieldOffset = 0
		m.Browser.FieldCursor = ""
		m.Browser.HasMoreFields = false

		if str, ok := msg.Result.(string); ok {
			switch str {
			case "string":
				m.SelectedOp = OpGet
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "GET", Args: []string{m.ActiveKey}}, m.ReadTimeout))
			case "hash":
				m.SelectedOp = OpHKeys
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "HKEYS", Args: []string{m.ActiveKey}}, m.ReadTimeout))
			case "list":
				m.SelectedOp = OpLRange
				end := strconv.Itoa(fieldPageSize - 1)
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "LRANGE", Args: []string{m.ActiveKey, "0", end}}, m.ReadTimeout))
			case "set":
				m.SelectedOp = OpSMembers
				count := strconv.Itoa(fieldPageSize)
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "SSCAN", Args: []string{m.ActiveKey, "0", "COUNT", count}}, m.ReadTimeout))
			case "zset":
				m.SelectedOp = OpZRange
				end := strconv.Itoa(fieldPageSize - 1)
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "ZRANGE", Args: []string{m.ActiveKey, "0", end, "WITHSCORES"}}, m.ReadTimeout))
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

	case OpExpireAfterSet:
		m.SelectedOp = OpSet
		m.CurrentState = StateOutput
		m.ActiveTTL = "fetching..."
		return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey, m.ReadTimeout)

	case OpSet, OpLSet, OpRename, OpExpirySet, OpExport, OpImport, OpExportDB, OpImportDB:
		if str, ok := msg.Result.(string); ok {
			m.Output = str
		} else if num, ok := msg.Result.(int); ok {
			m.Output = strconv.Itoa(num)
		} else {
			m.Output = "Unexpected response"
		}
		m.CurrentState = StateOutput

		if m.SelectedOp == OpSet && m.PreservedTTL > 0 {
			ttl := m.PreservedTTL
			m.PreservedTTL = 0
			m.SelectedOp = OpExpireAfterSet
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "EXPIRE", Args: []string{m.ActiveKey, strconv.Itoa(ttl)}}, m.ReadTimeout))
		}

		if m.SelectedOp == OpExpirySet || m.SelectedOp == OpRename {
			m.ActiveTTL = "fetching..."
			return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey, m.ReadTimeout)
		}

	case OpDel:
		m.Output = "Deleted Key: " + m.ActiveKey
		m.SelectedOp = OpExplore
		pattern := m.LastPattern
		if pattern == "" {
			pattern = "*"
		}
		m.Browser.Cursor = "0"
		m.Browser.Pattern = pattern
		return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, pattern, "0"))

	case OpHDel:
		m.Output = "Deleted Hash Key: " + m.ActiveKey
		m.SelectedOp = OpHKeys
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "HKEYS", Args: []string{m.ActiveKey}}, m.ReadTimeout))

	case OpLRem:
		m.Output = "Removed element from list: " + m.ActiveKey
		m.Browser.FieldOffset = 0
		m.Browser.HasMoreFields = false
		m.SelectedOp = OpLRange
		end := strconv.Itoa(fieldPageSize - 1)
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "LRANGE", Args: []string{m.ActiveKey, "0", end}}, m.ReadTimeout))

	case OpSRem:
		m.Output = "Removed element from set: " + m.ActiveKey
		m.Browser.FieldCursor = ""
		m.Browser.HasMoreFields = false
		m.SelectedOp = OpSMembers
		count := strconv.Itoa(fieldPageSize)
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "SSCAN", Args: []string{m.ActiveKey, "0", "COUNT", count}}, m.ReadTimeout))

	case OpZRem:
		m.Output = "Removed element from sorted set: " + m.ActiveKey
		m.Browser.FieldOffset = 0
		m.Browser.HasMoreFields = false
		m.SelectedOp = OpZRange
		end := strconv.Itoa(fieldPageSize - 1)
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "ZRANGE", Args: []string{m.ActiveKey, "0", end, "WITHSCORES"}}, m.ReadTimeout))

	case OpDelete, OpHSet, OpRPush, OpLPush, OpSAdd, OpZAdd:
		if res, ok := msg.Result.(int); ok {
			m.Output = strconv.Itoa(res)
		} else {
			m.Output = "Unexpected response"
		}
		m.CurrentState = StateOutput
	}

	return m, nil
}

func handleStateOutputKey(m Model, keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "esc":
		m.Input.Input.SetValue("")
		m.Input.Hint = ""
		m.Output = ""
		m.ActiveTTL = ""
		m.CopyStatus = ""

		previousState := m.popState()

		// Handle the "Creation Flow" (Hard Reset)
		if previousState == StateInputKey || previousState == StateInputField || previousState == StateInputFilePath {
			m.StateNavigationHistory = []AppState{}
			m.CurrentState = StateMenu
			return m, nil
		}

		m.CurrentState = previousState

		// Reset Op mode so 'Enter' works correctly when returning to the list.
		switch m.SelectedOp {
		case OpLSet:
			m.SelectedOp = OpExploreList
		case OpHSet:
			m.SelectedOp = OpHKeys
		}

		return m, nil

	case "e":
		// Set and ZSet members cannot be edited in-place (SREM+SADD would be needed).
		// Info output is also read-only.
		if m.SelectedOp == OpInfo || m.SelectedOp == OpExploreSet || m.SelectedOp == OpExploreZSet {
			break
		}
		m.PreservedTTL = 0
		if m.ActiveTTL != "no expiry" && m.ActiveTTL != "fetching..." && m.ActiveTTL != "" {
			if secs, err := strconv.Atoi(strings.TrimSuffix(m.ActiveTTL, "s")); err == nil && secs > 0 {
				m.PreservedTTL = secs
			}
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
			m.CopyStatus = clipboardErrorHint()
		} else {
			m.CopyStatus = "Copied to clipboard!"
		}
		return m, func() tea.Msg {
			time.Sleep(2 * time.Second)
			return ClearCopyStatusMsg{}
		}

	case "x":
		if m.SelectedOp == OpInfo {
			break
		}
		m.SelectedOp = OpExpirySet
		m.Input.Input.SetValue("")
		m.Input.Type = InputValue
		m.Input.Hint = "TTL in seconds (enter 0 to remove expiry / PERSIST):"
		m.Input.Input.Focus()
		m.pushState(m.CurrentState)
		m.CurrentState = StateInputValue
	}

	return m, nil
}

func handleStateConfirmationKey(m Model, keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey}}, m.ReadTimeout))
		case OpHDel:
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey, m.ActiveField}}, m.ReadTimeout))
		case OpLRem:
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey, "1", m.ActiveField}}, m.ReadTimeout))
		case OpSRem, OpZRem:
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey, m.ActiveField}}, m.ReadTimeout))
		}
	}

	return m, nil
}

// clipboardErrorHint returns a platform-specific message when clipboard access fails.
func clipboardErrorHint() string {
	switch runtime.GOOS {
	case "linux":
		return "Clipboard unavailable. Install xclip, xsel, or wl-clipboard (Wayland)."
	case "darwin":
		return "Clipboard unavailable. pbcopy should be built-in — check your PATH."
	case "windows":
		return "Clipboard unavailable. Windows clipboard API should work; check antivirus."
	default:
		return "Clipboard unavailable. Check system clipboard tools."
	}
}

// handleRedisConnection processes a RedisConnectionMsg, resetting reconnect
// state on success and scheduling a backoff retry on failure.
func handleRedisConnection(m Model, msg RedisConnectionMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		if msg.Fatal {
			// Permanent failure (wrong credentials, invalid DB index) — surface
			// the error immediately and stop retrying.
			m.Output = msg.Error.Error()
			m.ActiveTTL = ""
			m.CopyStatus = ""
			m.CurrentState = StateOutput
			return m, nil
		}
		m.ReconnectAttempts++
		return m, waitForNextConnection(m.ReconnectAttempts)
	}

	conn := msg.Conn
	reader := bufio.NewReader(conn)
	m.Reader = reader
	if m.Conn != nil {
		m.Conn.Close() // close the stale fd before overwriting; safe on a broken connection
	}
	m.Conn = conn
	m.ReconnectAttempts = 0

	if m.CurrentState == StateLoading {
		m.CurrentState = m.popState()
	}
	return m, nil
}
