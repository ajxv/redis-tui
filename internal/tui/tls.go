package tui

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildTLSConfig returns a *tls.Config ready for use with tls.Client, or nil
// when TLS is disabled (plain TCP path). Returns an error if certificate
// files are specified but cannot be loaded.
func BuildTLSConfig(enabled bool, skipVerify bool, certFile, keyFile, caFile string) (*tls.Config, error) {
	if !enabled {
		return nil, nil
	}

	cfg := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	// Load client certificate / key pair for mTLS
	if (certFile != "") != (keyFile != "") {
		return nil, fmt.Errorf("both -tls-cert and -tls-key must be provided together")
	}
	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	// Load custom CA certificate pool
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caFile)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}
