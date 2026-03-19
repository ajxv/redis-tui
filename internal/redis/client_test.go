package redis

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestRedisCmd_ToBytes(t *testing.T) {
	cmd := RedisCmd{
		Name: "SET",
		Args: []string{"mykey", "myval"},
	}

	expected := "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n"
	result := cmd.ToBytes()

	if string(result) != expected {
		t.Errorf("Expected ToBytes() to be %q, got %q", expected, string(result))
	}
}

func TestReadResp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected any
		err      bool
	}{
		{
			name:     "Simple String",
			input:    "+OK\r\n",
			expected: "OK",
		},
		{
			name:     "Error",
			input:    "-ERR unknown command 'foobar'\r\n",
			expected: "ERR unknown command 'foobar'",
		},
		{
			name:     "Integer",
			input:    ":1000\r\n",
			expected: 1000,
		},
		{
			name:     "Bulk String",
			input:    "$6\r\nfoobar\r\n",
			expected: "foobar",
		},
		{
			name:     "Empty Bulk String",
			input:    "$0\r\n\r\n",
			expected: "",
		},
		{
			name:     "Null Bulk String",
			input:    "$-1\r\n",
			expected: "(nil)",
		},
		{
			name:     "Array of Strings",
			input:    "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
			expected: []any{"foo", "bar"},
		},
		{
			name:     "Null Array",
			input:    "*-1\r\n",
			expected: "(nil)", // Using the same parsing logic for $-1 and *-1
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tc.input))
			result, err := ReadResp(reader)

			if tc.err && err == nil {
				t.Errorf("Expected an error for test %q, but got none", tc.name)
				return
			}
			if !tc.err && err != nil {
				t.Errorf("Did not expect an error for test %q, but got: %v", tc.name, err)
				return
			}

			// Validate results
			if arrayResult, ok := result.([]any); ok {
				expectedArray := tc.expected.([]any)
				if len(arrayResult) != len(expectedArray) {
					t.Errorf("Expected array length %d, got %d", len(expectedArray), len(arrayResult))
				}
				for i := range arrayResult {
					if arrayResult[i] != expectedArray[i] {
						t.Errorf("Expected array element %d to be %v, got %v", i, expectedArray[i], arrayResult[i])
					}
				}
			} else {
				if result != tc.expected {
					t.Errorf("Expected result to be %v (type %T), got %v (type %T)", tc.expected, tc.expected, result, result)
				}
			}
		})
	}
}

func TestReadResp_Partial(t *testing.T) {
	// A test specifically to ensure it handles multiple reads or buffers correctly if needed,
	// though our scanner uses strings.NewReader which is immediate.
	input := "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n"
	reader := bufio.NewReader(bytes.NewReader([]byte(input)))
	result, err := ReadResp(reader)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	resArray, ok := result.([]any)
	if !ok || len(resArray) != 3 {
		t.Fatalf("Expected array of length 3, got: %v", result)
	}

	if resArray[0] != "SET" || resArray[1] != "mykey" || resArray[2] != "myval" {
		t.Errorf("Unexpected array contents: %v", resArray)
	}
}
