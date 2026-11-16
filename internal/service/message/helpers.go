package message

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
	"uuid"
)

const maxMessageLength = 4096

func normalizeMessageText(text string) string {
	return strings.TrimSpace(text)
}

func validateMessageText(text string) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("%w: invalid UTF-8", ErrInvalidMessageText)
	}

	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: text cannot be empty", ErrInvalidMessageText)
	}

	if utf8.RuneCountInString(text) > maxMessageLength {
		return fmt.Errorf("%w: text exceeds %d characters", ErrInvalidMessageText, maxMessageLength)
	}

	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return fmt.Errorf("%w: control characters are not allowed", ErrInvalidMessageText)
		}
	}

	return nil
}

func calculateRequestHash(recipientID uuid.UUID, text string) [32]byte {
	h := sha256.New()

	h.Write([]byte("direct-message:v1"))
	h.Write(recipientID[:])
	h.Write([]byte(text))

	var result [32]byte
	copy(result[:], h.Sum(nil))

	return result
}
