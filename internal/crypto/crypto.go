package crypto

import (
"crypto/aes"
"crypto/cipher"
"crypto/ed25519"
"crypto/hmac"
"crypto/rand"
"crypto/sha256"
"encoding/hex"
"fmt"
"math/big"
"os"
"time"

"golang.org/x/crypto/argon2"
)

const (
Argon2Time      = 3
Argon2Memory    = 64 * 1024
Argon2Threads   = 4
Argon2KeyLength = 32
SaltLength      = 32
HMACReplayWindow = 5 * 60
)

func LoadOrGeneratePrivateKey(path string) (ed25519.PrivateKey, error) {
	if data, err := os.ReadFile(path); err == nil {
		if len(data) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(data), nil
		}
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	if err := os.WriteFile(path, privateKey, 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}
	return privateKey, nil
}

func GenerateSalt() (string, error) {
	salt := make([]byte, SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return hex.EncodeToString(salt), nil
}

func DeriveKey(licenseKey, hardwareID, salt string) ([]byte, error) {
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}
	combined := []byte(licenseKey + hardwareID)
	key := argon2.IDKey(combined, saltBytes, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLength)
	return key, nil
}

func EncryptData(key []byte, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

func DecryptData(key []byte, encryptedHex string) ([]byte, error) {
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

func SignLicense(privateKey ed25519.PrivateKey, licenseKey, email, expiresAt string) string {
	message := licenseKey + email + expiresAt
	signature := ed25519.Sign(privateKey, []byte(message))
	return hex.EncodeToString(signature)
}

func GenerateLicenseKey() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const keyLength = 25
	const groupSize = 5
	key := make([]byte, keyLength)
	for i := range key {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		key[i] = charset[n.Int64()]
	}
	var formatted string
	for i := 0; i < keyLength; i += groupSize {
		if i > 0 {
			formatted += "-"
		}
		end := i + groupSize
		if end > keyLength {
			end = keyLength
		}
		formatted += string(key[i:end])
	}
	return formatted, nil
}

func ValidateProxySignature(proxyKey, provider string, body []byte, timestamp int64, signature string) bool {
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	if diff > HMACReplayWindow {
		return false
	}
	message := fmt.Sprintf("%d%s%s", timestamp, provider, string(body))
	h := hmac.New(sha256.New, []byte(proxyKey))
	h.Write([]byte(message))
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

func GenerateProxyKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate proxy key: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func RedactPII(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func RedactEmail(email string) string {
	parts := []byte(email)
	atIndex := -1
	for i, c := range parts {
		if c == '@' {
			atIndex = i
			break
		}
	}
	if atIndex == -1 || atIndex == 0 {
		return "***@***"
	}
	redactStart := 2
	if atIndex < redactStart {
		redactStart = atIndex
	}
	for i := redactStart; i < atIndex; i++ {
		parts[i] = '*'
	}
	return string(parts)
}
