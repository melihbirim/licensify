package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}

	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}

	// Salts should be different
	if salt1 == salt2 {
		t.Error("GenerateSalt produced identical salts")
	}

	// Salt should be correct length (32 bytes = 64 hex chars)
	if len(salt1) != 64 {
		t.Errorf("Salt length incorrect: got %d, want 64", len(salt1))
	}

	// Salt should be valid hex
	if _, err := hex.DecodeString(salt1); err != nil {
		t.Errorf("Salt is not valid hex: %v", err)
	}
}

func TestDeriveKey(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}

	tests := []struct {
		name       string
		licenseKey string
		hardwareID string
		salt       string
		wantErr    bool
	}{
		{
			name:       "valid inputs",
			licenseKey: "TEST-12345-ABCDE-67890",
			hardwareID: "hardware123",
			salt:       salt,
			wantErr:    false,
		},
		{
			name:       "invalid salt",
			licenseKey: "TEST-12345",
			hardwareID: "hardware123",
			salt:       "invalid-hex",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DeriveKey(tt.licenseKey, tt.hardwareID, tt.salt)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeriveKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(key) != Argon2KeyLength {
					t.Errorf("Key length incorrect: got %d, want %d", len(key), Argon2KeyLength)
				}

				// Same inputs should produce same key
				key2, err := DeriveKey(tt.licenseKey, tt.hardwareID, tt.salt)
				if err != nil {
					t.Fatalf("Second DeriveKey failed: %v", err)
				}
				if hex.EncodeToString(key) != hex.EncodeToString(key2) {
					t.Error("Same inputs produced different keys")
				}
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate a key
	salt, _ := GenerateSalt()
	key, err := DeriveKey("test-license", "test-hardware", salt)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"short text", "Hello, World!"},
		{"long text", "This is a much longer text that spans multiple blocks."},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := EncryptData(key, []byte(tt.plaintext))
			if err != nil {
				t.Fatalf("EncryptData failed: %v", err)
			}

			// Decrypt
			decrypted, err := DecryptData(key, encrypted)
			if err != nil {
				t.Fatalf("DecryptData failed: %v", err)
			}

			if string(decrypted) != tt.plaintext {
				t.Errorf("Decrypted text doesn't match: got %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestValidateProxySignature(t *testing.T) {
	proxyKey := "test-proxy-key-12345"
	provider := "openai"
	body := []byte(`{"model":"gpt-4"}`)

	tests := []struct {
		name      string
		timestamp int64
		want      bool
	}{
		{"current time", time.Now().Unix(), true},
		{"1 minute ago", time.Now().Unix() - 60, true},
		{"4 minutes ago", time.Now().Unix() - 240, true},
		{"6 minutes ago", time.Now().Unix() - 360, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate valid signature for this timestamp
			message := fmt.Sprintf("%d%s%s", tt.timestamp, provider, string(body))
			h := hmac.New(sha256.New, []byte(proxyKey))
			h.Write([]byte(message))
			signature := hex.EncodeToString(h.Sum(nil))

			got := ValidateProxySignature(proxyKey, provider, body, tt.timestamp, signature)
			if got != tt.want {
				t.Errorf("ValidateProxySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedactPII(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal", "ABCD-1234-5678-EFGH", "ABCD...EFGH"},
		{"short", "SHORT", "***"},
		{"empty", "", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactPII(tt.input)
			if got != tt.want {
				t.Errorf("RedactPII(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadOrGeneratePrivateKey(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test.key")

	// First call should generate new key
	key1, err := LoadOrGeneratePrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrGeneratePrivateKey failed: %v", err)
	}

	// Check file was created
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file was not created")
	}

	// Second call should load existing key
	key2, err := LoadOrGeneratePrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrGeneratePrivateKey failed on second call: %v", err)
	}

	// Keys should be identical
	if !key1.Equal(key2) {
		t.Error("Loaded key doesn't match generated key")
	}
}
