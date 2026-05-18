package mitm

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"
)

func TestLoadOrCreate(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if ca == nil {
		t.Fatal("expected non-nil CA")
	}
}

func TestLoadOrCreateIdempotent(t *testing.T) {
	dir := t.TempDir()
	ca1, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("first LoadOrCreate: %v", err)
	}
	ca2, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("second LoadOrCreate: %v", err)
	}
	// Both should produce the same cert serial.
	if ca1.cert.SerialNumber.Cmp(ca2.cert.SerialNumber) != 0 {
		t.Error("expected same CA serial on reload")
	}
}

func TestCertPath(t *testing.T) {
	dir := t.TempDir()
	LoadOrCreate(dir) //nolint:errcheck
	if _, err := os.Stat(CertPath(dir)); err != nil {
		t.Errorf("cert file not created: %v", err)
	}
}

func TestTLSConfigFor(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	cfg, err := ca.TLSConfigFor("example.com")
	if err != nil {
		t.Fatalf("TLSConfigFor: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
	}
}

func TestTLSConfigForCached(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	cfg1, _ := ca.TLSConfigFor("example.com")
	cfg2, _ := ca.TLSConfigFor("example.com")
	// Same underlying DER bytes means cache hit (not new cert generation).
	leaf1 := cfg1.Certificates[0].Certificate[0]
	leaf2 := cfg2.Certificates[0].Certificate[0]
	if string(leaf1) != string(leaf2) {
		t.Error("expected same leaf cert DER on second call (cache hit)")
	}
}

func TestGeneratedCertSignedByCA(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	cfg, err := ca.TLSConfigFor("test.example.com")
	if err != nil {
		t.Fatalf("TLSConfigFor: %v", err)
	}

	// Build a cert pool with the CA and verify the leaf cert.
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)

	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf cert: %v", err)
	}
	_, err = leaf.Verify(x509.VerifyOptions{
		DNSName: "test.example.com",
		Roots:   pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		t.Errorf("cert verification failed: %v", err)
	}
}

func TestTLSConfigMinVersion(t *testing.T) {
	dir := t.TempDir()
	ca, _ := LoadOrCreate(dir)
	cfg, _ := ca.TLSConfigFor("example.com")
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("expected TLS 1.2 minimum, got %d", cfg.MinVersion)
	}
}
