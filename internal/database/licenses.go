package database

import (
	"database/sql"
	"fmt"
	"time"
)

// License represents a license record
type License struct {
	LicenseID      string
	CustomerName   string
	CustomerEmail  string
	Tier           string
	ExpiresAt      string
	DailyLimit     int
	MonthlyLimit   int
	MaxActivations int
	Active         bool
	EncryptionSalt string
}

// CreateLicense creates a new license
func (db *DB) CreateLicense(license License) error {
	query := fmt.Sprintf(`
		INSERT INTO licenses (
			license_id, customer_name, customer_email, tier, 
			expires_at, daily_limit, monthly_limit, max_activations, active, encryption_salt
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
	`, db.Placeholder(1), db.Placeholder(2), db.Placeholder(3), db.Placeholder(4),
		db.Placeholder(5), db.Placeholder(6), db.Placeholder(7), db.Placeholder(8),
		db.Placeholder(9), db.Placeholder(10))

	_, err := db.Exec(query,
		license.LicenseID,
		license.CustomerName,
		license.CustomerEmail,
		license.Tier,
		license.ExpiresAt,
		license.DailyLimit,
		license.MonthlyLimit,
		license.MaxActivations,
		license.Active,
		license.EncryptionSalt,
	)

	return err
}

// GetLicense retrieves a license by ID
func (db *DB) GetLicense(licenseID string) (*License, error) {
	var license License
	var encryptionSalt sql.NullString

	query := fmt.Sprintf(`
		SELECT customer_name, customer_email, tier, expires_at, 
		       daily_limit, monthly_limit, max_activations, active, encryption_salt
		FROM licenses WHERE license_id = %s
	`, db.Placeholder(1))

	err := db.QueryRow(query, licenseID).Scan(
		&license.CustomerName,
		&license.CustomerEmail,
		&license.Tier,
		&license.ExpiresAt,
		&license.DailyLimit,
		&license.MonthlyLimit,
		&license.MaxActivations,
		&license.Active,
		&encryptionSalt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("license not found")
	}
	if err != nil {
		return nil, err
	}

	license.LicenseID = licenseID
	if encryptionSalt.Valid {
		license.EncryptionSalt = encryptionSalt.String
	}

	return &license, nil
}

// GetLicenseByEmail retrieves a license by customer email
func (db *DB) GetLicenseByEmail(email string) (string, error) {
	var licenseID string
	query := fmt.Sprintf(`
		SELECT license_id FROM licenses WHERE customer_email = %s
	`, db.Placeholder(1))

	err := db.QueryRow(query, email).Scan(&licenseID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return licenseID, err
}

// UpdateLicenseSalt updates the encryption salt for a license
func (db *DB) UpdateLicenseSalt(licenseID, salt string) error {
	query := fmt.Sprintf("UPDATE licenses SET encryption_salt = %s WHERE license_id = %s",
		db.Placeholder(1), db.Placeholder(2))
	_, err := db.Exec(query, salt, licenseID)
	return err
}

// GetActiveLicense retrieves an active license with all fields
func (db *DB) GetActiveLicense(licenseID string) (license License, expiresAtUnix int64, err error) {
	if db.isPostgres {
		// PostgreSQL: use EXTRACT(EPOCH FROM expires_at)
		query := fmt.Sprintf(`
			SELECT license_id, tier, daily_limit, monthly_limit, EXTRACT(EPOCH FROM expires_at)::bigint
			FROM licenses 
			WHERE license_id = %s AND active = true
		`, db.Placeholder(1))

		err = db.QueryRow(query, licenseID).Scan(
			&license.LicenseID,
			&license.Tier,
			&license.DailyLimit,
			&license.MonthlyLimit,
			&expiresAtUnix,
		)

		if err == nil {
			license.ExpiresAt = time.Unix(expiresAtUnix, 0).Format(time.RFC3339)
		}
	} else {
		// SQLite: expires_at is stored as TEXT in RFC3339 format
		query := fmt.Sprintf(`
			SELECT license_id, tier, daily_limit, monthly_limit, expires_at
			FROM licenses 
			WHERE license_id = %s AND active = true
		`, db.Placeholder(1))

		err = db.QueryRow(query, licenseID).Scan(
			&license.LicenseID,
			&license.Tier,
			&license.DailyLimit,
			&license.MonthlyLimit,
			&license.ExpiresAt,
		)

		if err == nil {
			// Parse to get Unix timestamp
			t, parseErr := time.Parse(time.RFC3339, license.ExpiresAt)
			if parseErr == nil {
				expiresAtUnix = t.Unix()
			}
		}
	}

	return license, expiresAtUnix, err
}
