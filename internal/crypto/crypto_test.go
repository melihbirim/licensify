package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	tests := []struct {
		name       string
		licenseKey string
		hardwareID string
		salt       string
		wantLen    int
	}{
		{
			name:       "Valid key derivation",
			licenseKey: "LIC-202601-ABC123-XYZ789",
			hardwareID: "hw-12345678",
			salt:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantLen:    32, // AES-256 key size
		},
		{
			name:       "Different hardware ID produces different key",
			licenseKey: "LIC-202601-ABC123-XYZ789",
			hardwareID: "hw-87654321",
			salt:       "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantLen:    32,
		},
		{
			name:       "Legacy salt (non-hex)",
			licenseKey: "LIC-202601-ABC123-XYZ789",
			hardwareID: "hw-12345678",
			salt:       "legacy-salt-string",
			wantLen:    32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := DeriveKey(tt.licenseKey, tt.hardwareID, tt.salt)
			if len(key) != tt.wantLen {
				t.Errorf("DeriveKey() key length = %v, want %v", len(key), tt.wantLen)
			}
		})
	}

	// Test that same inputs produce same key (deterministic)
	key1 := DeriveKey("test-license", "test-hw", "abcdef123456")
	key2 := DeriveKey("test-license", "test-hw", "abcdef123456")
	if !bytes.Equal(key1, key2) {
		t.Error("DeriveKey() should be deterministic")
	}

	// Test that different inputs produce different keys
	key3 := DeriveKey("test-license", "different-hw", "abcdef123456")
	if bytes.Equal(key1, key3) {
		t.Error("DeriveKey() should produce different keys for different inputs")
	}
}

func TestGenerateSalt(t *testing.T) {
	// Generate multiple salts
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}

	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}

	// Check length (should be hex-encoded 32 bytes = 64 chars)
	if len(salt1) != 64 {
		t.Errorf("GenerateSalt() length = %v, want 64", len(salt1))
	}

	// Check that salts are different (random)
	if salt1 == salt2 {
		t.Error("GenerateSalt() should generate unique salts")
	}

	// Check that it's valid hex
	_, err = hex.DecodeString(salt1)
	if err != nil {
		t.Errorf("GenerateSalt() produced invalid hex: %v", err)
	}
}

