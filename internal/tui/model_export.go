package tui

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
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

// FieldExport is the self-describing JSON form of a single hash field, list
// element, set member, or sorted-set member — exported/imported by value
// (DUMP/RESTORE only work on whole keys).
type FieldExport struct {
	Key   string `json:"key"`
	Type  string `json:"type"`            // hash | list | set | zset
	Field string `json:"field,omitempty"` // hash field name (or list index)
	Value string `json:"value"`           // field value / member
	Score string `json:"score,omitempty"` // sorted-set score
}

// readResp writes a command and reads one response under the default deadline.
func readResp(conn net.Conn, reader *bufio.Reader, cmd redis.RedisCmd) (any, error) {
	if _, err := conn.Write(cmd.ToBytes()); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	resp, err := redis.ReadResp(reader)
	_ = conn.SetReadDeadline(time.Time{})
	return resp, err
}

// ExportField writes a single field/member to a self-describing JSON file.
func ExportField(conn net.Conn, reader *bufio.Reader, key, keyType, field string, index int, filePath string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}
		resolved, err := resolveFilePath(filePath, true, fieldExportFilename(key, keyType, field, index))
		if err != nil {
			return RedisResultMsg{Error: err}
		}

		fe := FieldExport{Key: key, Type: keyType, Field: field}
		switch keyType {
		case "hash":
			resp, err := readResp(conn, reader, redis.RedisCmd{Name: "HGET", Args: []string{key, field}})
			if err != nil {
				return RedisResultMsg{Error: err}
			}
			s, ok := resp.(string)
			if !ok || s == "(nil)" {
				return RedisResultMsg{Error: fmt.Errorf("field %q not found in %q", field, key)}
			}
			fe.Value = s
		case "list":
			resp, err := readResp(conn, reader, redis.RedisCmd{Name: "LINDEX", Args: []string{key, strconv.Itoa(index)}})
			if err != nil {
				return RedisResultMsg{Error: err}
			}
			s, _ := resp.(string)
			fe.Field = strconv.Itoa(index)
			fe.Value = s
		case "set":
			fe.Field = ""
			fe.Value = field
		case "zset":
			resp, err := readResp(conn, reader, redis.RedisCmd{Name: "ZSCORE", Args: []string{key, field}})
			if err != nil {
				return RedisResultMsg{Error: err}
			}
			fe.Field = ""
			fe.Value = field
			if s, ok := resp.(string); ok {
				fe.Score = s
			}
		default:
			return RedisResultMsg{Error: fmt.Errorf("unsupported type %q for field export", keyType)}
		}

		data, err := json.MarshalIndent(fe, "", "  ")
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to marshal JSON: %v", err)}
		}
		if err := os.WriteFile(resolved, data, 0600); err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to write file: %v", err)}
		}
		label := field
		if label == "" {
			label = fe.Value
		}
		return RedisResultMsg{Result: fmt.Sprintf("Exported %s entry '%s' to %s", keyType, label, resolved)}
	}
}

// ImportField restores a single field/member from a FieldExport JSON file into
// targetKey — the key currently being browsed — using its field/value/score.
// The file's type must match targetType (the file's own key is ignored).
func ImportField(conn net.Conn, reader *bufio.Reader, filePath, targetKey, targetType string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}
		resolved, err := resolveFilePath(filePath, false, "")
		if err != nil {
			return RedisResultMsg{Error: err}
		}
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to read file: %v", err)}
		}
		var fe FieldExport
		if err := json.Unmarshal(raw, &fe); err != nil {
			return RedisResultMsg{Error: fmt.Errorf("failed to parse JSON: %v", err)}
		}
		if fe.Type != "" && targetType != "" && fe.Type != targetType {
			return RedisResultMsg{Error: fmt.Errorf("file holds a %s entry, but %q is a %s", fe.Type, targetKey, targetType)}
		}
		typ := targetType
		if typ == "" {
			typ = fe.Type
		}

		var cmd redis.RedisCmd
		switch typ {
		case "hash":
			if fe.Field == "" {
				return RedisResultMsg{Error: fmt.Errorf("hash import requires a 'field' in the file")}
			}
			cmd = redis.RedisCmd{Name: "HSET", Args: []string{targetKey, fe.Field, fe.Value}}
		case "list":
			cmd = redis.RedisCmd{Name: "RPUSH", Args: []string{targetKey, fe.Value}}
		case "set":
			cmd = redis.RedisCmd{Name: "SADD", Args: []string{targetKey, fe.Value}}
		case "zset":
			score := fe.Score
			if score == "" {
				score = "0"
			}
			cmd = redis.RedisCmd{Name: "ZADD", Args: []string{targetKey, score, fe.Value}}
		default:
			return RedisResultMsg{Error: fmt.Errorf("unsupported type %q in import file", fe.Type)}
		}

		if _, err := readResp(conn, reader, cmd); err != nil {
			return RedisResultMsg{Error: err}
		}
		return RedisResultMsg{Result: fmt.Sprintf("Imported %s entry into '%s' from %s", typ, targetKey, resolved)}
	}
}

