package database

import (
	"database/sql"
	"fmt"
)

// StoreProxyKey stores a proxy key mapping with transaction support
func (db *DB) StoreProxyKey(proxyKey, licenseID, hardwareID string) error {
	// Use transaction to ensure atomicity
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing proxy key for this license+hardware (if any)
	deleteQuery := fmt.Sprintf(`DELETE FROM proxy_keys WHERE license_id = %s AND hardware_id = %s`,
		db.Placeholder(1), db.Placeholder(2))
	_, err = tx.Exec(deleteQuery, licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to delete old proxy key: %w", err)
	}

	// Insert new proxy key
	insertQuery := fmt.Sprintf(`
		INSERT INTO proxy_keys (proxy_key, license_id, hardware_id, created_at)
		VALUES (%s, %s, %s, CURRENT_TIMESTAMP)
	`, db.Placeholder(1), db.Placeholder(2), db.Placeholder(3))

	_, err = tx.Exec(insertQuery, proxyKey, licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to insert proxy key: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ValidateProxyKey validates a proxy key and returns associated license and hardware IDs
func (db *DB) ValidateProxyKey(proxyKey string) (licenseID, hardwareID string, err error) {
	query := fmt.Sprintf(`
		SELECT license_id, hardware_id FROM proxy_keys 
		WHERE proxy_key = %s
	`, db.Placeholder(1))

	err = db.QueryRow(query, proxyKey).Scan(&licenseID, &hardwareID)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("proxy key not found")
	}

	return licenseID, hardwareID, err
}
