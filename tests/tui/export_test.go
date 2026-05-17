package tui_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ajxv/redis-tui/internal/tui"
)

func writeTempJSON(t *testing.T, data []tui.ExportData) string {
	t.Helper()
	fileData, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp("", "redis_tui_import_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Write(fileData)
	return f.Name()
}

// TestImportKeys_OKResponse checks that a valid RESTORE +OK response
// increments the imported key count.
func TestImportKeys_OKResponse(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	path := writeTempJSON(t, []tui.ExportData{{Key: "mykey", TTL: -1, Value: encoded}})
	defer os.Remove(path)

	conn, reader := newMockConn("+OK\r\n")

	msg := tui.ImportKeys(conn, reader, path)()
	result, ok := msg.(tui.RedisResultMsg)
	if !ok {
		t.Fatalf("expected RedisResultMsg, got %T", msg)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(fmt.Sprintf("%v", result.Result), "1") {
		t.Errorf("want result to mention 1 imported key, got: %v", result.Result)
	}
}

// TestImportKeys_NonOKResponse checks that a non-OK RESTORE response does
// not count as a successfully imported key.
func TestImportKeys_NonOKResponse(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	path := writeTempJSON(t, []tui.ExportData{{Key: "mykey", TTL: -1, Value: encoded}})
	defer os.Remove(path)

	conn, reader := newMockConn("-ERR BUSYKEY Target key name already exists.\r\n")

	msg := tui.ImportKeys(conn, reader, path)()
	result, ok := msg.(tui.RedisResultMsg)
	if !ok {
		t.Fatalf("expected RedisResultMsg, got %T", msg)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.Contains(fmt.Sprintf("%v", result.Result), "1 keys") {
		t.Errorf("want 0 imported keys, got: %v", result.Result)
	}
}

// TestExportSingleKey_FilePermissions verifies the output JSON is written
// with restrictive permissions (0600).
func TestExportSingleKey_FilePermissions(t *testing.T) {
	conn, reader := newMockConn("$5\r\nhello\r\n:-1\r\n")
	outPath := t.TempDir() + "/export.json"

	msg := tui.ExportSingleKey(conn, reader, "mykey", outPath)()
	result, ok := msg.(tui.RedisResultMsg)
	if !ok {
		t.Fatalf("expected RedisResultMsg, got %T", msg)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat output file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions: want 0600, got %04o", perm)
	}
}

// TestExportSingleKey_RoundTrip exports a key then verifies the payload,
// key name, and TTL survive the base64/JSON round-trip intact.
func TestExportSingleKey_RoundTrip(t *testing.T) {
	payload := "round-trip-value"
	dumpLen := fmt.Sprintf("%d", len(payload))
	mockResponses := "$" + dumpLen + "\r\n" + payload + "\r\n" + ":3600\r\n"

	conn, reader := newMockConn(mockResponses)
	outPath := t.TempDir() + "/rt.json"

	msg := tui.ExportSingleKey(conn, reader, "rtkey", outPath)()
	if r, ok := msg.(tui.RedisResultMsg); !ok || r.Error != nil {
		t.Fatalf("export failed: %v", msg)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	var records []tui.ExportData
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("unmarshal exported JSON: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	if records[0].Key != "rtkey" {
		t.Errorf("key: want %q, got %q", "rtkey", records[0].Key)
	}
	if records[0].TTL != 3600 {
		t.Errorf("TTL: want 3600, got %d", records[0].TTL)
	}
	decoded, err := base64.StdEncoding.DecodeString(records[0].Value)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != payload {
		t.Errorf("payload: want %q, got %q", payload, string(decoded))
	}
}
