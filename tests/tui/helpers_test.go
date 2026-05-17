// Package tui_test contains black-box tests for the TUI state machine.
// All tests drive the public Model.Update / Model.View API only.
package tui_test

import (
	"bufio"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajxv/redis-tui/internal/tui"
)

// newTestModel returns a zero-state model with all Bubble Tea components
// initialised so Update / View never panic on nil fields.
func newTestModel() tui.Model {
	return tui.Model{
		CurrentState: tui.StateMenu,
		MenuList:     list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		Browser: tui.BrowserModel{
			KeyList:    list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
			FieldsList: list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		},
		Spinner: spinner.New(),
		Input:   tui.InputModel{Input: textinput.New()},
	}
}

// send calls m.Update and returns the updated model and the tea.Cmd.
func send(m tui.Model, msg interface{}) (tui.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(tui.Model), cmd
}

// -------------------------------------------------------------------
// mockConn — in-memory net.Conn backed by a strings.Reader.
// Write discards bytes; Read returns from the pre-loaded response string.
// -------------------------------------------------------------------

type mockConn struct {
	reader *strings.Reader
}

func newMockConn(data string) (*mockConn, *bufio.Reader) {
	mc := &mockConn{reader: strings.NewReader(data)}
	return mc, bufio.NewReader(mc)
}

func (c *mockConn) Read(b []byte) (int, error)         { return c.reader.Read(b) }
func (c *mockConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *mockConn) Close() error                       { return nil }
func (c *mockConn) LocalAddr() net.Addr                { return nil }
func (c *mockConn) RemoteAddr() net.Addr               { return nil }
func (c *mockConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// mockNetError satisfies net.Error for simulating connection failures.
type mockNetError struct{}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return false }
func (e *mockNetError) Temporary() bool { return true }
