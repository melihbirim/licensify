package database

import (
	"database/sql"
	"fmt"
	"time"
)

// VerificationCode represents a verification code record
type VerificationCode struct {
	Email     string
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// StoreVerificationCode stores a new verification code, replacing any existing one
func (db *DB) StoreVerificationCode(email, code string, expiresAt time.Time) error {
	// Delete existing code if any
	deleteQuery := fmt.Sprintf(`DELETE FROM verification_codes WHERE email = %s`, db.Placeholder(1))
	_, _ = db.Exec(deleteQuery, email)

	// Insert new code
	insertQuery := fmt.Sprintf(`
		INSERT INTO verification_codes (email, code, created_at, expires_at) 
		VALUES (%s, %s, CURRENT_TIMESTAMP, %s)
	`, db.Placeholder(1), db.Placeholder(2), db.Placeholder(3))

	_, err := db.Exec(insertQuery, email, code, expiresAt.Format(time.RFC3339))
	return err
}

// GetVerificationCode retrieves a verification code for an email
func (db *DB) GetVerificationCode(email string) (*VerificationCode, error) {
	var code VerificationCode
	var expiresAtStr string

	query := fmt.Sprintf(`
		SELECT code, expires_at FROM verification_codes 
		WHERE email = %s
	`, db.Placeholder(1))

	err := db.QueryRow(query, email).Scan(&code.Code, &expiresAtStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no verification code found")
	}
	if err != nil {
		return nil, err
	}

	code.Email = email
	code.ExpiresAt, err = time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expiration time: %w", err)
	}

	return &code, nil
}

// DeleteVerificationCode deletes a verification code for an email
func (db *DB) DeleteVerificationCode(email string) error {
	query := fmt.Sprintf("DELETE FROM verification_codes WHERE email = %s", db.Placeholder(1))
	_, err := db.Exec(query, email)
	return err
}

// IsVerificationCodeValid checks if a code is valid and not expired
func (db *DB) IsVerificationCodeValid(email, code string) (bool, error) {
	verificationCode, err := db.GetVerificationCode(email)
	if err != nil {
		return false, err
	}

	// Check if expired
	if time.Now().After(verificationCode.ExpiresAt) {
		return false, fmt.Errorf("verification code expired")
	}

	// Check if code matches
	if verificationCode.Code != code {
		return false, fmt.Errorf("invalid verification code")
	}

	return true, nil
}
