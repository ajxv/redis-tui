package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// NewHelp returns a help.Model styled with the v2 palette: accent keys, dim
// descriptions, and a faint separator.
func NewHelp() help.Model {
	h := help.New()
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnBorder))
	h.Styles.ShortKey = keyStyle
	h.Styles.ShortDesc = descStyle
	h.Styles.ShortSeparator = sepStyle
	h.Styles.FullKey = keyStyle
	h.Styles.FullDesc = descStyle
	h.Styles.FullSeparator = sepStyle
	h.ShortSeparator = "   "
	return h
}

// menuKeyMap — main command launcher.
type menuKeyMap struct {
	Run    key.Binding
	Filter key.Binding
	Quit   key.Binding
}

func (k menuKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Run, k.Filter, k.Quit}
}
func (k menuKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Run, k.Filter, k.Quit}}
}

var menuKeys = menuKeyMap{
	Run:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "run")),
	Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Quit:   key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
}

// browserKeyMap — key list (not viewing fields).
type browserKeyMap struct {
	Open    key.Binding
	Filter  key.Binding
	Delete  key.Binding
	Rename  key.Binding
	More    key.Binding
	Refresh key.Binding
	Back    key.Binding
}

func (k browserKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Filter, k.Delete, k.Rename, k.Back}
}
func (k browserKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Open, k.Filter, k.Delete}, {k.Rename, k.More}, {k.Refresh, k.Back}}
}

var browserKeys = browserKeyMap{
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
	Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Rename:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
	More:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "load more")),
	Refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

// hashFieldsKeyMap — fields browser for hash keys (includes 'a' to add a field).
type hashFieldsKeyMap struct {
	Open    key.Binding
	Filter  key.Binding
	Add     key.Binding
	Delete  key.Binding
	Export  key.Binding
	Import  key.Binding
	More    key.Binding
	Refresh key.Binding
	Back    key.Binding
}

func (k hashFieldsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Filter, k.Add, k.Delete, k.Export, k.Import, k.Back}
}
func (k hashFieldsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Open, k.Filter, k.Add, k.Delete}, {k.Export, k.Import, k.More}, {k.Refresh, k.Back}}
}

var hashFieldsKeys = hashFieldsKeyMap{
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open/edit")),
	Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add field")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Export:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "export")),
	Import:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "import")),
	More:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "load more")),
	Refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to keys")),
}

// otherFieldsKeyMap — fields browser for list / set / zset (add supported).
type otherFieldsKeyMap struct {
	Open    key.Binding
	Filter  key.Binding
	Add     key.Binding
	Delete  key.Binding
	Export  key.Binding
	Import  key.Binding
	More    key.Binding
	Refresh key.Binding
	Back    key.Binding
}

func (k otherFieldsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Filter, k.Add, k.Delete, k.Export, k.Import, k.Back}
}
func (k otherFieldsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Open, k.Filter, k.Add, k.Delete}, {k.Export, k.Import, k.More}, {k.Refresh, k.Back}}
}

var otherFieldsKeys = otherFieldsKeyMap{
	Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open")),
	Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Add:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Export:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "export")),
	Import:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "import")),
	More:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "load more")),
	Refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to keys")),
}

// outputKeyMap — value inspector screen.
type outputKeyMap struct {
	Scroll key.Binding
	Edit   key.Binding
	Copy   key.Binding
	TTL    key.Binding
	Back   key.Binding
}

func (k outputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Scroll, k.Copy, k.Edit, k.TTL, k.Back}
}
func (k outputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Scroll, k.Copy, k.Edit, k.TTL, k.Back}}
}

var outputKeys = outputKeyMap{
	Scroll: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "scroll")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	TTL:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "ttl")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "return")),
}

// memberOutputKeyMap — set/zset member view. Members are immutable in place
// (SREM+SADD would be needed to "edit" one), so unlike outputKeyMap this omits
// Edit rather than advertising a key that's a silent no-op.
type memberOutputKeyMap struct {
	Scroll key.Binding
	Copy   key.Binding
	TTL    key.Binding
	Back   key.Binding
}

func (k memberOutputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Scroll, k.Copy, k.TTL, k.Back}
}
func (k memberOutputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Scroll, k.Copy, k.TTL, k.Back}}
}

var memberOutputKeys = memberOutputKeyMap{
	Scroll: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "scroll")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	TTL:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "ttl")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "return")),
}

// infoOutputKeyMap — server INFO screen (read-only, no edit/ttl).
type infoOutputKeyMap struct {
	Scroll key.Binding
	Copy   key.Binding
	Back   key.Binding
}

func (k infoOutputKeyMap) ShortHelp() []key.Binding { return []key.Binding{k.Scroll, k.Copy, k.Back} }
func (k infoOutputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Scroll, k.Copy, k.Back}}
}

var infoOutputKeys = infoOutputKeyMap{
	Scroll: key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "scroll")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "return")),
}

// inputKeyMap — all text-input screens.
type inputKeyMap struct {
	Submit key.Binding
	Back   key.Binding
}

func (k inputKeyMap) ShortHelp() []key.Binding { return []key.Binding{k.Submit, k.Back} }
func (k inputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Submit, k.Back}}
}

var inputKeys = inputKeyMap{
	Submit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "submit")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

// filePathKeyMap — file path prompts (import/export), adds Tab completion.
type filePathKeyMap struct {
	Submit   key.Binding
	Complete key.Binding
	Back     key.Binding
}

func (k filePathKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Complete, k.Back}
}
func (k filePathKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Submit, k.Complete, k.Back}}
}

var filePathKeys = filePathKeyMap{
	Submit:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "submit")),
	Complete: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete path")),
	Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}
