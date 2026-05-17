package tui

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParsedURL holds the connection parameters extracted from a Redis URL.
type ParsedURL struct {
	Host     string
	Password string
	Username string
	DB       int
	TLS      bool
}

// ParseRedisURL parses a Redis connection URL of the form:
//
//	redis://[:password@]host[:port][/db]
//	rediss://[:password@]host[:port][/db]   (TLS implied)
//
// The port defaults to 6379 when omitted. The db defaults to 0 when omitted.
func ParseRedisURL(rawURL string) (ParsedURL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ParsedURL{}, fmt.Errorf("invalid URL: %w", err)
	}

	var result ParsedURL

	switch u.Scheme {
	case "redis":
		result.TLS = false
	case "rediss":
		result.TLS = true
	default:
		return ParsedURL{}, fmt.Errorf("unsupported scheme %q: use redis:// or rediss://", u.Scheme)
	}

	if u.Host == "" {
		return ParsedURL{}, fmt.Errorf("missing host in Redis URL")
	}

	// Resolve host:port — append default port when missing
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		// SplitHostPort fails when there is no port
		host = u.Host
		port = "6379"
	}
	if port == "" {
		port = "6379"
	}
	result.Host = net.JoinHostPort(host, port)

	// Auth credentials
	if u.User != nil {
		result.Username = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			result.Password = pw
		}
	}

	// Database index from path (e.g. "/2")
	dbStr := strings.TrimPrefix(u.Path, "/")
	if dbStr == "" {
		result.DB = 0
	} else {
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return ParsedURL{}, fmt.Errorf("invalid database index %q: must be a non-negative integer", dbStr)
		}
		if db < 0 {
			return ParsedURL{}, fmt.Errorf("invalid database index %d: must be non-negative", db)
		}
		result.DB = db
	}

	return result, nil
}
