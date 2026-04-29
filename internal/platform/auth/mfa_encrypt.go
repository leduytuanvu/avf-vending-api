package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// EncryptMFASecret seals plaintext TOTP seed bytes using AES-256-GCM (nonce prefix || ciphertext).
func EncryptMFASecret(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("auth: MFA encryption key must be 32 bytes")
	}
	if len(plaintext) == 0 {
		return nil, errors.New("auth: empty MFA secret")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptMFASecret reverses EncryptMFASecret.
func DecryptMFASecret(key, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("auth: MFA encryption key must be 32 bytes")
	}
	if len(ciphertext) < 12 {
		return nil, errors.New("auth: malformed MFA ciphertext")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("auth: malformed MFA ciphertext")
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}
