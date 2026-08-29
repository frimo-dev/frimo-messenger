package password

import (
	"errors"
	"strings"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/service/auth"
)

func TestArgon2Manager_HashAndVerify(t *testing.T) {
	hasher := NewArgon2Manager()

	const password = "long-secret-password"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == password {
		t.Fatal("password hash equals plaintext password")
	}

	if err := hasher.Verify(hash, password); err != nil {
		t.Fatalf("verify password: %v", err)
	}
}

func TestArgon2Manager_Verify_InvalidPassword(t *testing.T) {
	hasher := NewArgon2Manager()

	hash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	err = hasher.Verify(hash, "wrong-password")

	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got: %v",
			err,
		)
	}
}

func TestArgon2Manager_UsesRandomSalt(t *testing.T) {
	hasher := NewArgon2Manager()

	const password = "long-secret-password"

	first, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("create first hash: %v", err)
	}

	second, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("create second hash: %v", err)
	}

	if first == second {
		t.Fatal("expected different hashes because salts must differ")
	}
}

func TestArgon2Manager_HashFormat(t *testing.T) {
	hasher := NewArgon2Manager()

	hash, err := hasher.Hash("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	const prefix = "$argon2id$v=19$m=65536,t=3,p=2$"

	if !strings.HasPrefix(hash, prefix) {
		t.Fatalf(
			"unexpected hash format: %q",
			hash,
		)
	}
}

func TestDecodeHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{
			name:    "invalid format",
			hash:    "invalid",
			wantErr: true,
		},
		{
			name:    "unsupported algorithm",
			hash:    "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
			wantErr: true,
		},
		{
			name:    "invalid version",
			hash:    "$argon2id$v=abc$m=65536,t=3,p=2$c2FsdA$aGFzaA",
			wantErr: true,
		},
		{
			name:    "unsupported version",
			hash:    "$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA",
			wantErr: true,
		},
		{
			name:    "invalid parameters",
			hash:    "$argon2id$v=19$invalid$c2FsdA$aGFzaA",
			wantErr: true,
		},
		{
			name:    "invalid salt encoding",
			hash:    "$argon2id$v=19$m=65536,t=3,p=2$%%%$aGFzaA",
			wantErr: true,
		},
		{
			name:    "invalid hash encoding",
			hash:    "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$%%%",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := decodeHash(tt.hash)

			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeHash_Valid(t *testing.T) {
	const encodedHash = "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA"

	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		t.Fatalf("decode hash: %v", err)
	}

	if params.memory != 65536 {
		t.Errorf(
			"memory: expected %d, got %d",
			65536,
			params.memory,
		)
	}

	if params.iterations != 3 {
		t.Errorf(
			"iterations: expected %d, got %d",
			3,
			params.iterations,
		)
	}

	if params.parallelism != 2 {
		t.Errorf(
			"parallelism: expected %d, got %d",
			2,
			params.parallelism,
		)
	}

	if string(salt) != "salt" {
		t.Errorf(
			"salt: expected %q, got %q",
			"salt",
			salt,
		)
	}

	if string(hash) != "hash" {
		t.Errorf(
			"hash: expected %q, got %q",
			"hash",
			hash,
		)
	}
}

func TestArgon2Manager_Verify_InvalidHash(t *testing.T) {
	hasher := NewArgon2Manager()

	err := hasher.Verify("not-an-argon2-hash", "password")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf(
			"expected hash decoding error, got invalid credentials: %v",
			err,
		)
	}
}
