package license

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"
)

type License struct {
	LicenseID      string
	CustomerName   string
	CustomerEmail  string
	Tier           string
	ExpiresAt      time.Time
	EncryptionSalt string
	Active         bool
	Limits         Limits
}

type Limits struct {
	DailyLimit     int
	MonthlyLimit   int
	MaxActivations int
}

type Manager struct {
	db             *sql.DB
	isPostgresDB   bool
	sqlPlaceholder func(int) string
}

func NewManager(db *sql.DB, isPostgresDB bool, sqlPlaceholder func(int) string) *Manager {
	return &Manager{
		db:             db,
		isPostgresDB:   isPostgresDB,
		sqlPlaceholder: sqlPlaceholder,
	}
}

func (m *Manager) Get(licenseID string) (*License, error) {
	var license License
	license.LicenseID = licenseID

	var encryptionSalt sql.NullString

	err := m.db.QueryRow(fmt.Sprintf(`
SELECT customer_name, customer_email, tier, expires_at, 
       daily_limit, monthly_limit, max_activations, active, encryption_salt
FROM licenses WHERE license_id = %s
`, m.sqlPlaceholder(1)), licenseID).Scan(
		&license.CustomerName,
		&license.CustomerEmail,
		&license.Tier,
		&license.ExpiresAt,
		&license.Limits.DailyLimit,
		&license.Limits.MonthlyLimit,
		&license.Limits.MaxActivations,
		&license.Active,
		&encryptionSalt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("license not found")
	}
	if err != nil {
		return nil, err
	}

	if !encryptionSalt.Valid || encryptionSalt.String == "" {
		salt, err := GenerateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		_, err = m.db.Exec(fmt.Sprintf("UPDATE licenses SET encryption_salt = %s WHERE license_id = %s",
			m.sqlPlaceholder(1), m.sqlPlaceholder(2)), salt, licenseID)
		if err != nil {
			log.Printf("Warning: Failed to store salt for license %s: %v", RedactPII(licenseID), err)
		}
		license.EncryptionSalt = salt
	} else {
		license.EncryptionSalt = encryptionSalt.String
	}

	return &license, nil
}

func (m *Manager) Validate(licenseID string) error {
	license, err := m.Get(licenseID)
	if err != nil {
		return fmt.Errorf("invalid license: %w", err)
	}

	if !license.Active {
		return fmt.Errorf("license has been deactivated")
	}

	if time.Now().After(license.ExpiresAt) {
		return fmt.Errorf("license has expired")
	}

	return nil
}

func (m *Manager) GetActivationCount(licenseID string) (int, error) {
	var count int
	err := m.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM activations WHERE license_id = %s", m.sqlPlaceholder(1)), licenseID).Scan(&count)
	return count, err
}

func (m *Manager) IsHardwareActivated(licenseID, hardwareID string) (bool, error) {
	var count int
	err := m.db.QueryRow(fmt.Sprintf(`
SELECT COUNT(*) FROM activations 
WHERE license_id = %s AND hardware_id = %s
`, m.sqlPlaceholder(1), m.sqlPlaceholder(2)), licenseID, hardwareID).Scan(&count)
	return count > 0, err
}

func (m *Manager) RecordActivation(licenseID, hardwareID string) error {
	_, err := m.db.Exec(fmt.Sprintf(`
INSERT INTO activations (license_id, hardware_id) 
VALUES (%s, %s)
`, m.sqlPlaceholder(1), m.sqlPlaceholder(2)), licenseID, hardwareID)
	return err
}

func (m *Manager) IsFreeHardwareAlreadyActive(hardwareID, requestedLicenseID string) bool {
	var count int
	err := m.db.QueryRow(fmt.Sprintf(`
SELECT COUNT(DISTINCT a.license_id) 
FROM activations a
JOIN licenses l ON a.license_id = l.license_id
WHERE a.hardware_id = %s 
  AND l.tier = 'free' 
  AND l.active = true 
  AND l.expires_at > CURRENT_TIMESTAMP
  AND a.license_id != %s
`, m.sqlPlaceholder(1), m.sqlPlaceholder(2)), hardwareID, requestedLicenseID).Scan(&count)

	if err != nil {
		log.Printf("Error checking free hardware: %v", err)
		return false
	}

	return count > 0
}

func (m *Manager) RecordCheckIn(licenseID string) error {
	_, err := m.db.Exec(fmt.Sprintf(`
INSERT INTO check_ins (license_id, last_check_in) 
VALUES (%s, CURRENT_TIMESTAMP)
ON CONFLICT(license_id) DO UPDATE SET 
last_check_in = CURRENT_TIMESTAMP
`, m.sqlPlaceholder(1)), licenseID)
	return err
}

func (m *Manager) GetUsage(licenseID, date string) (dailyUsage, monthlyUsage int, err error) {
	err = m.db.QueryRow(fmt.Sprintf(`
SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
WHERE license_id = %s AND date = %s
`, m.sqlPlaceholder(1), m.sqlPlaceholder(2)), licenseID, date).Scan(&dailyUsage)

	if err != nil {
		return 0, 0, err
	}

	yearMonth := date[:7]
	err = m.db.QueryRow(fmt.Sprintf(`
SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
WHERE license_id = %s AND date LIKE %s
`, m.sqlPlaceholder(1), m.sqlPlaceholder(2)), licenseID, yearMonth+"%").Scan(&monthlyUsage)

	return dailyUsage, monthlyUsage, err
}

func (m *Manager) CanActivate(licenseID, hardwareID string) error {
	license, err := m.Get(licenseID)
	if err != nil {
		return fmt.Errorf("invalid license: %w", err)
	}

	if !license.Active {
		return fmt.Errorf("license has been deactivated")
	}

	if time.Now().After(license.ExpiresAt) {
		return fmt.Errorf("license has expired")
	}

	count, err := m.GetActivationCount(licenseID)
	if err != nil {
		return fmt.Errorf("failed to check activations: %w", err)
	}

	alreadyActivated, err := m.IsHardwareActivated(licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to check hardware: %w", err)
	}

	if !alreadyActivated && count >= license.Limits.MaxActivations {
		return fmt.Errorf("maximum activations (%d) reached", license.Limits.MaxActivations)
	}

	if license.Tier == "free" && m.IsFreeHardwareAlreadyActive(hardwareID, licenseID) {
		return fmt.Errorf("this device already has an active FREE license. Each device is limited to one free license")
	}

	return nil
}

func GenerateKey() string {
	timestamp := time.Now().Format("200601")
	part1 := randomString(6)
	part2 := randomString(6)
	return fmt.Sprintf("LIC-%s-%s-%s", timestamp, part1, part2)
}

func randomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result)
}

func GenerateSalt() (string, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", salt), nil
}

func RedactPII(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func ValidateHardwareID(hardwareID string) error {
	hardwareID = strings.TrimSpace(hardwareID)
	if len(hardwareID) < 8 {
		return fmt.Errorf("hardware_id must be at least 8 characters")
	}
	return nil
}

func ValidateLicenseKey(licenseKey string) error {
	licenseKey = strings.TrimSpace(licenseKey)
	if licenseKey == "" {
		return fmt.Errorf("license key is required")
	}
	return nil
}
