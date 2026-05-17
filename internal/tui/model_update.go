package tui

import (
	"errors"
	"io"
	"net"
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
			m.Browser.FieldsList.SetItems(items)
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
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
			var items []list.Item
			for index, key := range resp {
				if key, ok := key.(string); ok {
					items = append(items, ListItem{index: index, title: key, desc: "Index: " + strconv.Itoa(index)})
				}
			}
			m.SelectedOp = OpExploreList
			m.Browser.FieldsList.SetItems(items)
			m.Browser.ViewingFields = true
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
			m.Browser.FieldsList.SetItems(items)
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
		}

	case OpZRange:
		if resp, ok := msg.Result.([]any); ok {
			var items []list.Item
			for i := 0; i < len(resp); i += 2 {
				member, ok1 := resp[i].(string)
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
			m.Browser.FieldsList.SetItems(items)
			m.Browser.ViewingFields = true
			m.CurrentState = StateBrowser
		}

	case OpCheckType:
		if str, ok := msg.Result.(string); ok {
			switch str {
			case "string":
				m.SelectedOp = OpGet
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "GET", Args: []string{m.ActiveKey}}))
			case "hash":
				m.SelectedOp = OpHKeys
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "HKEYS", Args: []string{m.ActiveKey}}))
			case "list":
				m.SelectedOp = OpLRange
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "LRANGE", Args: []string{m.ActiveKey, "0", "-1"}}))
			case "set":
				m.SelectedOp = OpSMembers
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "SMEMBERS", Args: []string{m.ActiveKey}}))
			case "zset":
				m.SelectedOp = OpZRange
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "ZRANGE", Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"}}))
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
		return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey)

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
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "EXPIRE", Args: []string{m.ActiveKey, strconv.Itoa(ttl)}}))
		}

		if m.SelectedOp == OpExpirySet || m.SelectedOp == OpRename {
			m.ActiveTTL = "fetching..."
			return m, fetchTTL(m.Conn, m.Reader, m.ActiveKey)
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
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "HKEYS", Args: []string{m.ActiveKey}}))

	case OpLRem:
		m.Output = "Removed element from list: " + m.ActiveKey
		m.SelectedOp = OpLRange
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "LRANGE", Args: []string{m.ActiveKey, "0", "-1"}}))

	case OpSRem:
		m.Output = "Removed element from set: " + m.ActiveKey
		m.SelectedOp = OpSMembers
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "SMEMBERS", Args: []string{m.ActiveKey}}))

	case OpZRem:
		m.Output = "Removed element from sorted set: " + m.ActiveKey
		m.SelectedOp = OpZRange
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: "ZRANGE", Args: []string{m.ActiveKey, "0", "-1", "WITHSCORES"}}))

	case OpDelete, OpHSet, OpRPush, OpSAdd, OpZAdd:
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
		if m.SelectedOp == OpInfo {
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
			m.CopyStatus = "Clipboard unavailable (xclip/xsel missing?)"
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
			m.CurrentState = StateLoading
			return m, sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey}})
		case OpHDel:
			m.CurrentState = StateLoading
			return m, sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey, m.ActiveField}})
		case OpLRem:
			m.CurrentState = StateLoading
			return m, sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey, "1", m.ActiveField}})
		case OpSRem, OpZRem:
			m.CurrentState = StateLoading
			return m, sendRedisCmd(m.Conn, m.Reader, redis.RedisCmd{Name: m.SelectedOp.String(), Args: []string{m.ActiveKey, m.ActiveField}})
		}
	}

	return m, nil
}