// resolveFilePath cleans path, expanding a leading "~/", and creates parent
// directories when createDirs is set (export destinations). When defaultName
// is non-empty and path names a directory rather than a file — a trailing
// separator, ".", "~", or an existing directory on disk — defaultName is
// appended, so submitting just a folder still produces a usable file instead
// of failing to write (or, for imports, failing to read) a directory.
func resolveFilePath(path string, createDirs bool, defaultName string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine user home directory: %v", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	if defaultName != "" && looksLikeDir(path) {
		path = filepath.Join(path, defaultName)
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

// looksLikeDir reports whether path names a directory rather than a file:
// blank, ".", "~", a trailing path separator, or an existing directory.
func looksLikeDir(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "." || trimmed == "~" {
		return true
	}
	if strings.HasSuffix(trimmed, "/") || strings.HasSuffix(trimmed, string(os.PathSeparator)) {
		return true
	}
	info, err := os.Stat(trimmed)
	return err == nil && info.IsDir()
}

// fetchKeyExportData fetches a single key's DUMP payload and PTTL from Redis
// and returns an ExportData ready for JSON serialisation.
func fetchKeyExportData(conn net.Conn, reader *bufio.Reader, key string) (ExportData, error) {
	if _, err := conn.Write(redis.RedisCmd{Name: "DUMP", Args: []string{key}}.ToBytes()); err != nil {
		return ExportData{}, fmt.Errorf("DUMP write failed for %q: %w", key, err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	dumpResp, err := redis.ReadResp(reader)
	_ = conn.SetReadDeadline(time.Time{})
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
	_ = conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
	pttlResp, err := redis.ReadResp(reader)
	_ = conn.SetReadDeadline(time.Time{})
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

		resolvedPath, err := resolveFilePath(filePath, true, sanitizeFilename(key)+".dump")
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

		resolvedPath, err := resolveFilePath(filePath, false, "")
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
			_ = conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
			resp, err := redis.ReadResp(reader)
			_ = conn.SetReadDeadline(time.Time{})
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
func ExportFullDB(conn net.Conn, reader *bufio.Reader, db int, filePath string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return RedisResultMsg{Error: fmt.Errorf("no connection to Redis")}
		}

		resolvedPath, err := resolveFilePath(filePath, true, fmt.Sprintf("redis-db%d.json", db))
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
				_ = f.Close()
				_ = os.Remove(tmpPath)
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
			_ = conn.SetReadDeadline(time.Now().Add(defaultReadTimeout))
			response, err := redis.ReadResp(reader)
			_ = conn.SetReadDeadline(time.Time{})
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
		_ = f.Close()
		if err := os.Rename(tmpPath, resolvedPath); err != nil {
			_ = os.Remove(tmpPath)
			return RedisResultMsg{Error: fmt.Errorf("failed to finalise export: %v", err)}
		}
		success = true

		return RedisResultMsg{Result: fmt.Sprintf("Successfully exported %d keys to %s", exportedCount, resolvedPath)}
	}
}
