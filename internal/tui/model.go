package tui

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ajxv/redis-tui/internal/redis"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	CurrentState           AppState
	StateNavigationHistory []AppState
	MenuList               list.Model
	Input                  InputModel
	Output                 string
	ActiveKey              string
	ActiveField            string
	ActiveIndex            int
	ActiveValue            string
	ActiveTTL              string
	PreservedTTL           int
	CopyStatus             string
	SelectedOp             Op
	PickerOp               Op // original collection command driving the key picker
	Conn                   net.Conn
	RedisAddress           string
	Password               string
	Username               string
	DB                     int
	TLSConfig              *tls.Config
	DialTimeout            time.Duration
	ReadTimeout            time.Duration
	ReconnectAttempts      int
	LastPattern            string
	Reader                 *bufio.Reader
	Browser                BrowserModel
	Spinner                spinner.Model
	Help                   help.Model
	Viewport               viewport.Model // scrolls the value / INFO output
	WindowWidth            int
	WindowHeight           int
}

// outputVPSize returns the viewport dimensions for the output screen — the
// value box minus its border/padding, and the screen height minus the fixed
// chrome (header, label, box border, meta line, footer).
func (m Model) outputVPSize() (w, h int) {
	w = m.WindowWidth - 10
	if w < 10 {
		w = 10
	}
	h = m.WindowHeight - 9
	if h < 3 {
		h = 3
	}
	return w, h
}

// withOutputViewport loads the viewport whenever a handler lands on the output
// screen, so the value/INFO content is scrollable instead of overflowing.
func withOutputViewport(model tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if m, ok := model.(Model); ok && m.CurrentState == StateOutput {
		m.refreshOutputViewport()
		return m, cmd
	}
	return model, cmd
}

// colorizeOutput renders m.Output for the value inspector: INFO sections dimmed,
// JSON syntax-highlighted, everything else plain green.
func colorizeOutput(output string, op Op) string {
	trimmed := strings.TrimSpace(output)
	switch {
	case op == OpInfo:
		return colorizeInfo(output)
	case len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '['):
		return colorizeJSON(output)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(tnGreen)).Render(output)
	}
}

// wrapOutput soft-wraps colorized output to width w so long lines wrap inside
// the viewport instead of being clipped.
func wrapOutput(s string, w int) string {
	if w <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(w).Render(s)
}

// refreshOutputViewport loads the colorized output into the viewport, sizes it,
// and scrolls to the top. Called whenever we enter StateOutput (so scroll bounds
// are correct for key handling).
func (m *Model) refreshOutputViewport() {
	w, maxH := m.outputVPSize()
	content := wrapOutput(colorizeOutput(m.Output, m.SelectedOp), w)
	m.Viewport.Width = w
	m.Viewport.Height = outputBoxHeight(content, maxH)
	m.Viewport.SetContent(content)
	m.Viewport.GotoTop()
}

// outputBoxHeight caps the value box to the content's own height (with a
// 3-line floor for visual weight) so a one-word value doesn't leave a wall of
// empty space below it, while never exceeding the window's available height.
func outputBoxHeight(content string, maxH int) int {
	h := lipgloss.Height(content)
	if h < 3 {
		h = 3
	}
	if h > maxH {
		h = maxH
	}
	return h
}

// headerView renders the persistent connection status line + a bottom rule.
// No background fill — the terminal's own background shows through.
func (m Model) headerView() string {
	w := m.WindowWidth
	if w <= 0 {
		w = 80
	}

	app := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Bold(true).Render("redis-tui")
	addr := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim)).
		Render(fmt.Sprintf("%s · db%d", m.RedisAddress, m.DB))

	dotColor, glyph, label := tnGreen, "●", "connected"
	if m.Conn == nil {
		dotColor, glyph, label = tnRed, "○", "connecting…"
	}
	dot := lipgloss.NewStyle().Foreground(lipgloss.Color(dotColor)).Render(glyph)
	status := dot + " " + lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim)).Render(label)

	left := "  " + app + "  " + addr
	gap := w - lipgloss.Width(left) - lipgloss.Width(status) - 2
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + status + "  "

	rule := lipgloss.NewStyle().Foreground(lipgloss.Color(tnBorder)).Render(strings.Repeat("─", w))
	return bar + "\n" + rule
}

