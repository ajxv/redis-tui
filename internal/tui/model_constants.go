package tui

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

const fieldPageSize = 100

type AppState int

const (
	StateMenu AppState = iota
	StateInputKey
	StateInputField
	StateFieldSelect
	StateInputValue
	StateOutput
	StateBrowser
	StateLoading
	StateConfirmation
	StateInfo
	StateInputFilePath
)

type Op int

// SectionHeader is a non-selectable list item used as a category label in the menu.
type SectionHeader struct {
	Label string
}

// NewSectionHeader creates a section-header list item with the given label.
func NewSectionHeader(label string) SectionHeader { return SectionHeader{Label: label} }

func (s SectionHeader) Title() string       { return s.Label }
func (s SectionHeader) Description() string { return "" }
func (s SectionHeader) FilterValue() string { return "" }

// groupedMenuDelegate renders menu items with an optional section label on the
// first line. The cursor only ever lands on real ListItem entries — there are no
// separate SectionHeader items to accidentally select.
type groupedMenuDelegate struct {
	list.DefaultDelegate
}

// NewGroupedMenuDelegate returns the custom delegate used by the main menu.
func NewGroupedMenuDelegate() groupedMenuDelegate {
	return groupedMenuDelegate{DefaultDelegate: list.NewDefaultDelegate()}
}

// Height is one tight line per command (fits all 15 in an 80×24 terminal).
// Grouping is carried by the type color of each command name.
func (d groupedMenuDelegate) Height() int  { return 1 }
func (d groupedMenuDelegate) Spacing() int { return 0 }

// menuNameCol is the fixed width of the command-name column.
const menuNameCol = 11

// pointerGlyph is the (compact) selection indicator drawn before active rows.
const pointerGlyph = "› "

func (d groupedMenuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li, ok := item.(ListItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(commandColor(li.title))).Width(menuNameCol)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tnFaint))

	if isSelected {
		marker := lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent)).Render("❯ ")
		fmt.Fprintf(w, "%s%s %s", marker, nameStyle.Bold(true).Render(li.title), descStyle.Render(li.desc))
	} else {
		fmt.Fprintf(w, "  %s %s", nameStyle.Render(li.title), descStyle.Render(li.desc))
	}
}

// StyleList strips the Bubbles list chrome down to a clean, background-free
// look: no status bar / pagination dots, and a dim left-aligned title (the
// default title carries a colored background block we don't want).
func StyleList(l *list.Model) {
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color(tnSubtle))
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 2)
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim)).PaddingLeft(2)
	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent))
	l.FilterInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(tnAccent))
}

// commandColor maps a menu command name to its type color (matches the v2 spec).
func commandColor(name string) string {
	switch name {
	case "EXPLORE":
		return tnAccent
	case "SET", "GET":
		return tnBlue
	case "HSET", "HGET":
		return tnOrange
	case "RPUSH", "LPUSH":
		return tnPurple
	case "SADD":
		return tnGreen
	case "ZADD":
		return tnYellow
	case "DELETE":
		return tnRed
	case "EXPORT", "IMPORT", "EXPORT_DB", "IMPORT_DB":
		return tnSubtle
	case "INFO":
		return tnInfo
	default:
		return tnText
	}
}

const (
	OpNone Op = iota
	OpGet
	OpSet
	OpHSet
	OpHGet
	OpHKeys
	OpRPush
	OpLPush
	OpSAdd
	OpZAdd
	OpDelete
	OpDel
	OpHDel
	OpLRem
	OpSRem
	OpZRem
	OpLRange
	OpLSet
	OpSMembers
	OpZRange
	OpExplore
	OpExploreList
	OpExploreSet
	OpExploreZSet
	OpCheckType
	OpRename
	OpExpirySet
	OpInfo
	OpQuit
	OpExport
	OpImport
	OpExportDB
	OpImportDB
	OpExpireAfterSet
	OpAddItem     // generic add (HSET/ZADD/SADD/RPUSH) from the browser overlay
	OpExportField // export a single hash field / list / set / zset entry
	OpImportField // import a single field/member from a FieldExport file
)

// isAddOp reports whether op is an "add to a collection" command, for which the
// key picker should jump straight into the add form rather than browse the key.
func isAddOp(op Op) bool {
	switch op {
	case OpHSet, OpSAdd, OpZAdd, OpRPush, OpLPush:
		return true
	}
	return false
}

// collectionType maps a menu collection command to the Redis type the key
// picker should filter on.
func collectionType(op Op) string {
	switch op {
	case OpHSet, OpHGet:
		return "hash"
	case OpSAdd:
		return "set"
	case OpZAdd:
		return "zset"
	case OpRPush, OpLPush:
		return "list"
	}
	return ""
}

