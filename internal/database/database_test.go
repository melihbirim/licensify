package database

import (
	"os"
	"testing"
	"time"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	// Create temp directory for test schema files
	tmpDir := t.TempDir()
	schemaPath := tmpDir + "/init.sql"

	// Write minimal schema for tests
	schema := `
CREATE TABLE IF NOT EXISTS licenses (
    license_id TEXT PRIMARY KEY,
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    tier TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    daily_limit INTEGER NOT NULL,
    monthly_limit INTEGER NOT NULL,
    max_activations INTEGER NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    encryption_salt TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS activations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    license_id TEXT NOT NULL,
    hardware_id TEXT NOT NULL,
    activated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (license_id) REFERENCES licenses(license_id),
    UNIQUE(license_id, hardware_id)
);

CREATE TABLE IF NOT EXISTS daily_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    license_id TEXT NOT NULL,
    date TEXT NOT NULL,
    scans INTEGER NOT NULL DEFAULT 0,
    hardware_id TEXT NOT NULL,
    FOREIGN KEY (license_id) REFERENCES licenses(license_id),
    UNIQUE(license_id, date)
);

CREATE TABLE IF NOT EXISTS check_ins (
    license_id TEXT PRIMARY KEY,
    last_check_in TIMESTAMP NOT NULL,
    FOREIGN KEY (license_id) REFERENCES licenses(license_id)
);

CREATE TABLE IF NOT EXISTS verification_codes (
    email TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS proxy_keys (
    proxy_key TEXT PRIMARY KEY,
    license_id TEXT NOT NULL,
    hardware_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (license_id) REFERENCES licenses(license_id)
);
`

	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatalf("Failed to write test schema: %v", err)
	}

	// Create in-memory database
	cfg := Config{
		DBPath: ":memory:",
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Manually initialize schema since we're using a temp path
	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	return db
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "SQLite in-memory",
			cfg: Config{
				DBPath: ":memory:",
			},
			wantErr: false,
		},
		{
			name: "SQLite with path",
			cfg: Config{
				DBPath: t.TempDir() + "/test.db",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && db == nil {
				t.Error("New() returned nil database")
			}
			if db != nil {
				defer db.Close()

				// Test Placeholder method
				if db.IsPostgres() {
					if got := db.Placeholder(1); got != "$1" {
						t.Errorf("Placeholder(1) = %v, want $1", got)
					}
				} else {
					if got := db.Placeholder(1); got != "?" {
						t.Errorf("Placeholder(1) = %v, want ?", got)
					}
				}
			}
		})
	}
}

func TestCreateAndGetLicense(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	license := License{
		LicenseID:      "test-license-123",
		CustomerName:   "Test User",
		CustomerEmail:  "test@example.com",
		Tier:           "free",
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     10,
		MonthlyLimit:   100,
		MaxActivations: 3,
		Active:         true,
		EncryptionSalt: "test-salt-abc",
	}

	// Test CreateLicense
	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Test GetLicense
	got, err := db.GetLicense(license.LicenseID)
	if err != nil {
		t.Fatalf("GetLicense() error = %v", err)
	}

	if got.LicenseID != license.LicenseID {
		t.Errorf("GetLicense() LicenseID = %v, want %v", got.LicenseID, license.LicenseID)
	}
	if got.CustomerEmail != license.CustomerEmail {
		t.Errorf("GetLicense() CustomerEmail = %v, want %v", got.CustomerEmail, license.CustomerEmail)
	}
	if got.Tier != license.Tier {
		t.Errorf("GetLicense() Tier = %v, want %v", got.Tier, license.Tier)
	}
}

func TestGetLicenseNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.GetLicense("nonexistent")
	if err == nil {
		t.Error("GetLicense() expected error for nonexistent license")
	}
}

func TestGetLicenseByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	license := License{
		LicenseID:      "test-license-456",
		CustomerName:   "Test User 2",
		CustomerEmail:  "test2@example.com",
		Tier:           "pro",
		ExpiresAt:      time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     100,
		MonthlyLimit:   1000,
		MaxActivations: 5,
		Active:         true,
		EncryptionSalt: "test-salt-def",
	}

	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Test GetLicenseByEmail
	licenseID, err := db.GetLicenseByEmail(license.CustomerEmail)
	if err != nil {
		t.Fatalf("GetLicenseByEmail() error = %v", err)
	}

	if licenseID != license.LicenseID {
		t.Errorf("GetLicenseByEmail() = %v, want %v", licenseID, license.LicenseID)
	}
}