func TestEncryptDecryptAESGCM(t *testing.T) {
	// Generate a test key
	key := DeriveKey("test-license", "test-hw", "test-salt")

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "Simple text",
			plaintext: "Hello, World!",
		},
		{
			name:      "JSON data",
			plaintext: `{"api_key":"sk-test123","tier":"pro"}`,
		},
		{
			name:      "Empty string",
			plaintext: "",
		},
		{
			name:      "Unicode characters",
			plaintext: "Hello 世界 🚀",
		},
		{
			name:      "Large text",
			plaintext: string(make([]byte, 1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, iv, err := EncryptAESGCM([]byte(tt.plaintext), key)
			if err != nil {
				t.Fatalf("EncryptAESGCM() error = %v", err)
			}

			if ciphertext == "" {
				t.Error("EncryptAESGCM() returned empty ciphertext")
			}
			if iv == "" {
				t.Error("EncryptAESGCM() returned empty IV")
			}

			// Decrypt
			decrypted, err := DecryptAESGCM(ciphertext, iv, key)
			if err != nil {
				t.Fatalf("DecryptAESGCM() error = %v", err)
			}

			if string(decrypted) != tt.plaintext {
				t.Errorf("DecryptAESGCM() = %v, want %v", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestEncryptAESGCMProducesUniqueOutputs(t *testing.T) {
	key := DeriveKey("test-license", "test-hw", "test-salt")
	plaintext := []byte("same plaintext")

	// Encrypt same plaintext twice
	ciphertext1, iv1, err := EncryptAESGCM(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptAESGCM() error = %v", err)
	}

	ciphertext2, iv2, err := EncryptAESGCM(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptAESGCM() error = %v", err)
	}

	// IVs should be different (random nonce)
	if iv1 == iv2 {
		t.Error("EncryptAESGCM() should produce unique IVs")
	}

	// Ciphertexts should be different due to different IVs
	if ciphertext1 == ciphertext2 {
		t.Error("EncryptAESGCM() should produce unique ciphertexts")
	}
}

func TestDecryptAESGCMWithWrongKey(t *testing.T) {
	key1 := DeriveKey("license1", "hw1", "salt1")
	key2 := DeriveKey("license2", "hw2", "salt2")

	plaintext := []byte("secret message")

	// Encrypt with key1
	ciphertext, iv, err := EncryptAESGCM(plaintext, key1)
	if err != nil {
		t.Fatalf("EncryptAESGCM() error = %v", err)
	}

	// Try to decrypt with key2 (wrong key)
	_, err = DecryptAESGCM(ciphertext, iv, key2)
	if err == nil {
		t.Error("DecryptAESGCM() should fail with wrong key")
	}
}

func TestDecryptAESGCMWithInvalidData(t *testing.T) {
	key := DeriveKey("test-license", "test-hw", "test-salt")

	tests := []struct {
		name       string
		ciphertext string
		iv         string
		wantErr    bool
	}{
		{
			name:       "Invalid base64 ciphertext",
			ciphertext: "not-valid-base64!!!",
			iv:         "dGVzdA==",
			wantErr:    true,
		},
		{
			name:       "Invalid base64 IV",
			ciphertext: "dGVzdA==",
			iv:         "not-valid-base64!!!",
			wantErr:    true,
		},
		{
			name:       "Empty ciphertext",
			ciphertext: "",
			iv:         "dGVzdA==",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptAESGCM(tt.ciphertext, tt.iv, key)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecryptAESGCM() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeHMAC(t *testing.T) {
	key := []byte("secret-key")
	message := []byte("important message")

	signature := ComputeHMAC(key, message)

	// Check it's valid hex
	if len(signature) != 64 { // SHA256 produces 32 bytes = 64 hex chars
		t.Errorf("ComputeHMAC() length = %v, want 64", len(signature))
	}

	_, err := hex.DecodeString(signature)
	if err != nil {
		t.Errorf("ComputeHMAC() produced invalid hex: %v", err)
	}

	// Same inputs should produce same signature (deterministic)
	signature2 := ComputeHMAC(key, message)
	if signature != signature2 {
		t.Error("ComputeHMAC() should be deterministic")
	}

	// Different key should produce different signature
	differentKey := []byte("different-key")
	signature3 := ComputeHMAC(differentKey, message)
	if signature == signature3 {
		t.Error("ComputeHMAC() should produce different signatures for different keys")
	}

	// Different message should produce different signature
	differentMessage := []byte("different message")
	signature4 := ComputeHMAC(key, differentMessage)
	if signature == signature4 {
		t.Error("ComputeHMAC() should produce different signatures for different messages")
	}
}

func TestValidateHMAC(t *testing.T) {
	key := []byte("secret-key")
	message := []byte("important message")

	// Compute valid signature
	validSignature := ComputeHMAC(key, message)

	// Test valid signature
	if !ValidateHMAC(key, message, validSignature) {
		t.Error("ValidateHMAC() should return true for valid signature")
	}

	// Test invalid signature
	invalidSignature := "0000000000000000000000000000000000000000000000000000000000000000"
	if ValidateHMAC(key, message, invalidSignature) {
		t.Error("ValidateHMAC() should return false for invalid signature")
	}

	// Test with modified message
	modifiedMessage := []byte("modified message")
	if ValidateHMAC(key, modifiedMessage, validSignature) {
		t.Error("ValidateHMAC() should return false for modified message")
	}

	// Test with wrong key
	wrongKey := []byte("wrong-key")
	if ValidateHMAC(wrongKey, message, validSignature) {
		t.Error("ValidateHMAC() should return false for wrong key")
	}
}

func TestRedactPII(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Normal license key",
			input: "LIC-202601-ABC123-XYZ789",
			want:  "LIC-...Z789",
		},
		{
			name:  "Short string",
			input: "SHORT",
			want:  "***",
		},
		{
			name:  "Very short",
			input: "AB",
			want:  "***",
		},
		{
			name:  "Exactly 8 chars",
			input: "12345678",
			want:  "***",
		},
		{
			name:  "9 chars",
			input: "123456789",
			want:  "1234...6789",
		},
		{
			name:  "API key",
			input: "sk-proj-1234567890abcdefghijklmnopqrstuvwxyz",
			want:  "sk-p...wxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactPII(tt.input)
			if got != tt.want {
				t.Errorf("RedactPII() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedactEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{
			name:  "Normal email",
			email: "user@example.com",
			want:  "us***@example.com",
		},
		{
			name:  "Short username",
			email: "ab@example.com",
			want:  "***@example.com",
		},
		{
			name:  "Single char username",
			email: "a@example.com",
			want:  "***@example.com",
		},
		{
			name:  "Long username",
			email: "verylongusername@example.com",
			want:  "ve***@example.com",
		},
		{
			name:  "Invalid email (no @)",
			email: "notanemail",
			want:  "***@***",
		},
		{
			name:  "Invalid email (multiple @)",
			email: "user@@example.com",
			want:  "***@***",
		},
		{
			name:  "Empty string",
			email: "",
			want:  "***@***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactEmail(tt.email)
			if got != tt.want {
				t.Errorf("RedactEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkDeriveKey(b *testing.B) {
	licenseKey := "LIC-202601-ABC123-XYZ789"
	hardwareID := "hw-12345678"
	salt := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DeriveKey(licenseKey, hardwareID, salt)
	}
}

func BenchmarkEncryptAESGCM(b *testing.B) {
	key := DeriveKey("test-license", "test-hw", "test-salt")
	plaintext := []byte("test message for encryption")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncryptAESGCM(plaintext, key)
	}
}

func BenchmarkComputeHMAC(b *testing.B) {
	key := []byte("secret-key")
	message := []byte("message to sign")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeHMAC(key, message)
	}
}
