package signing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2Iterations = 100_000
	saltSize         = 16
	keySize          = 32 // AES-256
)

// Encrypt encrypts plaintext using AES-256-GCM with a password-derived key.
// Output format: salt (16 bytes) || nonce (12 bytes) || ciphertext+tag.
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// salt || nonce || ciphertext+tag
	result := make([]byte, 0, saltSize+len(nonce)+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// Decrypt decrypts data encrypted by Encrypt.
func Decrypt(data []byte, password string) ([]byte, error) {
	if len(data) < saltSize+12 { // salt + minimum nonce
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := data[:saltSize]
	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keySize, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < saltSize+nonceSize {
		return nil, fmt.Errorf("encrypted data too short for nonce")
	}

	nonce := data[saltSize : saltSize+nonceSize]
	ciphertext := data[saltSize+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed (wrong password?): %w", err)
	}

	return plaintext, nil
}