// fieldExportFilename builds the default JSON filename for exporting a single
// hash field, list element, or set/zset member — shared by the input prompt's
// pre-filled value and the fallback used when the user submits a bare
// directory instead of a full path.
func fieldExportFilename(key, keyType, field string, index int) string {
	base := sanitizeFilename(key) + "_"
	if keyType == "list" {
		base += strconv.Itoa(index)
	} else {
		base += sanitizeFilename(field)
	}
	return base + ".json"
}

// sanitizeFilename turns a Redis key into a safe default filename component by
// replacing path/special characters with underscores.
func sanitizeFilename(key string) string {
	repl := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '_'
		}
		return r
	}
	out := strings.Map(repl, key)
	if out == "" {
		out = "key"
	}
	return out
}

// footerSep renders the full-width rule drawn above a screen's key-hint footer.
func footerSep(width int) string {
	if width <= 0 {
		width = 80
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(tnBorder)).Render(strings.Repeat("─", width))
}

// bottomFooter pins a footer block to the bottom of a height-row screen by
// padding the gap between the body and the footer with blank lines.
func bottomFooter(body, foot string, height int) string {
	body = strings.TrimRight(body, "\n")
	gap := height - lipgloss.Height(body) - lipgloss.Height(foot)
	if gap < 0 {
		gap = 0
	}
	// gap+1 newlines: one terminates the body's last line, the rest are blank
	// filler, so the footer always starts on its own line and lands at the bottom.
	return body + strings.Repeat("\n", gap+1) + foot
}

const ansiReset = "\x1b[0m"

// baseBgSeq returns the SGR sequence that opens the base background under the
// active color profile (empty when the terminal has no color).
func baseBgSeq() string {
	s := lipgloss.NewStyle().Background(lipgloss.Color(tnBase)).Render("X")
	if i := strings.IndexByte(s, 'X'); i > 0 {
		return s[:i]
	}
	return ""
}

// applyBackground paints the base background across the whole screen. lipgloss
// fills each line to full width and the frame down to full height; then we
// re-open the background after every reset so foreground-colored spans don't
// punch holes in it.
func applyBackground(frame string, w, h int) string {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	wrapped := lipgloss.NewStyle().
		Background(lipgloss.Color(tnBase)).
		Width(w).
		Height(h).
		Render(frame)
	seq := baseBgSeq()
	if seq == "" {
		return wrapped
	}
	return strings.ReplaceAll(wrapped, ansiReset, ansiReset+seq)
}

// menuView renders the main command menu with grouped, spaced sections exactly
// like the redesign: a "Select a command" hint (or live filter), the promoted
// EXPLORE row, then each type group under a dim label. The item rows scroll to
// keep the selection visible when they exceed the available height.
func (m Model) menuView(avail int) string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	faint := lipgloss.NewStyle().Foreground(lipgloss.Color(tnFaint))

	var head string
	switch m.MenuList.FilterState() {
	case list.Filtering:
		head = "  " + m.MenuList.FilterInput.View()
	case list.FilterApplied:
		// Once a filter is accepted (Enter), the input box above disappears —
		// without this, "Select a command" would show unchanged and give no
		// hint that a filter is still narrowing the list, or that esc clears it.
		head = "  " + dim.Render("Filter: ") + faint.Render(m.MenuList.FilterInput.Value()) + dim.Render("  (esc to clear)")
	default:
		head = "  " + dim.Render("Select a command")
	}

	items := m.MenuList.VisibleItems()
	filtering := m.MenuList.FilterState() != list.Unfiltered
	selTitle := ""
	if sel, ok := m.MenuList.SelectedItem().(ListItem); ok {
		selTitle = sel.title
	}

	var lines []string
	selLine := 0
	lastGroup := ""
	for _, it := range items {
		li, ok := it.(ListItem)
		if !ok {
			continue
		}
		// Group label above the first item of each group (hidden while filtering).
		if !filtering && li.group != "" && li.group != lastGroup {
			lines = append(lines, "  "+dim.Render(li.group))
			lastGroup = li.group
		}
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(commandColor(li.title))).Width(menuNameCol)
		var row string
		if li.title == selTitle {
			marker := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Render(pointerGlyph)
			row = marker + nameStyle.Bold(true).Render(li.title) + " " + faint.Render(li.desc)
			selLine = len(lines)
		} else {
			row = "  " + nameStyle.Render(li.title) + " " + faint.Render(li.desc)
		}
		lines = append(lines, row, "") // blank line between rows for breathing room
	}

	// Scroll the item rows to keep the selection visible. head + blank take 2 rows.
	body := avail - 2
	if body > 0 && len(lines) > body {
		start := selLine - body/2
		if start < 0 {
			start = 0
		}
		if start > len(lines)-body {
			start = len(lines) - body
		}
		lines = lines[start : start+body]
	}
	return head + "\n\n" + strings.Join(lines, "\n")
}

