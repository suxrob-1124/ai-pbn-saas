package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// DeriveKey нормализует произвольную строку до 32-байтового ключа.
func DeriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encrypt шифрует данные при помощи AES-GCM и добавляет nonce в начало результата.
func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("empty key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm init: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, len(nonce)+len(ciphertext))
	copy(out, nonce)
	copy(out[len(nonce):], ciphertext)
	return out, nil
}

// Decrypt расшифровывает данные, полученные Encrypt.
func Decrypt(key []byte, data []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("empty key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm init: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
