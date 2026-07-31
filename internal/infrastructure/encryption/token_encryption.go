package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// TokenEncryptor handles encryption and decryption of sensitive tokens
// It uses AES-GCM for authenticated encryption
// The implementation is isolated to allow replacement with a secret manager later
type TokenEncryptor struct {
	key []byte
}

// NewTokenEncryptor creates a new token encryptor with the given encryption key
// The key must be 32 bytes for AES-256
func NewTokenEncryptor(key []byte) (*TokenEncryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes (AES-256)")
	}

	return &TokenEncryptor{
		key: key,
	}, nil
}

// Encrypt encrypts a plaintext token and returns a base64-encoded ciphertext
// The format is: base64(nonce + ciphertext) where nonce is 12 bytes
func (e *TokenEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create a nonce (12 bytes for GCM)
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt the plaintext
	ciphertext := aead.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64-encoded ciphertext (includes nonce)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext and returns the plaintext
func (e *TokenEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertextBytes
	nonce := data[:nonceSize]
	ciphertextBytes := data[nonceSize:]

	// Decrypt
	plaintext, err := aead.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
