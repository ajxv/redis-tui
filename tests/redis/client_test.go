// Package redis_test contains black-box tests for the RESP protocol implementation.
package redis_test

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/ajxv/redis-tui/internal/redis"
)

func TestRedisCmd_ToBytes(t *testing.T) {
	cmd := redis.RedisCmd{
		Name: "SET",
		Args: []string{"mykey", "myval"},
	}

	expected := "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n"
	result := cmd.ToBytes()

	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestReadResp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
		wantErr  bool
	}{
		{name: "simple string", input: "+OK\r\n", expected: "OK"},
		{name: "error response", input: "-ERR unknown command 'foobar'\r\n", expected: "ERR unknown command 'foobar'"},
		{name: "integer", input: ":1000\r\n", expected: 1000},
		{name: "bulk string", input: "$6\r\nfoobar\r\n", expected: "foobar"},
		{name: "empty bulk string", input: "$0\r\n\r\n", expected: ""},
		{name: "null bulk string", input: "$-1\r\n", expected: "(nil)"},
		{name: "array of strings", input: "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n", expected: []any{"foo", "bar"}},
		{name: "null array", input: "*-1\r\n", expected: "(nil)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tc.input))
			result, err := redis.ReadResp(reader)

			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got none")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if arrayResult, ok := result.([]any); ok {
				expectedArray := tc.expected.([]any)
				if len(arrayResult) != len(expectedArray) {
					t.Fatalf("array length: want %d, got %d", len(expectedArray), len(arrayResult))
				}
				for i := range arrayResult {
					if arrayResult[i] != expectedArray[i] {
						t.Errorf("element[%d]: want %v, got %v", i, expectedArray[i], arrayResult[i])
					}
				}
			} else if result != tc.expected {
				t.Errorf("want %v (%T), got %v (%T)", tc.expected, tc.expected, result, result)
			}
		})
	}
}

// TestReadResp_UnknownPrefix verifies that an unrecognised RESP prefix byte
// returns an error rather than silently consuming 1 byte and returning "".
// Returning "" would desync the bufio.Reader for all subsequent reads.
func TestReadResp_UnknownPrefix(t *testing.T) {
	// '!' is not a valid RESP2 prefix byte.
	reader := bufio.NewReader(strings.NewReader("!5\r\nhello\r\n"))
	_, err := redis.ReadResp(reader)
	if err == nil {
		t.Fatal("expected error for unknown RESP prefix, got nil")
	}
}

func TestReadResp_ArrayCommand(t *testing.T) {
	input := "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n"
	reader := bufio.NewReader(bytes.NewReader([]byte(input)))
	result, err := redis.ReadResp(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	arr, ok := result.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("expected []any of length 3, got %T: %v", result, result)
	}
	if arr[0] != "SET" || arr[1] != "mykey" || arr[2] != "myval" {
		t.Errorf("unexpected array contents: %v", arr)
	}
}
