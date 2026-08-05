package token_test

import (
	"bytes"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/security/token"
)

func TestGeneratorCreatesDifferentTokens(t *testing.T) {
	generator := token.NewGenerator()

	firstRaw, firstHash, err := generator.Generate()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}

	secondRaw, secondHash, err := generator.Generate()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}

	if firstRaw == secondRaw {
		t.Fatal("expected different raw tokens")
	}

	if bytes.Equal(firstHash, secondHash) {
		t.Fatal("expected different token hashes")
	}
}

func TestHashIsDeterministic(t *testing.T) {
	first := token.Hash("example-token")
	second := token.Hash("example-token")

	if !bytes.Equal(first, second) {
		t.Fatal("same token must produce same hash")
	}
}
