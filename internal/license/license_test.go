package license

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	schema := `
	CREATE TABLE licenses (
		license_id TEXT PRIMARY KEY,
		customer_name TEXT NOT NULL,
		customer_email TEXT NOT NULL,
		tier TEXT NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		daily_limit INTEGER NOT NULL,
		monthly_limit INTEGER NOT NULL,
		max_activations INTEGER NOT NULL,
		active BOOLEAN DEFAULT true,
		encryption_salt TEXT
	);

	CREATE TABLE activations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		license_id TEXT NOT NULL,
		hardware_id TEXT NOT NULL,
		activated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (license_id) REFERENCES licenses(license_id)
	);

	CREATE TABLE check_ins (
		license_id TEXT PRIMARY KEY,
		last_check_in TIMESTAMP NOT NULL,
		FOREIGN KEY (license_id) REFERENCES licenses(license_id)
	);

	CREATE TABLE daily_usage (
		license_id TEXT NOT NULL,
		date TEXT NOT NULL,
		scans INTEGER NOT NULL DEFAULT 0,
		hardware_id TEXT,
		PRIMARY KEY (license_id, date),
		FOREIGN KEY (license_id) REFERENCES licenses(license_id)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func sqlitePlaceholder(n int) string {
	return "?"
}

func insertTestLicense(t *testing.T, db *sql.DB, licenseID, tier string, expiresAt time.Time, active bool, salt string) {
	_, err := db.Exec(`
		INSERT INTO licenses (license_id, customer_name, customer_email, tier, expires_at, 
		                      daily_limit, monthly_limit, max_activations, active, encryption_salt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, licenseID, "Test User", "test@example.com", tier, expiresAt, 100, 3000, 3, active, salt)
	if err != nil {
		t.Fatalf("Failed to insert test license: %v", err)
	}
}

func TestNewManager(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)
	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}
}

func TestGetLicense(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	salt := "0123456789abcdef"
	insertTestLicense(t, db, "LIC-TEST-123", "pro", expiresAt, true, salt)

	license, err := manager.Get("LIC-TEST-123")
	if err != nil {
		t.Fatalf("Failed to get license: %v", err)
	}

	if license.LicenseID != "LIC-TEST-123" {
		t.Errorf("Expected license ID LIC-TEST-123, got %s", license.LicenseID)
	}

	if license.Tier != "pro" {
		t.Errorf("Expected tier 'pro', got %s", license.Tier)
	}
}

func TestValidateLicense(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	t.Run("Valid license", func(t *testing.T) {
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		insertTestLicense(t, db, "LIC-VALID-001", "pro", expiresAt, true, "salt")

		err := manager.Validate("LIC-VALID-001")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("Inactive license", func(t *testing.T) {
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		insertTestLicense(t, db, "LIC-INACTIVE-001", "pro", expiresAt, false, "salt")

		err := manager.Validate("LIC-INACTIVE-001")
		if err == nil || !strings.Contains(err.Error(), "deactivated") {
			t.Errorf("Expected deactivated error, got: %v", err)
		}
	})

	t.Run("Expired license", func(t *testing.T) {
		expiresAt := time.Now().Add(-24 * time.Hour)
		insertTestLicense(t, db, "LIC-EXPIRED-001", "pro", expiresAt, true, "salt")

		err := manager.Validate("LIC-EXPIRED-001")
		if err == nil || !strings.Contains(err.Error(), "expired") {
			t.Errorf("Expected expired error, got: %v", err)
		}
	})
}

func TestActivationFunctions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	insertTestLicense(t, db, "LIC-ACT-001", "pro", expiresAt, true, "salt")

	t.Run("Record and check activation", func(t *testing.T) {
		err := manager.RecordActivation("LIC-ACT-001", "hw-001")
		if err != nil {
			t.Fatalf("Failed to record activation: %v", err)
		}

		activated, err := manager.IsHardwareActivated("LIC-ACT-001", "hw-001")
		if err != nil {
			t.Fatalf("Failed to check activation: %v", err)
		}

		if !activated {
			t.Error("Expected hardware to be activated")
		}
	})

	t.Run("Get activation count", func(t *testing.T) {
		count, err := manager.GetActivationCount("LIC-ACT-001")
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}
	})
}

