package config

import "testing"

func TestDefaultValueAddr(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/test")
		
	cfg, err := Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Address != ":8080" {
		t.Fatalf("expected address :8080, got %s", cfg.HTTP.Address)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValueAddrFromEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/test")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTP.Address != ":9090" {
		t.Fatalf("expected address :9090, got %s", cfg.HTTP.Address)
	}
}
