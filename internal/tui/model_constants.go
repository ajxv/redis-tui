package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"net"
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
)

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

var statusTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#04B575")). // A nice bright green
	Bold(true)

var warningStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("196")).
	Padding(0, 1)

var helpTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("241")) // A subtle gray

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
}

// A message to tell us the wait time is over
type TickMsg struct{}
