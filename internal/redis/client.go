package redis

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type RedisCmd struct {
	Name string
	Args []string
}

func (cmd RedisCmd) ToBytes() []byte {
	// use a bytes buffer to avoid conversion later
	var buf bytes.Buffer

	// 1. header
	fmt.Fprintf(&buf, "*%d\r\n", len(cmd.Args)+1)

	// 2. command name
	fmt.Fprintf(&buf, "$%d\r\n%s\r\n", len(cmd.Name), cmd.Name)

	// 3. arguments
	for _, arg := range cmd.Args {
		fmt.Fprintf(&buf, "$%d\r\n%s\r\n", len(arg), arg)
	}

	return buf.Bytes()
}

func ReadResp(reader *bufio.Reader) (any, error) {
	// 1. Read the prefix byte
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	switch prefix {
	case '+', '-':
		// Simple String or Error: Read until newline
		msg, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(msg), nil // Clean up the result

	case '$':
		// Bulk String: Read length first
		lengthStr, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		// Convert length to integer
		lengthNum, err := strconv.Atoi(strings.TrimSpace(lengthStr))
		if err != nil {
			return "", err
		}

		if lengthNum == -1 {
			return "(nil)", nil // Handle NULL response
		}

		// Read the exact data bytes
		data := make([]byte, lengthNum)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			return "", err
		}

		// read the trailing clrf
		_, err = reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return string(data), nil
	case '*':
		// Bulk String: Read length first
		lengthStr, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		// Convert length to integer
		lengthNum, err := strconv.Atoi(strings.TrimSpace(lengthStr))
		if err != nil {
			return "", err
		}

		if lengthNum == -1 {
			return "(nil)", nil // Handle NULL response
		}

		var items []any

		for range lengthNum {
			item, err := ReadResp(reader)
			if err != nil {
				return "", err
			}

			items = append(items, item)

		}
		return items, nil

	}

	return "", nil
}
