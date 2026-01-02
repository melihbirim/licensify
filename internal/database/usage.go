package database

import (
	"fmt"
)

// RecordUsage records or updates usage for a license on a specific date
func (db *DB) RecordUsage(licenseID, date string, scans int, hardwareID string) error {
	query := fmt.Sprintf(`
		INSERT INTO daily_usage (license_id, date, scans, hardware_id) 
		VALUES (%s, %s, %s, %s)
		ON CONFLICT(license_id, date) DO UPDATE SET 
		scans = scans + excluded.scans
	`, db.Placeholder(1), db.Placeholder(2), db.Placeholder(3), db.Placeholder(4))

	_, err := db.Exec(query, licenseID, date, scans, hardwareID)
	return err
}

// GetDailyUsage returns the total scans for a license on a specific date
func (db *DB) GetDailyUsage(licenseID, date string) (int, error) {
	var dailyUsage int
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
		WHERE license_id = %s AND date = %s
	`, db.Placeholder(1), db.Placeholder(2))

	err := db.QueryRow(query, licenseID, date).Scan(&dailyUsage)
	return dailyUsage, err
}

// GetMonthlyUsage returns the total scans for a license in a specific month (YYYY-MM)
func (db *DB) GetMonthlyUsage(licenseID, yearMonth string) (int, error) {
	var monthlyUsage int
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
		WHERE license_id = %s AND date LIKE %s
	`, db.Placeholder(1), db.Placeholder(2))

	err := db.QueryRow(query, licenseID, yearMonth+"%").Scan(&monthlyUsage)
	return monthlyUsage, err
}

// GetUsage returns both daily and monthly usage for a license
func (db *DB) GetUsage(licenseID, date string) (dailyUsage, monthlyUsage int) {
	// Daily usage
	daily, err := db.GetDailyUsage(licenseID, date)
	if err == nil {
		dailyUsage = daily
	}

	// Monthly usage (current month)
	yearMonth := date[:7] // YYYY-MM
	monthly, err := db.GetMonthlyUsage(licenseID, yearMonth)
	if err == nil {
		monthlyUsage = monthly
	}

	return dailyUsage, monthlyUsage
}

// GetDailyUsageByHardware returns the usage for a specific license, date, and hardware ID
func (db *DB) GetDailyUsageByHardware(licenseID, date, hardwareID string) (int, error) {
	var currentUsage int
	query := fmt.Sprintf(`
		SELECT scans FROM daily_usage 
		WHERE license_id = %s AND date = %s AND hardware_id = %s
	`, db.Placeholder(1), db.Placeholder(2), db.Placeholder(3))

	err := db.QueryRow(query, licenseID, date, hardwareID).Scan(&currentUsage)
	return currentUsage, err
}