func TestUpdateLicenseSalt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	license := License{
		LicenseID:      "test-license-789",
		CustomerName:   "Test User 3",
		CustomerEmail:  "test3@example.com",
		Tier:           "enterprise",
		ExpiresAt:      time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     1000,
		MonthlyLimit:   10000,
		MaxActivations: 10,
		Active:         true,
		EncryptionSalt: "old-salt",
	}

	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Update salt
	newSalt := "new-salt-xyz"
	err = db.UpdateLicenseSalt(license.LicenseID, newSalt)
	if err != nil {
		t.Fatalf("UpdateLicenseSalt() error = %v", err)
	}

	// Verify update
	got, err := db.GetLicense(license.LicenseID)
	if err != nil {
		t.Fatalf("GetLicense() error = %v", err)
	}

	if got.EncryptionSalt != newSalt {
		t.Errorf("UpdateLicenseSalt() salt = %v, want %v", got.EncryptionSalt, newSalt)
	}
}

func TestActivations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	licenseID := "test-license-act"
	hardwareID1 := "hw-001"
	hardwareID2 := "hw-002"

	// Create test license
	license := License{
		LicenseID:      licenseID,
		CustomerName:   "Test User",
		CustomerEmail:  "test@example.com",
		Tier:           "free",
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     10,
		MonthlyLimit:   100,
		MaxActivations: 3,
		Active:         true,
		EncryptionSalt: "salt",
	}
	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Test GetActivationCount - should be 0
	count, err := db.GetActivationCount(licenseID)
	if err != nil {
		t.Fatalf("GetActivationCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("GetActivationCount() = %v, want 0", count)
	}

	// Test IsHardwareActivated - should be false
	activated, err := db.IsHardwareActivated(licenseID, hardwareID1)
	if err != nil {
		t.Fatalf("IsHardwareActivated() error = %v", err)
	}
	if activated {
		t.Error("IsHardwareActivated() = true, want false")
	}

	// Test RecordActivation
	err = db.RecordActivation(licenseID, hardwareID1)
	if err != nil {
		t.Fatalf("RecordActivation() error = %v", err)
	}

	// Test GetActivationCount - should be 1
	count, err = db.GetActivationCount(licenseID)
	if err != nil {
		t.Fatalf("GetActivationCount() error = %v", err)
	}
	if count != 1 {
		t.Errorf("GetActivationCount() = %v, want 1", count)
	}

	// Test IsHardwareActivated - should be true
	activated, err = db.IsHardwareActivated(licenseID, hardwareID1)
	if err != nil {
		t.Fatalf("IsHardwareActivated() error = %v", err)
	}
	if !activated {
		t.Error("IsHardwareActivated() = false, want true")
	}

	// Add another activation
	err = db.RecordActivation(licenseID, hardwareID2)
	if err != nil {
		t.Fatalf("RecordActivation() error = %v", err)
	}

	// Test GetActivationCount - should be 2
	count, err = db.GetActivationCount(licenseID)
	if err != nil {
		t.Fatalf("GetActivationCount() error = %v", err)
	}
	if count != 2 {
		t.Errorf("GetActivationCount() = %v, want 2", count)
	}
}

func TestRecordCheckIn(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	licenseID := "test-license-checkin"

	// Create test license
	license := License{
		LicenseID:      licenseID,
		CustomerName:   "Test User",
		CustomerEmail:  "test@example.com",
		Tier:           "free",
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     10,
		MonthlyLimit:   100,
		MaxActivations: 3,
		Active:         true,
		EncryptionSalt: "salt",
	}
	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Test RecordCheckIn
	err = db.RecordCheckIn(licenseID)
	if err != nil {
		t.Fatalf("RecordCheckIn() error = %v", err)
	}

	// Verify check-in was recorded
	var lastCheckIn string
	query := "SELECT last_check_in FROM check_ins WHERE license_id = ?"
	err = db.QueryRow(query, licenseID).Scan(&lastCheckIn)
	if err != nil {
		t.Fatalf("Failed to query check_ins: %v", err)
	}
	if lastCheckIn == "" {
		t.Error("RecordCheckIn() did not record check-in time")
	}

	// Test updating check-in (should not error)
	time.Sleep(10 * time.Millisecond)
	err = db.RecordCheckIn(licenseID)
	if err != nil {
		t.Fatalf("RecordCheckIn() update error = %v", err)
	}
}

