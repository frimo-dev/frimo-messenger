package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type Cipher struct {
	// cipher.AEAD - объект, умеющий:
	//     - шифровать;
	//     - проверять целостность;
	//     - расшифровывать;
	//     - проверять подлинность.
	aead cipher.AEAD
}

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must contain exactly 32 bytes")
	}

	// Создает AES
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	//  Добавляет к AES:
	//  	- безопасный режим для данных произвольной длины;
	//  	- проверку целостности;
	//  	- поддержку AAD.
	// Получается AES-GCM
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}

	return &Cipher{
		aead: aead,
	}, nil
}

func (c *Cipher) Encrypt(plaintext []byte, additionalData []byte) ([]byte, error) {
	// Для AES-GCM стандартный размер nonce 12 байт
	nonce := make([]byte, c.aead.NonceSize())

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate encryption nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, plaintext, additionalData)

	result := make([]byte, 0, len(nonce)+len(ciphertext))
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

func (c *Cipher) Decrypt(value []byte, additionalData []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()

	if len(value) < nonceSize {
		return nil, errors.New("encrypted value is too short")
	}

	nonce := value[:nonceSize]
	ciphertext := value[nonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("decrypt or authenticate encrypted value: %w", err)
	}

	return plaintext, nil
}