func (m *Model) pushState(state AppState) {
	n := len(m.StateNavigationHistory)
	if n > 0 && m.StateNavigationHistory[n-1] == state {
		return
	}
	m.StateNavigationHistory = append(m.StateNavigationHistory, state)
}

func (m *Model) popState() AppState {
	if len(m.StateNavigationHistory) == 0 {
		return StateMenu
	}

	// get last state
	lastIndex := len(m.StateNavigationHistory) - 1
	lastState := m.StateNavigationHistory[lastIndex]

	// remove last state from stack
	m.StateNavigationHistory = m.StateNavigationHistory[:lastIndex]

	return lastState
}

func (m Model) switchToLoadingAndExecute(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.CurrentState = StateLoading
	// Re-seed the spinner tick so it animates on every loading entry.
	// Without this, the tick chain dies after the first time we leave StateLoading,
	// and the spinner freezes on all subsequent loads.
	return m, tea.Batch(m.Spinner.Tick, cmd)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, connectToRedis(m), textarea.Blink)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// handle keyboard events
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	case BackMsg:
		m.CurrentState = m.popState()
		if m.CurrentState == StateMenu {
			m.Browser.Picking = false // leaving the key picker back to the menu
		}
		// Sync Input.Type and Hint with the recovered state.
		// Without this, esc from a nested input (e.g. ZAdd member → score) leaves
		// Input.Type pointing at the wrong step, causing the form title to be wrong
		// and InputCompleteMsg to carry the wrong Type when the user submits.
		switch m.CurrentState {
		case StateInputKey:
			m.Input.Type = InputKey
			m.Input.Hint = ""
		case StateInputField:
			m.Input.Type = InputField
			if m.SelectedOp == OpZAdd {
				m.Input.Hint = "Input the score:"
			} else {
				m.Input.Hint = ""
			}
		case StateInputValue:
			m.Input.Type = InputValue
			switch m.SelectedOp {
			case OpZAdd:
				m.Input.Hint = "Input the member:"
			case OpExpirySet:
				m.Input.Hint = "TTL in seconds (enter 0 to remove expiry / PERSIST):"
			default:
				m.Input.Hint = ""
			}
		case StateInputFilePath:
			m.Input.Type = InputFilePath
			m.Input.Hint = ""
		default:
			m.Input.Hint = ""
		}

	case SelectKeyMsg:
		m.ActiveKey = msg.Key

		// Picker + add command: the user picked an existing collection to add to,
		// so jump straight into the add form instead of browsing it.
		if m.Browser.Picking && isAddOp(m.PickerOp) {
			m.SelectedOp = m.PickerOp
			m.Browser.ActiveKey = msg.Key
			m.Browser.ActiveKeyType = m.Browser.PickerType
			cmd := m.Browser.StartAdd()
			return m, cmd
		}

		// Picker + EXPORT: picked a key to dump — go to the file-path prompt with
		// a sensible default destination.
		if m.Browser.Picking && m.PickerOp == OpExport {
			// Stay in the picker (Picking stays true) so returning to the key
			// list still exports on the next selection rather than browsing.
			m.SelectedOp = OpExport
			m.ActiveKey = msg.Key
			m.pushState(m.CurrentState)
			m.Input.Input.SetValue("./" + sanitizeFilename(msg.Key) + ".dump")
			m.Input.Input.CursorEnd()
			m.Input.Input.Focus()
			m.Input.Type = InputFilePath
			m.Input.Hint = "Destination file for " + msg.Key + ":"
			m.CurrentState = StateInputFilePath
			return m, nil
		}

		m.pushState(m.CurrentState)

		cmd := redis.RedisCmd{
			Name: "TYPE",
			Args: []string{m.ActiveKey},
		}

		m.SelectedOp = OpCheckType
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

	case InputCompleteMsg:
		// Handle the data based on what kind of input it was
		switch msg.Type {
		case InputPattern:
			m.ActiveKey = msg.Value
			switch m.SelectedOp {
			case OpExplore:
				m.LastPattern = m.ActiveKey
				m.Input.RecentPatterns = prependPattern(m.Input.RecentPatterns, m.ActiveKey)
				m.Browser.Cursor = "0"
				m.Browser.Pattern = m.ActiveKey
				return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, m.ActiveKey, "0"))
			}

		case InputKey:
			m.ActiveKey = msg.Value

			// New key under an add command — go straight to the add form.
			if m.Browser.Picking && isAddOp(m.SelectedOp) {
				m.Browser.ActiveKey = msg.Value
				m.Browser.ActiveKeyType = collectionType(m.SelectedOp)
				m.CurrentState = StateBrowser
				cmd := m.Browser.StartAdd()
				return m, cmd
			}

			// decide where to go next
			switch m.SelectedOp {
			case OpGet:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpSet, OpRPush, OpLPush, OpSAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				// clear previous input
				m.Input.Input.SetValue("")

			case OpHSet, OpZAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputField
				m.Input.Type = InputField
				if m.SelectedOp == OpZAdd {
					m.Input.Hint = "Input the score:"
				} else {
					m.Input.Hint = ""
				}
				// clear previous input
				m.Input.Input.SetValue("")

			case OpHGet:
				cmd := redis.RedisCmd{
					Name: "HKEYS",
					Args: []string{m.ActiveKey},
				}

				m.SelectedOp = OpHKeys

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpDelete:
				// Route through the same confirm-before-delete screen the
				// Explore browser's 'd' key uses, instead of deleting the typed
				// key immediately — a typo here would otherwise be unrecoverable.
				m.ActiveField = ""
				m.SelectedOp = OpDel
				m.pushState(m.CurrentState)
				m.CurrentState = StateConfirmation
				return m, nil

			}

		case InputField:
			m.ActiveField = msg.Value

			// decide where to go next
			switch m.SelectedOp {
			case OpHSet, OpZAdd:
				m.pushState(m.CurrentState)
				m.CurrentState = StateInputValue
				m.Input.Type = InputValue
				if m.SelectedOp == OpZAdd {
					m.Input.Hint = "Input the member:"
				} else {
					m.Input.Hint = ""
				}
				// clear previous input
				m.Input.Input.SetValue("")
			}

		case InputValue:
			m.ActiveValue = msg.Value

			switch m.SelectedOp {
			case OpSet:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpHSet, OpZAdd:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveField, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpLSet:
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, strconv.Itoa(m.ActiveIndex), m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpRPush, OpLPush, OpSAdd:
				// send command
				cmd := redis.RedisCmd{
					Name: m.SelectedOp.String(),
					Args: []string{m.ActiveKey, m.ActiveValue},
				}

				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpRename:
				cmd := redis.RedisCmd{
					Name: "RENAME",
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				m.ActiveKey = m.ActiveValue // keep model in sync
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			case OpExpirySet:
				if m.ActiveValue == "0" {
					cmd := redis.RedisCmd{
						Name: "PERSIST",
						Args: []string{m.ActiveKey},
					}
					return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
				}
				cmd := redis.RedisCmd{
					Name: "EXPIRE",
					Args: []string{m.ActiveKey, m.ActiveValue},
				}
				return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

			}

		case InputFilePath:
			filePath := msg.Value

			switch m.SelectedOp {
			case OpExport:
				return m.switchToLoadingAndExecute(ExportSingleKey(m.Conn, m.Reader, m.ActiveKey, filePath))
			case OpImport:
				return m.switchToLoadingAndExecute(ImportKeys(m.Conn, m.Reader, filePath))
			case OpExportDB:
				return m.switchToLoadingAndExecute(ExportFullDB(m.Conn, m.Reader, m.DB, filePath))
			case OpImportDB:
				return m.switchToLoadingAndExecute(ImportKeys(m.Conn, m.Reader, filePath))
			case OpExportField:
				return m.switchToLoadingAndExecute(ExportField(m.Conn, m.Reader, m.ActiveKey, m.Browser.ActiveKeyType, m.ActiveField, m.ActiveIndex, filePath))
			case OpImportField:
				return m.switchToLoadingAndExecute(ImportField(m.Conn, m.Reader, filePath, m.ActiveKey, m.Browser.ActiveKeyType))
			}
		}

	case SelectFieldMsg:
		m.ActiveField = msg.Field
		m.ActiveIndex = msg.Index

		// Save state so we can go back
		m.pushState(m.CurrentState)

		// Route by the browser's tracked key type rather than the previous
		// SelectedOp: SelectedOp is left pointing at whatever the last command
		// was (e.g. OpExportField after an export), so switching on it here
		// could silently pick the wrong branch — or none at all — for the very
		// next field selected. ActiveKeyType is always kept in sync with what's
		// actually being browsed.
		switch m.Browser.ActiveKeyType {
		case "hash":
			cmd := redis.RedisCmd{
				Name: "HGET",
				Args: []string{m.ActiveKey, m.ActiveField},
			}
			m.SelectedOp = OpHGet
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

		case "list":
			m.SelectedOp = OpExploreList
			m.Output = m.ActiveField
			m.CurrentState = StateOutput
			m.refreshOutputViewport()

		case "set":
			// Set members are their own values; display directly.
			m.SelectedOp = OpExploreSet
			m.Output = m.ActiveField
			m.CurrentState = StateOutput
			m.refreshOutputViewport()

		case "zset":
			// The score is visible in the list description but not surfaced
			// here — the user can copy the member name with 'c'.
			m.SelectedOp = OpExploreZSet
			m.Output = m.ActiveField
			m.CurrentState = StateOutput
			m.refreshOutputViewport()
		}

	case DeleteRequestMsg:
		m.ActiveKey = msg.Key
		m.ActiveField = msg.Field

		// Set the correct Op based on context
		if msg.Field == "" {
			m.SelectedOp = OpDel // Delete Key
		} else {
			// Delete Field/Member — routed by ActiveKeyType (see SelectFieldMsg
			// above) rather than SelectedOp, which may be stale.
			switch m.Browser.ActiveKeyType {
			case "list":
				m.SelectedOp = OpLRem
			case "set":
				m.SelectedOp = OpSRem
			case "zset":
				m.SelectedOp = OpZRem
			default:
				m.SelectedOp = OpHDel
			}
		}

		m.pushState(m.CurrentState)
		m.CurrentState = StateConfirmation

	case RenameRequestMsg:
		m.ActiveKey = msg.Key
		m.SelectedOp = OpRename
		m.Input.Input.SetValue(msg.Key)
		m.Input.Type = InputValue
		m.Input.Hint = ""
		m.Input.Input.Focus()
		m.Input.Input.CursorEnd()
		m.pushState(m.CurrentState)
		m.CurrentState = StateInputValue

	case FieldExportRequestMsg:
		// Export the highlighted field/member to a self-describing JSON file.
		m.ActiveField = msg.Field
		m.ActiveIndex = msg.Index
		m.SelectedOp = OpExportField
		m.pushState(m.CurrentState)
		m.Input.Input.SetValue("./" + fieldExportFilename(m.ActiveKey, m.Browser.ActiveKeyType, msg.Field, msg.Index))
		m.Input.Input.CursorEnd()
		m.Input.Input.Focus()
		m.Input.Type = InputFilePath
		m.Input.Hint = "Destination file for this " + m.Browser.ActiveKeyType + " entry:"
		m.CurrentState = StateInputFilePath

	case FieldImportRequestMsg:
		m.SelectedOp = OpImportField
		m.pushState(m.CurrentState)
		m.Input.Input.SetValue("")
		m.Input.Input.Focus()
		m.Input.Type = InputFilePath
		m.Input.Hint = "Source .json field file to import:"
		m.CurrentState = StateInputFilePath

	case NewKeyRequestMsg:
		// "＋ new key…" chosen in the picker — prompt for a new key name (the
		// picker context stays active so InputKey can route into the add form).
		m.SelectedOp = m.PickerOp
		m.Input.Input.Focus()
		m.Input.Input.SetValue("")
		m.Input.Type = InputKey
		m.Input.Hint = ""
		m.CurrentState = StateInputKey

	case AddItemMsg:
		m.ActiveKey = msg.Key
		var cmd redis.RedisCmd
		switch msg.Type {
		case "hash":
			cmd = redis.RedisCmd{Name: "HSET", Args: []string{msg.Key, msg.A, msg.B}}
		case "zset":
			cmd = redis.RedisCmd{Name: "ZADD", Args: []string{msg.Key, msg.B, msg.A}}
		case "set":
			cmd = redis.RedisCmd{Name: "SADD", Args: []string{msg.Key, msg.A}}
		case "list":
			cmd = redis.RedisCmd{Name: "RPUSH", Args: []string{msg.Key, msg.A}}
		default:
			return m, nil
		}
		m.SelectedOp = OpAddItem
		return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))

	case tea.WindowSizeMsg:
		m.WindowWidth = msg.Width
		m.WindowHeight = msg.Height
		m.Input.Width = msg.Width
		m.Input.Height = msg.Height

		// List area fills the screen between the 2-line header and the 2-line
		// footer (separator rule + key-hint line).
		listHeight := msg.Height - 4

		m.MenuList.SetWidth(msg.Width)
		m.MenuList.SetHeight(listHeight)

		m.Browser.FieldsList.SetWidth(msg.Width)
		m.Browser.FieldsList.SetHeight(listHeight)
		m.Browser.KeyList.SetHeight(listHeight)
		m.Browser.KeyList.SetWidth(msg.Width)
		m.Browser.Width = msg.Width
		m.Browser.Height = msg.Height

		vw, vh := m.outputVPSize()
		m.Viewport.Width = vw
		m.Viewport.Height = vh

	case TickMsg:
		return m, connectToRedis(m)

	case spinner.TickMsg:
		if m.CurrentState == StateLoading {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}

	case RedisTTLResultMsg:
		if msg.TTL == -1 || msg.TTL == -2 {
			m.ActiveTTL = "no expiry"
		} else {
			m.ActiveTTL = strconv.Itoa(msg.TTL) + " s"
		}
		return m, nil

	case ClearCopyStatusMsg:
		m.CopyStatus = ""
		return m, nil

	case RedisConnectionMsg:
		return withOutputViewport(handleRedisConnection(m, msg))

	case LoadMoreKeysMsg:
		if m.Browser.Picking && m.Browser.PickerType != "" {
			return m.switchToLoadingAndExecute(scanRedisKeysOfType(m.Conn, m.Reader, m.Browser.Pattern, m.Browser.Cursor, m.Browser.PickerType))
		}
		return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, m.Browser.Pattern, m.Browser.Cursor))

	case LoadMoreFieldsMsg:
		switch m.SelectedOp {
		case OpExploreList:
			end := m.Browser.FieldOffset + fieldPageSize - 1
			cmd := redis.RedisCmd{Name: "LRANGE", Args: []string{m.ActiveKey, strconv.Itoa(m.Browser.FieldOffset), strconv.Itoa(end)}}
			m.SelectedOp = OpLRange
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		case OpExploreSet:
			cmd := redis.RedisCmd{Name: "SSCAN", Args: []string{m.ActiveKey, m.Browser.FieldCursor, "COUNT", strconv.Itoa(fieldPageSize)}}
			m.SelectedOp = OpSMembers
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		case OpExploreZSet:
			end := m.Browser.FieldOffset + fieldPageSize - 1
			cmd := redis.RedisCmd{Name: "ZRANGE", Args: []string{m.ActiveKey, strconv.Itoa(m.Browser.FieldOffset), strconv.Itoa(end), "WITHSCORES"}}
			m.SelectedOp = OpZRange
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		}

	case RefreshMsg:
		if m.Browser.ViewingFields {
			// Instead of trusting stale state, let's confidently check the key TYPE again.
			// This automatically drops into OpCheckType, which correctly re-routes to
			// OpHKeys, OpExploreList, etc., OR catches if the key was deleted in the meantime!
			m.SelectedOp = OpCheckType
			cmd := redis.RedisCmd{Name: "TYPE", Args: []string{m.ActiveKey}}
			return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
		} else {
			// Top-level key scan
			m.SelectedOp = OpExplore
			pattern := m.Browser.Pattern
			if pattern == "" {
				pattern = "*"
			}
			m.Browser.Cursor = "0"
			if m.Browser.Picking && m.Browser.PickerType != "" {
				return m.switchToLoadingAndExecute(scanRedisKeysOfType(m.Conn, m.Reader, pattern, "0", m.Browser.PickerType))
			}
			return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, pattern, "0"))
		}
	case RedisResultMsg:
		return withOutputViewport(handleRedisResult(m, msg))
	}

	switch m.CurrentState {
	case StateMenu:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			isFiltering := m.MenuList.FilterState() == list.Filtering

			switch keyMsg.String() {
			case "enter":
				// While the filter input is active, let the list handle Enter
				// (it confirms the filter and moves to FilterApplied state).
				if !isFiltering {
					selectedItem := m.MenuList.SelectedItem()
					if selectedItem, ok := selectedItem.(ListItem); ok {
						m.SelectedOp = ParseOp(selectedItem.title)

						// save state history
						m.pushState(m.CurrentState)

						switch m.SelectedOp {
						case OpSet, OpGet, OpDelete:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("") // Clear previous input
							m.CurrentState = StateInputKey
							m.Input.Type = InputKey
						case OpHSet, OpHGet, OpSAdd, OpZAdd, OpRPush, OpLPush:
							// Key picker: choose an existing key of the matching
							// type (or "＋ new key…"). Browse via the Explore machinery.
							m.PickerOp = m.SelectedOp
							m.Browser.Picking = true
							m.Browser.PickerType = collectionType(m.SelectedOp)
							m.Browser.ViewingFields = false
							m.Browser.Cursor = "0"
							m.Browser.Pattern = "*"
							m.SelectedOp = OpExplore
							return m.switchToLoadingAndExecute(scanRedisKeysOfType(m.Conn, m.Reader, "*", "0", m.Browser.PickerType))
						case OpExport:
							// Key picker over all keys; selecting one pre-fills a
							// sensible ./<key>.dump destination path.
							m.PickerOp = OpExport
							m.Browser.Picking = true
							m.Browser.PickerType = "" // any type
							m.Browser.ViewingFields = false
							m.Browser.Cursor = "0"
							m.Browser.Pattern = "*"
							m.SelectedOp = OpExplore
							return m.switchToLoadingAndExecute(scanRedisKeys(m.Conn, m.Reader, "*", "0"))
						case OpExportDB:
							m.Input.Input.Focus()
							m.Input.Input.SetValue(fmt.Sprintf("./redis-db%d.json", m.DB))
							m.Input.Input.CursorEnd()
							m.Input.Hint = "Destination file (database → JSON):"
							m.CurrentState = StateInputFilePath
							m.Input.Type = InputFilePath
						case OpImport:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("")
							m.Input.Hint = "Source .dump file to restore:"
							m.CurrentState = StateInputFilePath
							m.Input.Type = InputFilePath
						case OpImportDB:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("")
							m.Input.Hint = "Source .json file to import:"
							m.CurrentState = StateInputFilePath
							m.Input.Type = InputFilePath
						case OpExplore:
							m.Input.Input.Focus()
							m.Input.Input.SetValue("*") // Default search is everything
							m.CurrentState = StateInputKey
							m.Input.Type = InputPattern
						case OpInfo:
							cmd := redis.RedisCmd{
								Name: "INFO",
							}
							return m.switchToLoadingAndExecute(sendRedisCmd(m.Conn, m.Reader, cmd, m.ReadTimeout))
						}
					}
				}
			case "q":
				// q quits directly from the main menu (no confirmation).
				// While filtering, let the list handle it as filter input.
				if !isFiltering {
					return m, tea.Quit
				}
			case "esc":
				// esc cancels the filter while typing, or clears an
				// already-applied filter — either way, let the list handle
				// it. isFiltering above only covers the actively-typing case,
				// so it's checked separately here: without this, esc on a
				// filtered-but-not-currently-typing menu (i.e. right after
				// pressing Enter to accept a filter) would do nothing at all,
				// permanently stranding the menu on the filtered subset with
				// no way back short of quitting the app.
				if m.MenuList.FilterState() != list.Unfiltered {
					break
				}
				// On the idle, unfiltered menu esc is a no-op — the menu is
				// the root screen, nothing to go back to.
				return m, nil
			default:
				// Any printable character while the filter is not yet active
				// automatically opens the filter input, then the character(s)
				// are forwarded so they appear immediately in the search box.
				// Runes can arrive batched (fast typing, paste, or laggy input)
				// rather than one KeyMsg per character, so this must not
				// require exactly one rune or those keystrokes are silently
				// dropped instead of opening the filter.
				if !isFiltering && len(keyMsg.Runes) >= 1 {
					m.MenuList, _ = m.MenuList.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
				}
			}
		}

		updatedModel, cmd := m.MenuList.Update(msg)
		m.MenuList = updatedModel
		return m, cmd

	case StateInputKey, StateInputField, StateInputValue, StateInputFilePath:
		inputModel, cmd := m.Input.Update(msg)
		m.Input = inputModel
		return m, cmd

	case StateOutput:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return handleStateOutputKey(m, keyMsg)
		}

	case StateBrowser:
		browserModel, cmd := m.Browser.Update((msg))
		m.Browser = browserModel
		return m, cmd

	case StateConfirmation:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return handleStateConfirmationKey(m, keyMsg)
		}

	}

	return m, nil
}