func TestUsage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	licenseID := "test-license-usage"
	hardwareID := "hw-usage-001"
	date := "2026-01-02"

	// Create test license
	license := License{
		LicenseID:      licenseID,
		CustomerName:   "Test User",
		CustomerEmail:  "test@example.com",
		Tier:           "free",
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     10,
		MonthlyLimit:   100,
		MaxActivations: 3,
		Active:         true,
		EncryptionSalt: "salt",
	}
	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Test RecordUsage
	err = db.RecordUsage(licenseID, date, 5, hardwareID)
	if err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}

	// Test GetDailyUsage
	dailyUsage, err := db.GetDailyUsage(licenseID, date)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}
	if dailyUsage != 5 {
		t.Errorf("GetDailyUsage() = %v, want 5", dailyUsage)
	}

	// Test GetUsage
	daily, monthly := db.GetUsage(licenseID, date)
	if daily != 5 {
		t.Errorf("GetUsage() daily = %v, want 5", daily)
	}
	if monthly != 5 {
		t.Errorf("GetUsage() monthly = %v, want 5", monthly)
	}

	// Record more usage (should increment)
	err = db.RecordUsage(licenseID, date, 3, hardwareID)
	if err != nil {
		t.Fatalf("RecordUsage() increment error = %v", err)
	}

	// Verify total
	dailyUsage, err = db.GetDailyUsage(licenseID, date)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}
	if dailyUsage != 8 {
		t.Errorf("GetDailyUsage() after increment = %v, want 8", dailyUsage)
	}
}

func TestVerificationCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	email := "verify@example.com"
	code := "123456"
	expiresAt := time.Now().Add(15 * time.Minute)

	// Test StoreVerificationCode
	err := db.StoreVerificationCode(email, code, expiresAt)
	if err != nil {
		t.Fatalf("StoreVerificationCode() error = %v", err)
	}

	// Test GetVerificationCode
	vc, err := db.GetVerificationCode(email)
	if err != nil {
		t.Fatalf("GetVerificationCode() error = %v", err)
	}

	if vc.Code != code {
		t.Errorf("GetVerificationCode() code = %v, want %v", vc.Code, code)
	}
	if vc.Email != email {
		t.Errorf("GetVerificationCode() email = %v, want %v", vc.Email, email)
	}

	// Test IsVerificationCodeValid
	valid, err := db.IsVerificationCodeValid(email, code)
	if err != nil {
		t.Fatalf("IsVerificationCodeValid() error = %v", err)
	}
	if !valid {
		t.Error("IsVerificationCodeValid() = false, want true")
	}

	// Test with wrong code
	valid, err = db.IsVerificationCodeValid(email, "wrong")
	if err == nil {
		t.Error("IsVerificationCodeValid() expected error for wrong code")
	}
	if valid {
		t.Error("IsVerificationCodeValid() = true, want false for wrong code")
	}

	// Test DeleteVerificationCode
	err = db.DeleteVerificationCode(email)
	if err != nil {
		t.Fatalf("DeleteVerificationCode() error = %v", err)
	}

	// Verify deleted
	_, err = db.GetVerificationCode(email)
	if err == nil {
		t.Error("GetVerificationCode() expected error after delete")
	}
}

func TestProxyKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	proxyKey := "px_test123"
	licenseID := "test-license-proxy"
	hardwareID := "hw-proxy-001"

	// Create test license
	license := License{
		LicenseID:      licenseID,
		CustomerName:   "Test User",
		CustomerEmail:  "test@example.com",
		Tier:           "free",
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		DailyLimit:     10,
		MonthlyLimit:   100,
		MaxActivations: 3,
		Active:         true,
		EncryptionSalt: "salt",
	}
	err := db.CreateLicense(license)
	if err != nil {
		t.Fatalf("CreateLicense() error = %v", err)
	}

	// Test StoreProxyKey
	err = db.StoreProxyKey(proxyKey, licenseID, hardwareID)
	if err != nil {
		t.Fatalf("StoreProxyKey() error = %v", err)
	}

	// Test ValidateProxyKey
	gotLicense, gotHardware, err := db.ValidateProxyKey(proxyKey)
	if err != nil {
		t.Fatalf("ValidateProxyKey() error = %v", err)
	}

	if gotLicense != licenseID {
		t.Errorf("ValidateProxyKey() license = %v, want %v", gotLicense, licenseID)
	}
	if gotHardware != hardwareID {
		t.Errorf("ValidateProxyKey() hardware = %v, want %v", gotHardware, hardwareID)
	}

	// Test updating proxy key (should replace old one)
	newProxyKey := "px_test456"
	err = db.StoreProxyKey(newProxyKey, licenseID, hardwareID)
	if err != nil {
		t.Fatalf("StoreProxyKey() update error = %v", err)
	}

	// Old key should not work
	_, _, err = db.ValidateProxyKey(proxyKey)
	if err == nil {
		t.Error("ValidateProxyKey() expected error for replaced key")
	}

	// New key should work
	gotLicense, gotHardware, err = db.ValidateProxyKey(newProxyKey)
	if err != nil {
		t.Fatalf("ValidateProxyKey() new key error = %v", err)
	}
	if gotLicense != licenseID {
		t.Errorf("ValidateProxyKey() new key license = %v, want %v", gotLicense, licenseID)
	}
}
