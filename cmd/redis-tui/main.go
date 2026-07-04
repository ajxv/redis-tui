package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajxv/redis-tui/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func run() error {
	versionFlag := flag.Bool("version", false, "Print version and exit")

	// Connection flags
	host := flag.String("host", "localhost:6379", "Redis server host:port")
	password := flag.String("password", os.Getenv("REDIS_PASSWORD"), "Redis password (default: $REDIS_PASSWORD)")
	username := flag.String("username", "", "Redis ACL username (Redis 6+)")
	db := flag.Int("db", 0, "Redis database index")
	redisURL := flag.String("url", "", "Redis URL: redis://[:pass@]host[:port][/db] or rediss://...")
	dialTimeout := flag.Duration("dial-timeout", 5*time.Second, "TCP connection timeout (e.g. 5s, 500ms)")
	readTimeout := flag.Duration("read-timeout", 10*time.Second, "Redis read deadline per command")

	// TLS flags
	tlsEnabled := flag.Bool("tls", false, "Enable TLS/SSL")
	tlsSkipVerify := flag.Bool("tls-skip-verify", false, "Skip TLS certificate verification (insecure)")
	tlsCert := flag.String("tls-cert", "", "Path to client TLS certificate (PEM)")
	tlsKey := flag.String("tls-key", "", "Path to client TLS private key (PEM)")
	tlsCA := flag.String("tls-ca", "", "Path to CA certificate (PEM)")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("redis-tui %s\n", version)
		return nil
	}

	// URL overrides individual flags when provided
	if *redisURL != "" {
		parsed, err := tui.ParseRedisURL(*redisURL)
		if err != nil {
			fmt.Printf("Invalid Redis URL: %v\n", err)
			return err
		}
		*host = parsed.Host
		if parsed.Password != "" {
			*password = parsed.Password
		}
		if parsed.Username != "" {
			*username = parsed.Username
		}
		*db = parsed.DB
		if parsed.TLS {
			*tlsEnabled = true
		}
	}

	// Build TLS config (nil when TLS is disabled — plain TCP)
	tlsCfg, err := tui.BuildTLSConfig(*tlsEnabled, *tlsSkipVerify, *tlsCert, *tlsKey, *tlsCA)
	if err != nil {
		fmt.Printf("TLS config error: %v\n", err)
		return err
	}

	// Go's tls.Client requires either ServerName or InsecureSkipVerify.
	// Derive ServerName from the host address so certificate hostname
	// validation works out of the box without the user having to specify it.
	if tlsCfg != nil && !tlsCfg.InsecureSkipVerify && tlsCfg.ServerName == "" {
		if hostname, _, herr := net.SplitHostPort(*host); herr == nil && hostname != "" {
			tlsCfg.ServerName = hostname
		}
	}

	// Fail-fast connectivity pre-check
	rawConn, err := net.DialTimeout("tcp", *host, *dialTimeout)
	if err != nil {
		fmt.Printf("Connection error: %v\n", err)
		return err
	}
	if tlsCfg != nil {
		tc := tls.Client(rawConn, tlsCfg)
		if err := tc.Handshake(); err != nil {
			_ = rawConn.Close()
			fmt.Printf("TLS handshake error: %v\n", err)
			return err
		}
		_ = tc.Close() // closes the TLS layer and the underlying rawConn
	} else {
		_ = rawConn.Close()
	}

	// Build grouped menu — EXPLORE is promoted to the top, commands grouped by type.
	// The group label is embedded in the first item of each section; there are no
	// separate non-selectable header items, so the cursor always lands on a command.
	items := []list.Item{
		tui.NewListItem("EXPLORE", "Scan, filter, inspect, edit and delete keys"),
		tui.NewListItemInGroup("SET", "Set a key-value pair", "STRINGS"),
		tui.NewListItem("GET", "Get the value of a key"),
		tui.NewListItemInGroup("HSET", "Set a hash field", "HASHES"),
		tui.NewListItem("HGET", "Get the value of a hash field"),
		tui.NewListItemInGroup("RPUSH", "Append a value to the end of a list", "LISTS"),
		tui.NewListItem("LPUSH", "Prepend a value to the start of a list"),
		tui.NewListItemInGroup("SADD", "Add a member to a set", "SETS & SORTED SETS"),
		tui.NewListItem("ZADD", "Add a scored member to a sorted set"),
		tui.NewListItemInGroup("DELETE", "Delete a key", "MANAGE"),
		tui.NewListItem("EXPORT", "Dump a key to a file (DUMP)"),
		tui.NewListItem("IMPORT", "Restore a key from a file (RESTORE)"),
		tui.NewListItem("EXPORT_DB", "Export the entire database to JSON"),
		tui.NewListItem("IMPORT_DB", "Import the entire database from JSON"),
		tui.NewListItemInGroup("INFO", "View Redis server statistics", "SERVER"),
	}

	menuList := list.New(items, tui.NewGroupedMenuDelegate(), 0, 0)
	menuList.Title = "Select a command"
	tui.StyleList(&menuList)
	menuList.FilterInput.Placeholder = "type to filter commands…"

	fieldsList := list.New([]list.Item{}, tui.BrowserDelegate(), 0, 0)
	fieldsList.Title = "Select a field"
	tui.StyleList(&fieldsList)

	keyList := list.New([]list.Item{}, tui.BrowserDelegate(), 0, 0)
	keyList.Title = "Select a key"
	tui.StyleList(&keyList)

	input := textarea.New()
	input.ShowLineNumbers = false
	// Prompt/cursor styling is applied per-render in InputModel.View().

	// textinput instances for the add-field overlay in the browser.
	fieldInput := textinput.New()
	fieldInput.Placeholder = "field name"
	fieldInput.CharLimit = 512

	valueInput := textinput.New()
	valueInput.Placeholder = "value"
	valueInput.CharLimit = 4096

	initialModel := tui.Model{
		CurrentState: tui.StateMenu,
		MenuList:     menuList,
		Help:         tui.NewHelp(),
		Browser: tui.BrowserModel{
			KeyList:    keyList,
			FieldsList: fieldsList,
			FieldInput: fieldInput,
			ValueInput: valueInput,
			Help:       tui.NewHelp(),
		},
		Spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		Viewport: viewport.New(0, 0),
		Input: tui.InputModel{
			Input: input,
		},
		RedisAddress: *host,
		Password:     *password,
		Username:     *username,
		DB:           *db,
		TLSConfig:    tlsCfg,
		DialTimeout:  *dialTimeout,
		ReadTimeout:  *readTimeout,
	}

	p := tea.NewProgram(initialModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("An error occurred: %v\n", err)
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
