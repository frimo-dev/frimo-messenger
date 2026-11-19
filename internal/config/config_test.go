package config

import (
	"encoding/base64"
	"os"
	"testing"
)

func setValidConfigEnv(t *testing.T) {
	t.Helper()

	testSecret := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/test")
	t.Setenv("APP_BASE_URL", "http://localhost")
	t.Setenv("VERIFICATION_TOKEN_ENCRYPTION_KEY", testSecret)
	t.Setenv("ACCESS_TOKEN_SECRET", testSecret)
}

func TestDefaultValueAddr(t *testing.T) {
	setValidConfigEnv(t)
	// Setenv in setValidConfigEnv restores the original value after the test.
	if err := os.Unsetenv("HTTP_ADDR"); err != nil {
		t.Fatalf("unset HTTP_ADDR: %v", err)
	}

	cfg, err := Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Address != ":8080" {
		t.Fatalf("expected address :8080, got %s", cfg.HTTP.Address)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	want := "DATABASE_URL is required"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestValueAddrFromEnvironment(t *testing.T) {
	setValidConfigEnv(t)
	t.Setenv("HTTP_ADDR", ":9090")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Address != ":9090" {
		t.Fatalf("expected address :9090, got %s", cfg.HTTP.Address)
	}
}
