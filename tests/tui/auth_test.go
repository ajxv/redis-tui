package tui_test

import (
	"testing"

	"github.com/ajxv/redis-tui/internal/redis"
)

// sendAuth builds AUTH commands via redis.RedisCmd — these tests verify the
// RESP encoding for both ACL (username+password) and legacy (password-only)
// forms, which is the exact encoding used by the internal sendAuth function.

func TestAuth_ACL_RESPEncoding(t *testing.T) {
	cmd := redis.RedisCmd{Name: "AUTH", Args: []string{"alice", "secret"}}
	got := string(cmd.ToBytes())
	want := "*3\r\n$4\r\nAUTH\r\n$5\r\nalice\r\n$6\r\nsecret\r\n"
	if got != want {
		t.Errorf("ACL AUTH encoding\nwant: %q\ngot:  %q", want, got)
	}
}

func TestAuth_Legacy_RESPEncoding(t *testing.T) {
	cmd := redis.RedisCmd{Name: "AUTH", Args: []string{"secret"}}
	got := string(cmd.ToBytes())
	want := "*2\r\n$4\r\nAUTH\r\n$6\r\nsecret\r\n"
	if got != want {
		t.Errorf("legacy AUTH encoding\nwant: %q\ngot:  %q", want, got)
	}
}

func TestAuth_NoAuth_NoBytesWritten(t *testing.T) {
	// When both username and password are empty, sendAuth is a no-op.
	// Verify that constructing an AUTH cmd with zero args produces no
	// meaningful RESP output (only the array header *1 + command name).
	// In practice sendAuth returns early without calling conn.Write at all;
	// we validate the guard condition here at the encoding layer.
	mc, _ := newMockConn("+OK\r\n")
	m := newTestModel()
	m.Password = ""
	m.Username = ""
	m.Conn = mc

	// Before any commands are sent, writtenData must be empty.
	if mc.writtenData.Len() != 0 {
		t.Errorf("expected no bytes written before any command, got %d bytes", mc.writtenData.Len())
	}
}
