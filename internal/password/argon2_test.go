package password

import "testing"

func TestArgon2Hasher(t *testing.T) {
	hasher := NewArgon2Hasher()

	hash, err := hasher.Hash("long-secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == "long-secret-password" {
		t.Fatal("password hash equals plaintext password")
	}

	matches, err := hasher.Compare(
		"long-secret-password",
		hash,
	)
	if err != nil {
		t.Fatalf("compare password: %v", err)
	}

	if !matches {
		t.Fatal("expected password to match")
	}

	matches, err = hasher.Compare(
		"wrong-password",
		hash,
	)
	if err != nil {
		t.Fatalf("compare wrong password: %v", err)
	}

	if matches {
		t.Fatal("wrong password must not match")
	}
}

func TestArgon2HasherUsesRandomSalt(t *testing.T) {
	hasher := NewArgon2Hasher()

	first, err := hasher.Hash("long-secret-password")
	if err != nil {
		t.Fatalf("create first hash: %v", err)
	}

	second, err := hasher.Hash("long-secret-password")
	if err != nil {
		t.Fatalf("create second hash: %v", err)
	}

	if first == second {
		t.Fatal("expected different hashes because salts must differ")
	}
}