// View paints the active screen and fills the whole terminal with the base
// background.
func (m Model) View() string {
	return applyBackground(m.viewContent(), m.WindowWidth, m.WindowHeight)
}

func (m Model) viewContent() string {
	header := m.headerView()

	h := m.Help
	h.Width = m.WindowWidth

	switch m.CurrentState {
	case StateMenu:
		avail := m.WindowHeight - 4 // header(2) + footer rule(1) + help(1)
		body := header + "\n" + m.menuView(avail)
		foot := footerSep(m.WindowWidth) + "\n  " + h.View(menuKeys)
		return bottomFooter(body, foot, m.WindowHeight)

	case StateInputKey, StateInputField, StateInputValue, StateInputFilePath:
		return header + "\n" + m.Input.View()

	case StateOutput:
		var helpView string
		switch m.SelectedOp {
		case OpInfo:
			helpView = "  " + h.View(infoOutputKeys)
		case OpExploreSet, OpExploreZSet:
			helpView = "  " + h.View(memberOutputKeys)
		default:
			helpView = "  " + h.View(outputKeys)
		}

		// "Output: ..." label line. m.ActiveKey is stale for operations that
		// aren't scoped to a single key (server INFO, whole-database export
		// /import) — showing it there would just be whatever key was last
		// browsed, so swap in a label that actually describes the screen.
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle))
		outputSubject := m.ActiveKey
		switch m.SelectedOp {
		case OpInfo:
			outputSubject = "Server INFO"
		case OpExportDB, OpImportDB:
			outputSubject = fmt.Sprintf("Database %d", m.DB)
		}
		label := "  " + labelStyle.Render("Output: ") + keyStyle.Render(outputSubject)

		// The value/INFO content lives in a scrollable viewport so long output
		// never overflows or loses its top. Render from a local copy with the
		// content set (the scroll offset persists on m.Viewport).
		contentWidth := m.WindowWidth - 8
		if contentWidth <= 0 {
			contentWidth = 72
		}
		vp := m.Viewport
		var maxH int
		vp.Width, maxH = m.outputVPSize()
		content := wrapOutput(colorizeOutput(m.Output, m.SelectedOp), vp.Width)
		vp.Height = outputBoxHeight(content, maxH)
		vp.SetContent(content)
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(tnBorder)).
			Padding(0, 1).
			MarginLeft(2).
			Width(contentWidth).
			Render(vp.View())

		// Meta row — TTL on the left, copy confirmation right-justified.
		metaLeft := ""
		if m.ActiveTTL != "" {
			metaLeft = labelStyle.Render("TTL: ") + keyStyle.Render(m.ActiveTTL)
		}
		toast := ""
		if m.CopyStatus != "" {
			toast = lipgloss.NewStyle().Foreground(lipgloss.Color(tnGreen)).Render("✓ " + m.CopyStatus)
		}
		metaRow := ""
		if metaLeft != "" || toast != "" {
			gap := contentWidth - lipgloss.Width(metaLeft) - lipgloss.Width(toast)
			if gap < 1 {
				gap = 1
			}
			metaRow = "\n  " + metaLeft + strings.Repeat(" ", gap) + toast
		}

		body := header + "\n\n" + label + "\n" + box + metaRow
		foot := footerSep(m.WindowWidth) + "\n" + helpView
		return bottomFooter(body, foot, m.WindowHeight)

	case StateBrowser:
		return header + "\n" + m.Browser.View()

	case StateLoading:
		spin := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Render(m.Spinner.View())
		label := lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle)).Render("Loading…")
		return header + "\n\n  " + spin + " " + label

	case StateConfirmation:
		// Inline layout (v2): header on top, left-aligned content, footer bar.
		var label, value string
		switch m.SelectedOp {
		case OpDel:
			label, value = "key", m.ActiveKey
		case OpHDel:
			label, value = "field", m.ActiveField
		case OpLRem:
			label, value = "element", m.ActiveField
		case OpSRem, OpZRem:
			label, value = "member", m.ActiveField
		default:
			label, value = "", m.SelectedOp.String()
		}

		title := lipgloss.NewStyle().Foreground(lipgloss.Color(tnRed)).Bold(true).Render("⚠  confirm delete")
		body := "  " + title + "\n\n"
		if label != "" {
			body += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle)).Render(label) + "\n"
		}
		body += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(tnText)).Render(value)

		yPart := lipgloss.NewStyle().Foreground(lipgloss.Color(tnRed)).Render("[y] confirm")
		nPart := lipgloss.NewStyle().Foreground(lipgloss.Color(tnFaint)).Render("[n / esc] cancel")
		footer := "  " + yPart + "    " + nPart

		return bottomFooter(header+"\n\n"+body, footerSep(m.WindowWidth)+"\n"+footer, m.WindowHeight)

	default:
		return ""
	}
}
