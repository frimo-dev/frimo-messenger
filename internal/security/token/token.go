package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenSize = 32

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate() (string, []byte, error) {
	randomBytes := make([]byte, tokenSize)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("generate random token: %w", err)
	}

	// base64.RawURLEncoding - не использует проблемные для URL символы + и /,
	// не добавляет padding =, удобно передаётся как query parameter
	rawToken := base64.RawURLEncoding.EncodeToString(randomBytes)
	tokenHash := Hash(rawToken)

	return rawToken, tokenHash, nil
}

func Hash(rawToken string) []byte {
	sum := sha256.Sum256([]byte(rawToken))
	return sum[:]
}
