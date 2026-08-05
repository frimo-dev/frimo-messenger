package secret_test

import (
	"bytes"
	"testing"

	"github.com/frimo-dev/frimo-messenger/internal/security/secret"
)

func TestCipherEncryptsAndDecryptsValue(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)

	tokenCipher, err := secret.NewCipher(key)
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	plaintext := []byte("raw-verification-token")
	additionalData := []byte("verification-id")

	encrypted, err := tokenCipher.Encrypt(
		plaintext,
		additionalData,
	)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}

	decrypted, err := tokenCipher.Decrypt(
		encrypted,
		additionalData,
	)
	if err != nil {
		t.Fatalf("decrypt token: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf(
			"expected plaintext %q, got %q",
			plaintext,
			decrypted,
		)
	}
}

func TestCipherRejectsDifferentAdditionalData(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)

	tokenCipher, err := secret.NewCipher(key)
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	encrypted, err := tokenCipher.Encrypt(
		[]byte("raw-verification-token"),
		[]byte("verification-id-1"),
	)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}

	_, err = tokenCipher.Decrypt(
		encrypted,
		[]byte("verification-id-2"),
	)
	if err == nil {
		t.Fatal("expected authentication error")
	}
}

func TestCipherUsesRandomNonce(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)

	tokenCipher, err := secret.NewCipher(key)
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	first, err := tokenCipher.Encrypt(
		[]byte("same-value"),
		[]byte("verification-id"),
	)
	if err != nil {
		t.Fatalf("first encryption: %v", err)
	}

	second, err := tokenCipher.Encrypt(
		[]byte("same-value"),
		[]byte("verification-id"),
	)
	if err != nil {
		t.Fatalf("second encryption: %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("expected different ciphertexts")
	}
}
