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

// fetchKeyExportData fetches a single key's DUMP payload and PTTL from Redis
// and returns an ExportData ready for JSON serialisation.
func fetchKeyExportData(conn net.Conn, reader *bufio.Reader, key string) (ExportData, error) {
	if _, err := conn.Write(redis.RedisCmd{Name: "DUMP", Args: []string{key}}.ToBytes()); err != nil {
		return ExportData{}, fmt.Errorf("DUMP write failed for %q: %w", key, err)
	}
	conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	dumpResp, err := redis.ReadResp(reader)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return ExportData{}, err
	}
	dumpPayload, ok := dumpResp.(string)
	// ReadResp returns the literal string "(nil)" for Redis null bulk strings ($-1).
	// Treat it the same as a missing key rather than exporting corrupt data.
	if !ok || dumpPayload == "" || dumpPayload == "(nil)" {
		return ExportData{}, fmt.Errorf("key %q does not exist or has no payload", key)
	}

	if _, err := conn.Write(redis.RedisCmd{Name: "PTTL", Args: []string{key}}.ToBytes()); err != nil {
		return ExportData{}, fmt.Errorf("PTTL write failed for %q: %w", key, err)
	}
	conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	pttlResp, err := redis.ReadResp(reader)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return ExportData{}, fmt.Errorf("PTTL failed for %q: %w", key, err)
	}
	ttl := -1
	if p, ok := pttlResp.(int); ok {
		ttl = p
	}

	return ExportData{
		Key:   key,
		TTL:   ttl,
		Value: base64.StdEncoding.EncodeToString([]byte(dumpPayload)),
	}, nil
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

		data, err := fetchKeyExportData(conn, reader, key)
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		fileData, err := json.MarshalIndent([]ExportData{data}, "", "  ")
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to marshal JSON: %v", err)}
		}

		if err := os.WriteFile(resolvedPath, fileData, 0600); err != nil {
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
		if err := json.Unmarshal(fileData, &data); err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to parse JSON: %v", err)}
		}

		importedCount := 0
		for _, item := range data {
			decodedDump, err := base64.StdEncoding.DecodeString(item.Value)
			if err != nil {
				continue
			}

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
			if _, err := conn.Write(cmd.ToBytes()); err != nil {
				return RedisResultMsg{Error: fmt.Errorf("RESTORE write failed for %q: %w", item.Key, err)}
			}
			conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
			resp, err := redis.ReadResp(reader)
			conn.SetReadDeadline(time.Time{})
			if err == nil {
				if strResp, ok := resp.(string); ok && strResp == "OK" {
					importedCount++
				}
			}
		}

		return RedisResultMsg{Result: fmt.Sprintf("Successfully imported %d keys from %s", importedCount, resolvedPath)}
	}
}

// ExportFullDB exports every key in the current Redis database to a JSON file.
// Keys are streamed via SCAN and written incrementally — memory usage is
// proportional to one SCAN batch, not the entire dataset.
//
// The file is written to a temporary path first and atomically renamed on
// success, so a partial or interrupted export never corrupts a previous export.
func ExportFullDB(conn net.Conn, reader *bufio.Reader, filePath string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		resolvedPath, err := resolveFilePath(filePath, true)
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		tmpPath := resolvedPath + ".tmp"

		f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to create export file: %v", err)}
		}

		// Ensure the .tmp file is cleaned up on any error path.
		// f.Close() is safe to call multiple times (no-op after first call).
		success := false
		defer func() {
			if !success {
				f.Close()
				os.Remove(tmpPath)
			}
		}()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")

		if _, err := f.Write([]byte("[\n")); err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to write export file: %v", err)}
		}

		first := true
		exportedCount := 0
		cursor := "0"

		for {
			if _, err := conn.Write(redis.RedisCmd{Name: "SCAN", Args: []string{cursor}}.ToBytes()); err != nil {
				return RedisResultMsg{Error: err}
			}
			conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
			response, err := redis.ReadResp(reader)
			conn.SetReadDeadline(time.Time{})
			if err != nil {
				return RedisResultMsg{Error: err}
			}

			var keys []string
			if resp, ok := response.([]any); ok && len(resp) >= 2 {
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
				data, err := fetchKeyExportData(conn, reader, key)
				if err != nil {
					continue // skip unreadable keys; don't abort the whole export
				}

				if !first {
					if _, err := f.Write([]byte(",\n")); err != nil {
						return RedisResultMsg{Error: fmt.Errorf("write error: %v", err)}
					}
				}
				if err := enc.Encode(data); err != nil {
					return RedisResultMsg{Error: fmt.Errorf("JSON encode error: %v", err)}
				}
				first = false
				exportedCount++
			}

			if cursor == "0" {
				break
			}
		}

		if _, err := f.Write([]byte("]\n")); err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to finalise export file: %v", err)}
		}

		// Close before rename so Windows can move the file (open handles block
		// os.Rename on Windows). On Linux/macOS this is a no-op in terms of safety.
		f.Close()
		if err := os.Rename(tmpPath, resolvedPath); err != nil {
			os.Remove(tmpPath)
			return RedisResultMsg{Error: fmt.Errorf("failed to finalise export: %v", err)}
		}
		success = true

		return RedisResultMsg{Result: fmt.Sprintf("Successfully exported %d keys to %s", exportedCount, resolvedPath)}
	}
}
