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

	// Build menu items
	items := []list.Item{
		tui.NewListItem("SET", "Set a key-value pair"),
		tui.NewListItem("GET", "Get the value of a key"),
		tui.NewListItem("HSET", "Set a hash field"),
		tui.NewListItem("HGET", "Get the value of a hash field"),
		tui.NewListItem("RPUSH", "Add value to the end of a list"),
		tui.NewListItem("LPUSH", "Add value to the beginning of a list"),
		tui.NewListItem("SADD", "Add value to a set"),
		tui.NewListItem("ZADD", "Add value to a sorted set"),
		tui.NewListItem("DELETE", "Delete a key-value pair or an entire hash"),
		tui.NewListItem("EXPLORE", "Browse keys and values"),
		tui.NewListItem("INFO", "View Redis server statistics"),
		tui.NewListItem("EXPORT", "Export a key to a file via DUMP"),
		tui.NewListItem("IMPORT", "Import a key from a file via RESTORE"),
		tui.NewListItem("EXPORT_DB", "Export the entire database to a JSON file"),
		tui.NewListItem("IMPORT_DB", "Import the entire database from a JSON file"),
	}

	menuList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menuList.Title = "Redis TUI"
	menuList.FilterInput.Placeholder = "type to search..."

	fieldsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	fieldsList.Title = "Select a field"

	keyList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	keyList.Title = "Select a key"

	input := textarea.New()
	input.ShowLineNumbers = false
	input.Prompt = ""

	initialModel := tui.Model{
		CurrentState: tui.StateMenu,
		MenuList:     menuList,
		Browser: tui.BrowserModel{
			KeyList:       keyList,
			FieldsList:    fieldsList,
			ViewingFields: false,
		},
		Spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
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

	p := tea.NewProgram(initialModel)
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