func TestCanActivate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	t.Run("Valid activation", func(t *testing.T) {
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		insertTestLicense(t, db, "LIC-CAN-001", "pro", expiresAt, true, "salt")

		err := manager.CanActivate("LIC-CAN-001", "hw-new")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("Max activations reached", func(t *testing.T) {
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		insertTestLicense(t, db, "LIC-CAN-002", "pro", expiresAt, true, "salt")

		db.Exec("INSERT INTO activations (license_id, hardware_id) VALUES (?, ?)", "LIC-CAN-002", "hw1")
		db.Exec("INSERT INTO activations (license_id, hardware_id) VALUES (?, ?)", "LIC-CAN-002", "hw2")
		db.Exec("INSERT INTO activations (license_id, hardware_id) VALUES (?, ?)", "LIC-CAN-002", "hw3")

		err := manager.CanActivate("LIC-CAN-002", "hw-new")
		if err == nil {
			t.Error("Expected max activations error")
		} else if !strings.Contains(err.Error(), "maximum activations") {
			t.Errorf("Expected max activations error, got: %v", err)
		}
	})
}

func TestGenerateKey(t *testing.T) {
	key1 := GenerateKey()
	key2 := GenerateKey()

	if !strings.HasPrefix(key1, "LIC-") {
		t.Errorf("Expected key to start with 'LIC-', got: %s", key1)
	}

	if key1 == key2 {
		t.Error("Expected unique keys")
	}
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	if len(salt1) != 64 {
		t.Errorf("Expected salt length 64, got %d", len(salt1))
	}

	salt2, _ := GenerateSalt()
	if salt1 == salt2 {
		t.Error("Expected unique salts")
	}
}

func TestRedactPII(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"LIC-202601-ABC123-XYZ789", "LIC-...Z789"},
		{"SHORT", "***"},
		{"12345678", "***"},
	}

	for _, tt := range tests {
		result := RedactPII(tt.input)
		if result != tt.expected {
			t.Errorf("RedactPII(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestValidateHardwareID(t *testing.T) {
	tests := []struct {
		hardwareID string
		wantError  bool
	}{
		{"12345678", false},
		{"short", true},
		{"", true},
	}

	for _, tt := range tests {
		err := ValidateHardwareID(tt.hardwareID)
		if (err != nil) != tt.wantError {
			t.Errorf("ValidateHardwareID(%s) error = %v, wantError %v", tt.hardwareID, err, tt.wantError)
		}
	}
}

func TestValidateLicenseKey(t *testing.T) {
	tests := []struct {
		licenseKey string
		wantError  bool
	}{
		{"LIC-123", false},
		{"", true},
		{"  ", true},
	}

	for _, tt := range tests {
		err := ValidateLicenseKey(tt.licenseKey)
		if (err != nil) != tt.wantError {
			t.Errorf("ValidateLicenseKey(%s) error = %v, wantError %v", tt.licenseKey, err, tt.wantError)
		}
	}
}

func TestRecordCheckIn(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	insertTestLicense(t, db, "LIC-CHECK-001", "pro", expiresAt, true, "salt")

	err := manager.RecordCheckIn("LIC-CHECK-001")
	if err != nil {
		t.Fatalf("Failed to record check-in: %v", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM check_ins WHERE license_id = ?", "LIC-CHECK-001").Scan(&count)

	if count != 1 {
		t.Errorf("Expected 1 check-in record, got %d", count)
	}
}

func TestGetUsage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	insertTestLicense(t, db, "LIC-USAGE-001", "pro", expiresAt, true, "salt")

	db.Exec("INSERT INTO daily_usage (license_id, date, scans) VALUES (?, ?, ?)", "LIC-USAGE-001", "2026-01-01", 10)
	db.Exec("INSERT INTO daily_usage (license_id, date, scans) VALUES (?, ?, ?)", "LIC-USAGE-001", "2026-01-02", 20)

	dailyUsage, monthlyUsage, err := manager.GetUsage("LIC-USAGE-001", "2026-01-02")
	if err != nil {
		t.Fatalf("Failed to get usage: %v", err)
	}

	if dailyUsage != 20 {
		t.Errorf("Expected daily usage 20, got %d", dailyUsage)
	}

	if monthlyUsage != 30 {
		t.Errorf("Expected monthly usage 30, got %d", monthlyUsage)
	}
}

func TestIsFreeHardwareAlreadyActive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := NewManager(db, false, sqlitePlaceholder)

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	insertTestLicense(t, db, "LIC-FREE-001", "free", expiresAt, true, "salt")
	insertTestLicense(t, db, "LIC-FREE-002", "free", expiresAt, true, "salt")

	db.Exec("INSERT INTO activations (license_id, hardware_id) VALUES (?, ?)", "LIC-FREE-001", "hw-shared")

	hasActive := manager.IsFreeHardwareAlreadyActive("hw-shared", "LIC-FREE-002")
	if !hasActive {
		t.Error("Expected hardware to have active free license")
	}

	hasActiveSame := manager.IsFreeHardwareAlreadyActive("hw-shared", "LIC-FREE-001")
	if hasActiveSame {
		t.Error("Same license should not count as already active")
	}
}
