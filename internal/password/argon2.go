package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memory      = 64 * 1024
	iterations  = 3
	parallelism = 2
	saltLength  = 16
	keyLength   = 32
)

type parameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

type Argon2Hasher struct{}

func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{}
}

func (h *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLength)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		encodedSalt,
		encodedHash,
	)

	return encoded, nil
}

func (h *Argon2Hasher) Compare(password string, encodedHash string) (bool, error) {
	parameters, salt, expectedHash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		parameters.iterations,
		parameters.memory,
		parameters.parallelism,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}

func decodeHash(encodedHash string) (parameters, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 {
		return parameters{}, nil, nil, errors.New(
			"invalid encoded password hash",
		)
	}

	if parts[1] != "argon2id" {
		return parameters{}, nil, nil, errors.New(
			"unsupported password hash algorithm",
		)
	}

	var version int

	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return parameters{}, nil, nil, fmt.Errorf(
			"parse argon2 version: %w",
			err,
		)
	}

	if version != argon2.Version {
		return parameters{}, nil, nil, errors.New(
			"unsupported argon2 version",
		)
	}

	var params parameters

	if _, err := fmt.Sscanf(
		parts[3],
		"m=%d,t=%d,p=%d",
		&params.memory,
		&params.iterations,
		&params.parallelism,
	); err != nil {
		return parameters{}, nil, nil, fmt.Errorf(
			"parse argon2 parameters: %w",
			err,
		)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return parameters{}, nil, nil, fmt.Errorf(
			"decode salt: %w",
			err,
		)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return parameters{}, nil, nil, fmt.Errorf(
			"decode password hash: %w",
			err,
		)
	}

	return params, salt, hash, nil
}
