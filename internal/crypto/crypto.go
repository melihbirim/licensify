package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2 parameters (recommended for password hashing and key derivation)
const (
	Argon2Time    = 3         // Number of iterations
	Argon2Memory  = 64 * 1024 // Memory cost in KiB (64 MB)
	Argon2Threads = 4         // Parallelism
	Argon2KeyLen  = 32        // Output key length (AES-256)
	SaltSize      = 32        // 256-bit salt
)

// DeriveKey uses Argon2id to derive encryption key from license, hardware ID, and salt
func DeriveKey(licenseKey, hardwareID, salt string) []byte {
	// Combine license key and hardware ID as the "password"
	password := []byte(licenseKey + ":" + hardwareID)
	saltBytes, _ := hex.DecodeString(salt)

	// If salt decode fails (legacy), use salt as-is
	if len(saltBytes) == 0 {
		saltBytes = []byte(salt)
	}

	return argon2.IDKey(password, saltBytes, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)
}

// GenerateSalt creates a cryptographically secure random salt
func GenerateSalt() (string, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return hex.EncodeToString(salt), nil
}

// EncryptAESGCM encrypts plaintext using AES-256-GCM with the provided key
// Returns base64-encoded ciphertext and IV (nonce)
func EncryptAESGCM(plaintext []byte, key []byte) (ciphertext, iv string, err error) {
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertextBytes := gcm.Seal(nil, nonce, plaintext, nil)

	// Encode to base64
	ciphertext = base64.StdEncoding.EncodeToString(ciphertextBytes)
	iv = base64.StdEncoding.EncodeToString(nonce)

	return ciphertext, iv, nil
}

// DecryptAESGCM decrypts base64-encoded ciphertext using AES-256-GCM with the provided key and IV
func DecryptAESGCM(ciphertext, iv string, key []byte) ([]byte, error) {
	// Validate inputs
	if ciphertext == "" {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}
	if iv == "" {
		return nil, fmt.Errorf("IV cannot be empty")
	}

	// Decode from base64
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonceBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Validate nonce size
	if len(nonceBytes) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(nonceBytes), gcm.NonceSize())
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonceBytes, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// ComputeHMAC computes HMAC-SHA256 signature for the message using the provided key
// Returns hex-encoded signature
func ComputeHMAC(key, message []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(message)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateHMAC validates an HMAC-SHA256 signature using constant-time comparison
func ValidateHMAC(key, message []byte, signature string) bool {
	expectedSignature := ComputeHMAC(key, message)
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// RedactPII returns a redacted version of sensitive data for logging
// Shows first 4 and last 4 characters for identification without full exposure
func RedactPII(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// RedactEmail redacts email addresses for logging
func RedactEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		return "***@" + domain
	}
	return username[:min(2, len(username))] + "***@" + domain
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
