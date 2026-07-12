package config

import "testing"

func TestDefaultValueAddr(t *testing.T) {
	cfg, err := Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Address != ":8080" {
		t.Fatalf("expected address :8080, got %s", cfg.HTTP.Address)
	}
}

func TestValueAddrFromEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Address != ":9090" {
		t.Fatalf("expected address :9090, got %s", cfg.HTTP.Address)
	}
}