func (o Op) String() string {
	switch o {
	case OpGet:
		return "GET"
	case OpSet:
		return "SET"
	case OpHSet:
		return "HSET"
	case OpHGet:
		return "HGET"
	case OpHKeys:
		return "HKEYS"
	case OpRPush:
		return "RPUSH"
	case OpLPush:
		return "LPUSH"
	case OpSAdd:
		return "SADD"
	case OpZAdd:
		return "ZADD"
	case OpDelete:
		return "DELETE"
	case OpDel:
		return "DEL"
	case OpHDel:
		return "HDEL"
	case OpLRem:
		return "LREM"
	case OpSRem:
		return "SREM"
	case OpZRem:
		return "ZREM"
	case OpLSet:
		return "LSET"
	case OpExplore, OpExploreList, OpExploreSet, OpExploreZSet:
		return "EXPLORE"
	case OpCheckType:
		return "TYPE"
	case OpRename:
		return "RENAME"
	case OpExpirySet:
		return "EXPIRE"
	case OpInfo:
		return "INFO"
	case OpExport:
		return "EXPORT"
	case OpImport:
		return "IMPORT"
	case OpExportDB:
		return "EXPORT_DB"
	case OpImportDB:
		return "IMPORT_DB"
	}
	return "UNKNOWN"
}

func ParseOp(s string) Op {
	switch s {
	case "GET":
		return OpGet
	case "SET":
		return OpSet
	case "HSET":
		return OpHSet
	case "HGET":
		return OpHGet
	case "HKEYS":
		return OpHKeys
	case "RPUSH":
		return OpRPush
	case "LPUSH":
		return OpLPush
	case "SADD":
		return OpSAdd
	case "ZADD":
		return OpZAdd
	case "DELETE", "DEL", "HDEL", "LREM", "SREM", "ZREM":
		return OpDelete // Maps to general delete in menu
	case "EXPLORE", "EXPLORE_LIST", "EXPLORE_SET", "EXPLORE_ZSET":
		return OpExplore
	case "TYPE":
		return OpCheckType
	case "RENAME":
		return OpRename
	case "EXPIRE":
		return OpExpirySet
	case "INFO":
		return OpInfo
	case "EXPORT":
		return OpExport
	case "IMPORT":
		return OpImport
	case "EXPORT_DB":
		return OpExportDB
	case "IMPORT_DB":
		return OpImportDB
	}
	return OpNone
}

// GitHub-dark palette — matches the v2 design spec exactly.
const (
	tnBase   = "#0d1117" // base background
	tnBorder = "#21262d" // borders / rules
	tnAccent = "#58a6ff" // blue accent / cursor
	tnText   = "#c9d1d9" // primary text
	tnSubtle = "#8b949e" // secondary text
	tnDim    = "#3d4450" // dim / section labels
	tnDimmer = "#2d333b" // even dimmer hints
	tnFaint  = "#58626c" // inline descriptions / json punctuation
	tnGreen  = "#3fb950" // connected / success / sets
	tnRed    = "#f85149" // error / danger / delete
	tnPurple = "#d2a8ff" // lists
	tnOrange = "#ffa657" // hashes / json numbers
	tnYellow = "#e3b341" // zset / scores
	tnBlue   = "#79c0ff" // strings / json keys
	tnInfo   = "#39d353" // INFO command
)

// typeDescStyle returns a lipgloss style for a Redis type label.
// It matches the v2 color coding (hash=orange, list=purple, set=green,
// zset=yellow, string/other=dim).
func typeDescStyle(desc string) lipgloss.Style {
	switch desc {
	case "hash":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(tnOrange))
	case "list":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(tnPurple))
	case "set":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(tnGreen))
	case "zset":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(tnYellow))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	}
}

// fieldDescStyle resolves a field/member list description into its display text
// and color: bare scores (yellow), "field"/"idx N" markers (dim), or a Redis
// type name (color-coded via typeDescStyle).
func fieldDescStyle(desc string) (string, lipgloss.Style) {
	switch {
	case strings.HasPrefix(desc, "score:"):
		return desc[len("score:"):], lipgloss.NewStyle().Foreground(lipgloss.Color(tnYellow))
	case desc == "field" || strings.HasPrefix(desc, "idx "):
		return desc, lipgloss.NewStyle().Foreground(lipgloss.Color(tnDim))
	default:
		return desc, typeDescStyle(desc)
	}
}

// isKeyTypeDesc reports whether desc is one of the bare Redis type names used
// for top-level key rows, as opposed to a field/member descriptor ("field",
// "idx N", "score:N"). Used to gate the TTL badge so it only ever appears on
// keys — which are the only thing ListItem.ttl is populated for.
func isKeyTypeDesc(desc string) bool {
	switch desc {
	case "string", "hash", "list", "set", "zset", "key":
		return true
	}
	return false
}

// formatTTL renders a key's remaining TTL compactly: seconds under a minute,
// otherwise the coarsest useful unit (minutes, hours, or days).
func formatTTL(seconds int) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%dh", seconds/3600)
	default:
		return fmt.Sprintf("%dd", seconds/86400)
	}
}

// helpTextStyle — footer key hints.
var helpTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(tnDim))

type ScanResult struct {
	Cursor string
	Keys   []list.Item
}

type RedisResultMsg struct {
	Result any
	Error  error
}

type RedisTTLResultMsg struct {
	TTL int
}

type ClearCopyStatusMsg struct{}

type RedisConnectionMsg struct {
	Conn  net.Conn
	Error error
	// Fatal indicates a permanent error (wrong password, invalid DB) that
	// should not be retried. The app transitions to StateOutput with the error.
	Fatal bool
}

// A message to tell us the wait time is over
type TickMsg struct{}
