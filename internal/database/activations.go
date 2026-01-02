package database

import "fmt"

// GetActivationCount returns the number of activations for a license
func (db *DB) GetActivationCount(licenseID string) (int, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM activations WHERE license_id = %s", db.Placeholder(1))
	err := db.QueryRow(query, licenseID).Scan(&count)
	return count, err
}

// IsHardwareActivated checks if a hardware ID is activated for a license
func (db *DB) IsHardwareActivated(licenseID, hardwareID string) (bool, error) {
	var count int
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM activations 
		WHERE license_id = %s AND hardware_id = %s
	`, db.Placeholder(1), db.Placeholder(2))

	err := db.QueryRow(query, licenseID, hardwareID).Scan(&count)
	return count > 0, err
}

// RecordActivation records a new hardware activation
func (db *DB) RecordActivation(licenseID, hardwareID string) error {
	query := fmt.Sprintf(`
		INSERT INTO activations (license_id, hardware_id) 
		VALUES (%s, %s)
	`, db.Placeholder(1), db.Placeholder(2))

	_, err := db.Exec(query, licenseID, hardwareID)
	return err
}

// IsFreeHardwareAlreadyActive checks if hardware has another active free license
func (db *DB) IsFreeHardwareAlreadyActive(hardwareID, requestedLicenseID string) (bool, error) {
	var count int
	// Use boolean true for PostgreSQL compatibility, works with SQLite too
	query := fmt.Sprintf(`
		SELECT COUNT(DISTINCT a.license_id) 
		FROM activations a
		JOIN licenses l ON a.license_id = l.license_id
		WHERE a.hardware_id = %s 
		  AND l.tier = 'free' 
		  AND l.active = true 
		  AND l.expires_at > CURRENT_TIMESTAMP
		  AND a.license_id != %s
	`, db.Placeholder(1), db.Placeholder(2))

	err := db.QueryRow(query, hardwareID, requestedLicenseID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RecordCheckIn records or updates last check-in time for a license
func (db *DB) RecordCheckIn(licenseID string) error {
	query := fmt.Sprintf(`
		INSERT INTO check_ins (license_id, last_check_in) 
		VALUES (%s, CURRENT_TIMESTAMP)
		ON CONFLICT(license_id) DO UPDATE SET 
		last_check_in = CURRENT_TIMESTAMP
	`, db.Placeholder(1))

	_, err := db.Exec(query, licenseID)
	return err
}
