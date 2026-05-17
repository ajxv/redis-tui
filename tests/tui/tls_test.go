package tui_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajxv/redis-tui/internal/tui"
)

func TestBuildTLSConfig_Disabled(t *testing.T) {
	cfg, err := tui.BuildTLSConfig(false, false, "", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config when TLS disabled, got non-nil")
	}
}

func TestBuildTLSConfig_SkipVerify(t *testing.T) {
	cfg, err := tui.BuildTLSConfig(true, true, "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestBuildTLSConfig_EnabledNoSkipVerify(t *testing.T) {
	cfg, err := tui.BuildTLSConfig(true, false, "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=false")
	}
}

func TestBuildTLSConfig_BadCertPath(t *testing.T) {
	_, err := tui.BuildTLSConfig(true, false, "/nonexistent/cert.pem", "/nonexistent/key.pem", "")
	if err == nil {
		t.Fatal("expected error for bad cert path, got nil")
	}
}

func TestBuildTLSConfig_CertWithoutKey(t *testing.T) {
	_, err := tui.BuildTLSConfig(true, false, "/some/cert.pem", "", "")
	if err == nil {
		t.Fatal("expected error when cert provided without key")
	}
}

func TestBuildTLSConfig_KeyWithoutCert(t *testing.T) {
	_, err := tui.BuildTLSConfig(true, false, "", "/some/key.pem", "")
	if err == nil {
		t.Fatal("expected error when key provided without cert")
	}
}

func TestBuildTLSConfig_BadCAPath(t *testing.T) {
	_, err := tui.BuildTLSConfig(true, false, "", "", "/nonexistent/ca.pem")
	if err == nil {
		t.Fatal("expected error for bad CA path, got nil")
	}
}

func TestBuildTLSConfig_ValidCAFile(t *testing.T) {
	// Generate a self-signed cert and write it as a CA file
	caFile := writeSelfSignedCert(t)

	cfg, err := tui.BuildTLSConfig(true, false, "", "", caFile)
	if err != nil {
		t.Fatalf("unexpected error with valid CA file: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.RootCAs == nil {
		t.Error("expected non-nil RootCAs pool")
	}
}

// writeSelfSignedCert generates a minimal self-signed PEM certificate, writes
// it to a temp file, and returns the path. The file is cleaned up via t.Cleanup.
func writeSelfSignedCert(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	f, err := os.Create(caFile)
	if err != nil {
		t.Fatalf("create CA file: %v", err)
	}
	defer f.Close()

	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("pem encode: %v", err)
	}

	return caFile
}
