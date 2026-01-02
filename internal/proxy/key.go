package proxy

import (
"crypto/rand"
"database/sql"
"encoding/base64"
"fmt"
)

type KeyManager struct {
	db          *sql.DB
	placeholder func(int) string
}

func NewKeyManager(db *sql.DB, placeholder func(int) string) *KeyManager {
	return &KeyManager{db: db, placeholder: placeholder}
}

func (km *KeyManager) Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "px_" + base64.URLEncoding.EncodeToString(b)[:43], nil
}

func (km *KeyManager) Store(proxyKey, licenseID, hardwareID string) error {
	tx, err := km.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(fmt.Sprintf(`DELETE FROM proxy_keys WHERE license_id = %s AND hardware_id = %s`, km.placeholder(1), km.placeholder(2)), licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to delete old proxy key: %w", err)
	}

	_, err = tx.Exec(fmt.Sprintf(`INSERT INTO proxy_keys (proxy_key, license_id, hardware_id) VALUES (%s, %s, %s)`, km.placeholder(1), km.placeholder(2), km.placeholder(3)), proxyKey, licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to insert proxy key: %w", err)
	}

	return tx.Commit()
}

func (km *KeyManager) Validate(proxyKey string) (licenseID, hardwareID string, err error) {
	err = km.db.QueryRow(fmt.Sprintf(`SELECT license_id, hardware_id FROM proxy_keys WHERE proxy_key = %s`, km.placeholder(1)), proxyKey).Scan(&licenseID, &hardwareID)
	return
}
