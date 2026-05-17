package tui

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajxv/redis-tui/internal/redis"
	tea "github.com/charmbracelet/bubbletea"
)

type ExportData struct {
	Key   string `json:"key"`
	TTL   int    `json:"ttl"`
	Value string `json:"value"` // base64 encoded DUMP payload
}

func resolveFilePath(path string, createDirs bool) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine user home directory: %v", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	path = filepath.Clean(path)

	if createDirs {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("could not create parent directories: %v", err)
		}
	}

	return path, nil
}

func ExportSingleKey(conn net.Conn, reader *bufio.Reader, key string, filePath string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		resolvedPath, err := resolveFilePath(filePath, true)
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		// Issue DUMP command
		cmd := redis.RedisCmd{Name: "DUMP", Args: []string{key}}
		_, err = conn.Write(cmd.ToBytes())
		if err != nil {
			return RedisResultMsg{Error: err}
		}
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		resp, err := redis.ReadResp(reader)
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		dumpPayload, ok := resp.(string)
		if !ok || dumpPayload == "" {
			return RedisResultMsg{Error: fmt.Errorf("key does not exist or payload invalid")}
		}

		// Issue PTTL command
		pttlCmd := redis.RedisCmd{Name: "PTTL", Args: []string{key}}
		if _, err = conn.Write(pttlCmd.ToBytes()); err != nil {
			return RedisResultMsg{Error: fmt.Errorf("PTTL write failed: %v", err)}
		}
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		pttlResp, err := redis.ReadResp(reader)
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("PTTL read failed: %v", err)}
		}
		ttl := -1
		if p, ok := pttlResp.(int); ok {
			ttl = p
		}

		data := []ExportData{
			{
				Key:   key,
				TTL:   ttl,
				Value: base64.StdEncoding.EncodeToString([]byte(dumpPayload)),
			},
		}

		fileData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to marshal JSON: %v", err)}
		}

		err = os.WriteFile(resolvedPath, fileData, 0600)
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to write file: %v", err)}
		}

		return RedisResultMsg{Result: fmt.Sprintf("Successfully exported key '%s' to %s", key, resolvedPath)}
	}
}

func ImportKeys(conn net.Conn, reader *bufio.Reader, filePath string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		resolvedPath, err := resolveFilePath(filePath, false)
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		fileData, err := os.ReadFile(resolvedPath)
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to read file: %v", err)}
		}

		var data []ExportData
		err = json.Unmarshal(fileData, &data)
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to parse JSON: %v", err)}
		}

		importedCount := 0
		for _, item := range data {
			decodedDump, err := base64.StdEncoding.DecodeString(item.Value)
			if err != nil {
				continue // Skip invalid items
			}

			// In RESTORE, TTL of 0 means no expiry.
			restoreTTL := item.TTL
			if restoreTTL < 0 {
				restoreTTL = 0
			}

			cmd := redis.RedisCmd{
				Name: "RESTORE",
				Args: []string{
					item.Key,
					fmt.Sprintf("%d", restoreTTL),
					string(decodedDump),
					"REPLACE",
				},
			}
			conn.Write(cmd.ToBytes())
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			resp, err := redis.ReadResp(reader)
			conn.SetReadDeadline(time.Time{})
			if err == nil {
				if strResp, ok := resp.(string); ok && strResp == "OK" {
					importedCount++
				}
			}
		}

		return RedisResultMsg{Result: fmt.Sprintf("Successfully imported %d keys from %s", importedCount, filePath)}
	}
}

func ExportFullDB(conn net.Conn, reader *bufio.Reader, filePath string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		resolvedPath, err := resolveFilePath(filePath, true)
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		var data []ExportData
		cursor := "0"

		for {
			cmd := redis.RedisCmd{
				Name: "SCAN",
				Args: []string{cursor},
			}
			conn.Write(cmd.ToBytes())
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			response, err := redis.ReadResp(reader)
			conn.SetReadDeadline(time.Time{})
			if err != nil {
				return RedisResultMsg{Error: err}
			}

			var keys []string
			if resp, ok := response.([]any); ok {
				if c, ok := resp[0].(string); ok {
					cursor = c
				}
				if slice, ok := resp[1].([]any); ok {
					for _, str := range slice {
						if s, ok := str.(string); ok {
							keys = append(keys, s)
						}
					}
				}
			}

			for _, key := range keys {
				// DUMP
				conn.Write(redis.RedisCmd{Name: "DUMP", Args: []string{key}}.ToBytes())
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				dumpResp, err := redis.ReadResp(reader)
				conn.SetReadDeadline(time.Time{})
				if err != nil {
					continue
				}
				dumpPayload, ok := dumpResp.(string)
				if !ok || dumpPayload == "" {
					continue
				}

				// PTTL
				if _, err = conn.Write(redis.RedisCmd{Name: "PTTL", Args: []string{key}}.ToBytes()); err != nil {
					continue
				}
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				pttlResp, err := redis.ReadResp(reader)
				conn.SetReadDeadline(time.Time{})
				if err != nil {
					continue
				}
				ttl := -1
				if p, ok := pttlResp.(int); ok {
					ttl = p
				}

				data = append(data, ExportData{
					Key:   key,
					TTL:   ttl,
					Value: base64.StdEncoding.EncodeToString([]byte(dumpPayload)),
				})
			}

			if cursor == "0" {
				break
			}
		}

		fileData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to marshal JSON: %v", err)}
		}

		err = os.WriteFile(resolvedPath, fileData, 0600)
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to write file: %v", err)}
		}

		return RedisResultMsg{Result: fmt.Sprintf("Successfully exported %d keys to %s", len(data), resolvedPath)}
	}
}
