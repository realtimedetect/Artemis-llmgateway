package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"strings"
)

const secretPrefix = "enc:v1:"

func secretKey() []byte {
	if raw := strings.TrimSpace(os.Getenv("DATA_ENCRYPTION_KEY")); raw != "" {
		sum := sha256.Sum256([]byte(raw))
		return sum[:]
	}
	// Fallback to JWT secret so encryption remains available even without an explicit data key.
	sum := sha256.Sum256([]byte(os.Getenv("JWT_SECRET")))
	return sum[:]
}

func encryptSecret(plain string) (string, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", nil
	}
	if strings.HasPrefix(plain, secretPrefix) {
		return plain, nil
	}

	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, sealed...)
	return secretPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func decryptSecret(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, secretPrefix) {
		// Backward compatibility for existing plaintext values.
		return stored, nil
	}

	encoded := strings.TrimPrefix(stored, secretPrefix)
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", io.ErrUnexpectedEOF
	}
	nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func encryptSecrets(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		enc, err := encryptSecret(value)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(enc) == "" {
			continue
		}
		out = append(out, enc)
	}
	return out, nil
}
