package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

const (
	DefaultPort = "8080"
	DBFile      = "licensify.db"
)

var (
	db         *sql.DB
	privateKey ed25519.PrivateKey

	// Build information (set via ldflags)
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Config represents server configuration
type Config struct {
	Port            string
	PrivateKeyB64   string
	ProtectedAPIKey string
	DatabasePath    string
	DatabaseURL     string
	ResendAPIKey    string
	FromEmail       string
}

// LicenseData represents license information
type LicenseData struct {
	LicenseID     string    `json:"license_id"`
	CustomerName  string    `json:"customer_name"`
	CustomerEmail string    `json:"customer_email"`
	ExpiresAt     time.Time `json:"expires_at"`
	Tier          string    `json:"tier"`
	Limits        struct {
		DailyLimit     int `json:"daily_limit"`
		MonthlyLimit   int `json:"monthly_limit"`
		MaxActivations int `json:"max_activations"`
	} `json:"limits"`
	Active bool `json:"active"`
}

// ActivationRequest from CLI
type ActivationRequest struct {
	LicenseKey string `json:"license_key"`
	HardwareID string `json:"hardware_id"`
	Timestamp  string `json:"timestamp"`
}

// ActivationResponse to CLI
type ActivationResponse struct {
	Success         bool      `json:"success"`
	CustomerName    string    `json:"customer_name,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	Tier            string    `json:"tier,omitempty"`
	EncryptedAPIKey string    `json:"encrypted_api_key,omitempty"`
	IV              string    `json:"iv,omitempty"`
	Limits          struct {
		DailyLimit     int `json:"daily_limit"`
		MonthlyLimit   int `json:"monthly_limit"`
		MaxActivations int `json:"max_activations"`
	} `json:"limits,omitempty"`
	Error string `json:"error,omitempty"`
}

// InitRequest for free tier onboarding
type InitRequest struct {
	Email string `json:"email"`
}

// InitResponse with verification code
type InitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Email   string `json:"email,omitempty"`
	Error   string `json:"error,omitempty"`
}

// VerifyRequest for email verification
type VerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// VerifyResponse with license key
type VerifyResponse struct {
	Success    bool   `json:"success"`
	LicenseKey string `json:"license_key,omitempty"`
	Tier       string `json:"tier,omitempty"`
	DailyLimit int    `json:"daily_limit,omitempty"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

// UsageReport from CLI
type UsageReport struct {
	LicenseKey string `json:"license_key"`
	Date       string `json:"date"` // YYYY-MM-DD
	Scans      int    `json:"scans"`
	HardwareID string `json:"hardware_id"`
}

// UsageResponse to CLI
type UsageResponse struct {
	Success      bool   `json:"success"`
	DailyUsage   int    `json:"daily_usage,omitempty"`
	MonthlyUsage int    `json:"monthly_usage,omitempty"`
	DailyLimit   int    `json:"daily_limit,omitempty"`
	MonthlyLimit int    `json:"monthly_limit,omitempty"`
	Tier         string `json:"tier,omitempty"`
	Error        string `json:"error,omitempty"`
}

// DecryptedData represents the data bundle sent to client
type DecryptedData struct {
	APIKey       string    `json:"api_key"`
	CustomerName string    `json:"customer_name"`
	ExpiresAt    time.Time `json:"expires_at"`
	Tier         string    `json:"tier"`
	Limits       struct {
		DailyLimit     int `json:"daily_limit"`
		MonthlyLimit   int `json:"monthly_limit"`
		MaxActivations int `json:"max_activations"`
	} `json:"limits"`
}

func loadConfig() *Config {
	return &Config{
		Port:            getEnv("PORT", DefaultPort),
		PrivateKeyB64:   getEnv("PRIVATE_KEY", ""),
		ProtectedAPIKey: getEnv("PROTECTED_API_KEY", ""),
		DatabasePath:    getEnv("DB_PATH", DBFile),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		ResendAPIKey:    getEnv("RESEND_API_KEY", ""),
		FromEmail:       getEnv("FROM_EMAIL", "noreply@licensify.com"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func initDB(dbPath, dbURL string) error {
	var err error
	var driverName, dataSource string
	var isPostgres bool

	// Detect database type
	if dbURL != "" {
		// PostgreSQL
		driverName = "postgres"
		dataSource = dbURL
		isPostgres = true
		log.Printf("📊 Using PostgreSQL database")
	} else {
		// SQLite
		driverName = "sqlite"
		dataSource = dbPath
		isPostgres = false
		log.Printf("📊 Using SQLite database: %s", dbPath)
	}

	db, err = sql.Open(driverName, dataSource)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables with appropriate syntax
	var schema string
	if isPostgres {
		schema = `
		CREATE TABLE IF NOT EXISTS licenses (
			license_id TEXT PRIMARY KEY,
			customer_name TEXT NOT NULL,
			customer_email TEXT NOT NULL,
			tier TEXT NOT NULL DEFAULT 'free',
			expires_at TIMESTAMP NOT NULL,
			daily_limit INTEGER NOT NULL,
			monthly_limit INTEGER NOT NULL,
			max_activations INTEGER NOT NULL,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS activations (
			id SERIAL PRIMARY KEY,
			license_id TEXT NOT NULL,
			hardware_id TEXT NOT NULL,
			activated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (license_id) REFERENCES licenses(license_id)
		);

		CREATE TABLE IF NOT EXISTS verification_codes (
			email TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS daily_usage (
			id SERIAL PRIMARY KEY,
			license_id TEXT NOT NULL,
			date TEXT NOT NULL,
			scans INTEGER DEFAULT 0,
			hardware_id TEXT NOT NULL,
			UNIQUE(license_id, date),
			FOREIGN KEY (license_id) REFERENCES licenses(license_id)
		);

		CREATE TABLE IF NOT EXISTS check_ins (
			license_id TEXT NOT NULL PRIMARY KEY,
			last_check_in TIMESTAMP NOT NULL,
			FOREIGN KEY (license_id) REFERENCES licenses(license_id)
		);

		CREATE INDEX IF NOT EXISTS idx_license_id ON activations(license_id);
		CREATE INDEX IF NOT EXISTS idx_hardware_id ON activations(hardware_id);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_license ON daily_usage(license_id);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(date);
		`
	} else {
		schema = `
		CREATE TABLE IF NOT EXISTS licenses (
			license_id TEXT PRIMARY KEY,
			customer_name TEXT NOT NULL,
			customer_email TEXT NOT NULL,
			tier TEXT NOT NULL DEFAULT 'free',
			expires_at DATETIME NOT NULL,
			daily_limit INTEGER NOT NULL,
			monthly_limit INTEGER NOT NULL,
			max_activations INTEGER NOT NULL,
			active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS activations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			license_id TEXT NOT NULL,
			hardware_id TEXT NOT NULL,
			activated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (license_id) REFERENCES licenses(license_id)
		);

		CREATE TABLE IF NOT EXISTS verification_codes (
			email TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS daily_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			license_id TEXT NOT NULL,
			date TEXT NOT NULL,
			scans INTEGER DEFAULT 0,
			hardware_id TEXT NOT NULL,
			UNIQUE(license_id, date),
			FOREIGN KEY (license_id) REFERENCES licenses(license_id)
		);

		CREATE TABLE IF NOT EXISTS check_ins (
			license_id TEXT NOT NULL,
			last_check_in DATETIME NOT NULL,
			PRIMARY KEY (license_id),
			FOREIGN KEY (license_id) REFERENCES licenses(license_id)
		);

		CREATE INDEX IF NOT EXISTS idx_license_id ON activations(license_id);
		CREATE INDEX IF NOT EXISTS idx_hardware_id ON activations(hardware_id);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_license ON daily_usage(license_id);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(date);
		`
	}

	_, err = db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "ok",
		"service":    "licensify",
		"version":    Version,
		"git_commit": GitCommit,
		"build_time": BuildTime,
	})
}

func handleInit(resendAPIKey, fromEmail string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req InitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate email
		if !strings.Contains(req.Email, "@") {
			sendError(w, "Invalid email address", http.StatusBadRequest)
			return
		}

		// Generate 6-digit code
		code, err := generateVerificationCode()
		if err != nil {
			log.Printf("Failed to generate code: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Store code (expires in 15 minutes)
		expiresAt := time.Now().Add(15 * time.Minute)

		// Delete existing code if any
		_, _ = db.Exec(`DELETE FROM verification_codes WHERE email = $1`, req.Email)

		// Insert new code
		_, err = db.Exec(`
			INSERT INTO verification_codes (email, code, created_at, expires_at) 
			VALUES ($1, $2, CURRENT_TIMESTAMP, $3)
		`, req.Email, code, expiresAt)
		if err != nil {
			log.Printf("Failed to store verification code: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Send email via Resend
		if err := sendVerificationEmail(resendAPIKey, fromEmail, req.Email, code); err != nil {
			log.Printf("Failed to send verification email: %v", err)
			sendError(w, "Failed to send verification email", http.StatusInternalServerError)
			return
		}

		log.Printf("Sent verification code to %s", req.Email)

		resp := InitResponse{
			Success: true,
			Message: "Verification code sent to your email",
			Email:   req.Email,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleVerify(resendAPIKey, fromEmail string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Verify code
		var storedCode string
		var expiresAt time.Time
		err := db.QueryRow(`
			SELECT code, expires_at FROM verification_codes 
			WHERE email = ?
		`, req.Email).Scan(&storedCode, &expiresAt)

		if err == sql.ErrNoRows {
			sendError(w, "No verification code found for this email", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("Database error: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if time.Now().After(expiresAt) {
			sendError(w, "Verification code expired", http.StatusBadRequest)
			return
		}

		if storedCode != req.Code {
			sendError(w, "Invalid verification code", http.StatusUnauthorized)
			return
		}

		// Check if user already has a license
		var existingLicense string
		err = db.QueryRow(`
			SELECT license_id FROM licenses WHERE customer_email = ?
		`, req.Email).Scan(&existingLicense)

		if err == nil {
			// User already has a license
			resp := VerifyResponse{
				Success:    true,
				LicenseKey: existingLicense,
				Tier:       "free",
				DailyLimit: 10,
				Message:    "Email verified! Your existing license key is ready.",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Generate FREE license
		licenseKey := generateLicenseKey()
		expiresAtLicense := time.Now().AddDate(0, 1, 0) // 1 month for free tier

		_, err = db.Exec(`
			INSERT INTO licenses (
license_id, customer_name, customer_email, tier, 
expires_at, daily_limit, monthly_limit, max_activations, active
) VALUES (?, ?, ?, 'free', ?, 10, 10, 3, 1)
		`, licenseKey, req.Email, req.Email, expiresAtLicense)

		if err != nil {
			log.Printf("Failed to create license: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Delete verification code
		db.Exec("DELETE FROM verification_codes WHERE email = ?", req.Email)

		// Send license email
		if err := sendLicenseEmail(resendAPIKey, fromEmail, req.Email, licenseKey, "free", 10); err != nil {
			log.Printf("Failed to send license email: %v", err)
			// Don't fail - license is already created
		}

		log.Printf("Created FREE license for %s: %s", req.Email, licenseKey)

		resp := VerifyResponse{
			Success:    true,
			LicenseKey: licenseKey,
			Tier:       "free",
			DailyLimit: 10,
			Message:    "Email verified! Your FREE license is ready.",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleActivation(protectedAPIKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ActivationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		log.Printf("Activation request: license=%s, hardware=%s", req.LicenseKey, req.HardwareID[:8]+"...")

		// Validate license key exists
		license, err := getLicense(req.LicenseKey)
		if err != nil {
			log.Printf("License not found: %v", err)
			sendError(w, "Invalid license key", http.StatusUnauthorized)
			return
		}

		// For FREE tier: Check if this hardware already has an active free license
		if license.Tier == "free" && isFreeHardwareAlreadyActive(req.HardwareID, req.LicenseKey) {
			log.Printf("Hardware %s already has an active free license, blocking new free license %s", req.HardwareID[:8]+"...", req.LicenseKey)
			sendError(w, "This device already has an active FREE license. Each device is limited to one free license.", http.StatusForbidden)
			return
		}

		// Check if license is active
		if !license.Active {
			sendError(w, "License has been deactivated", http.StatusForbidden)
			return
		}

		// Check if expired
		if time.Now().After(license.ExpiresAt) {
			sendError(w, "License has expired", http.StatusForbidden)
			return
		}

		// Check activation count
		count, err := getActivationCount(req.LicenseKey)
		if err != nil {
			log.Printf("Error checking activations: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if count >= license.Limits.MaxActivations {
			sendError(w, fmt.Sprintf("Maximum activations (%d) reached", license.Limits.MaxActivations), http.StatusForbidden)
			return
		}

		// Check if already activated on this hardware
		alreadyActivated, err := isHardwareActivated(req.LicenseKey, req.HardwareID)
		if err != nil {
			log.Printf("Error checking hardware: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Record activation if new hardware
		if !alreadyActivated {
			if err := recordActivation(req.LicenseKey, req.HardwareID); err != nil {
				log.Printf("Error recording activation: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			log.Printf("New activation recorded for license %s", req.LicenseKey)
		} else {
			log.Printf("Re-activation on existing hardware for license %s", req.LicenseKey)
		}

		// Record check-in
		recordCheckIn(req.LicenseKey)

		// Encrypt API key bundle
		encryptedData, iv, err := encryptAPIKeyBundle(protectedAPIKey, license, req.LicenseKey, req.HardwareID)
		if err != nil {
			log.Printf("Encryption error: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Send response
		resp := ActivationResponse{
			Success:         true,
			CustomerName:    license.CustomerName,
			ExpiresAt:       license.ExpiresAt,
			Tier:            license.Tier,
			EncryptedAPIKey: encryptedData,
			IV:              iv,
			Limits: struct {
				DailyLimit     int `json:"daily_limit"`
				MonthlyLimit   int `json:"monthly_limit"`
				MaxActivations int `json:"max_activations"`
			}{
				DailyLimit:     license.Limits.DailyLimit,
				MonthlyLimit:   license.Limits.MonthlyLimit,
				MaxActivations: license.Limits.MaxActivations,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		log.Printf("✅ Activation successful for %s", req.LicenseKey)
	}
}

func handleUsageReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req UsageReport
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate license exists
		license, err := getLicense(req.LicenseKey)
		if err != nil {
			sendError(w, "Invalid license key", http.StatusUnauthorized)
			return
		}

		// Record check-in
		recordCheckIn(req.LicenseKey)

		// Update usage
		_, err = db.Exec(`
INSERT INTO daily_usage (license_id, date, scans, hardware_id) 
VALUES (?, ?, ?, ?)
ON CONFLICT(license_id, date) DO UPDATE SET 
scans = scans + excluded.scans
`, req.LicenseKey, req.Date, req.Scans, req.HardwareID)

		if err != nil {
			log.Printf("Failed to record usage: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Get current usage
		dailyUsage, monthlyUsage := getUsage(req.LicenseKey, req.Date)

		resp := UsageResponse{
			Success:      true,
			DailyUsage:   dailyUsage,
			MonthlyUsage: monthlyUsage,
			DailyLimit:   license.Limits.DailyLimit,
			MonthlyLimit: license.Limits.MonthlyLimit,
			Tier:         license.Tier,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func getLicense(licenseID string) (*LicenseData, error) {
	var license LicenseData
	license.LicenseID = licenseID

	err := db.QueryRow(`
SELECT customer_name, customer_email, tier, expires_at, 
       daily_limit, monthly_limit, max_activations, active
FROM licenses WHERE license_id = ?
`, licenseID).Scan(
		&license.CustomerName,
		&license.CustomerEmail,
		&license.Tier,
		&license.ExpiresAt,
		&license.Limits.DailyLimit,
		&license.Limits.MonthlyLimit,
		&license.Limits.MaxActivations,
		&license.Active,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("license not found")
	}

	return &license, err
}

func getActivationCount(licenseID string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM activations WHERE license_id = ?", licenseID).Scan(&count)
	return count, err
}

func isHardwareActivated(licenseID, hardwareID string) (bool, error) {
	var count int
	err := db.QueryRow(`
SELECT COUNT(*) FROM activations 
WHERE license_id = ? AND hardware_id = ?
`, licenseID, hardwareID).Scan(&count)
	return count > 0, err
}

func recordActivation(licenseID, hardwareID string) error {
	_, err := db.Exec(`
INSERT INTO activations (license_id, hardware_id) 
VALUES (?, ?)
`, licenseID, hardwareID)
	return err
}

func isFreeHardwareAlreadyActive(hardwareID, requestedLicenseID string) bool {
	var count int
	err := db.QueryRow(`
SELECT COUNT(DISTINCT a.license_id) 
FROM activations a
JOIN licenses l ON a.license_id = l.license_id
WHERE a.hardware_id = ? 
  AND l.tier = 'free' 
  AND l.active = 1 
  AND l.expires_at > CURRENT_TIMESTAMP
  AND a.license_id != ?
`, hardwareID, requestedLicenseID).Scan(&count)

	if err != nil {
		log.Printf("Error checking free hardware: %v", err)
		return false
	}

	return count > 0
}

func recordCheckIn(licenseID string) {
	db.Exec(`
INSERT INTO check_ins (license_id, last_check_in) 
VALUES (?, CURRENT_TIMESTAMP)
ON CONFLICT(license_id) DO UPDATE SET 
last_check_in = CURRENT_TIMESTAMP
`, licenseID)
}

func getUsage(licenseID, date string) (int, int) {
	var dailyUsage int
	db.QueryRow(`
SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
WHERE license_id = ? AND date = ?
`, licenseID, date).Scan(&dailyUsage)

	// Monthly usage (current month)
	var monthlyUsage int
	yearMonth := date[:7] // YYYY-MM
	db.QueryRow(`
SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
WHERE license_id = ? AND date LIKE ?
`, licenseID, yearMonth+"%").Scan(&monthlyUsage)

	return dailyUsage, monthlyUsage
}

func encryptAPIKeyBundle(protectedAPIKey string, license *LicenseData, licenseKey, hwID string) (string, string, error) {
	// Prepare bundle
	bundle := DecryptedData{
		APIKey:       protectedAPIKey,
		CustomerName: license.CustomerName,
		ExpiresAt:    license.ExpiresAt,
		Tier:         license.Tier,
		Limits: struct {
			DailyLimit     int `json:"daily_limit"`
			MonthlyLimit   int `json:"monthly_limit"`
			MaxActivations int `json:"max_activations"`
		}{
			DailyLimit:     license.Limits.DailyLimit,
			MonthlyLimit:   license.Limits.MonthlyLimit,
			MaxActivations: license.Limits.MaxActivations,
		},
	}

	// Serialize
	plaintext, err := json.Marshal(bundle)
	if err != nil {
		return "", "", err
	}

	// Derive key from license + hardware ID (same as client)
	key := deriveKey(licenseKey, hwID)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Encode to base64
	encrypted := base64.StdEncoding.EncodeToString(ciphertext)
	iv := base64.StdEncoding.EncodeToString(nonce)

	return encrypted, iv, nil
}

func deriveKey(licenseKey, hardwareID string) []byte {
	h := sha256.New()
	h.Write([]byte(licenseKey))
	h.Write([]byte(hardwareID))
	return h.Sum(nil)
}

func sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ActivationResponse{
		Success: false,
		Error:   message,
	})
}

func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func generateLicenseKey() string {
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

func sendVerificationEmail(apiKey, fromEmail, toEmail, code string) error {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
        .code { 
            font-size: 32px; 
            font-weight: bold; 
            letter-spacing: 8px; 
            text-align: center;
            background: #f5f5f5;
            padding: 20px;
            border-radius: 8px;
            margin: 30px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧾 Verify Your Email</h1>
        <p>Your verification code is:</p>
        <div class="code">%s</div>
        <p>Run: <code>licensify init --email=%s --verify=%s</code></p>
        <p><strong>Free Tier: 10 scans/day</strong></p>
    </div>
</body>
</html>
`, code, toEmail, code)

	return sendResendEmail(apiKey, fromEmail, toEmail, "Verify Your Email - Licensify", html)
}

func sendLicenseEmail(apiKey, fromEmail, toEmail, licenseKey, tier string, dailyLimit int) error {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
        .license-key {
            font-size: 18px;
            font-weight: bold;
            font-family: monospace;
            background: #f0f9ff;
            padding: 20px;
            border-radius: 8px;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎉 Your Licensify License</h1>
        <p>Your license key:</p>
        <div class="license-key">%s</div>
        <p><strong>Tier:</strong> %s | <strong>Daily Limit:</strong> %d scans</p>
        <p>Quick start: <code>licensify activate %s</code></p>
    </div>
</body>
</html>
`, licenseKey, strings.ToUpper(tier), dailyLimit, licenseKey)

	return sendResendEmail(apiKey, fromEmail, toEmail, "Your Licensify License Key", html)
}

func sendResendEmail(apiKey, fromEmail, toEmail, subject, html string) error {
	payload := map[string]interface{}{
		"from":    fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    html,
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.resend.com/emails", strings.NewReader(string(jsonData)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error: %s", body)
	}

	return nil
}

func main() {
	// Load .env file (ignore error if doesn't exist)
	_ = godotenv.Load()

	// Load configuration
	config := loadConfig()

	// Initialize database
	if err := initDB(config.DatabasePath, config.DatabaseURL); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Load private key
	privKeyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKeyB64)
	if err != nil {
		log.Fatalf("Failed to decode private key: %v", err)
	}
	privateKey = ed25519.PrivateKey(privKeyBytes)

	// Setup HTTP routes
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/init", handleInit(config.ResendAPIKey, config.FromEmail))
	http.HandleFunc("/verify", handleVerify(config.ResendAPIKey, config.FromEmail))
	http.HandleFunc("/activate", handleActivation(config.ProtectedAPIKey))
	http.HandleFunc("/usage", handleUsageReport())

	addr := ":" + config.Port
	log.Printf("🚀 Activation server starting on %s", addr)

	// Log database information
	if config.DatabaseURL != "" {
		// Extract host from PostgreSQL URL for logging (hide password)
		dbInfo := "PostgreSQL"
		if strings.Contains(config.DatabaseURL, "@") {
			parts := strings.Split(config.DatabaseURL, "@")
			if len(parts) > 1 {
				dbInfo = "PostgreSQL (" + strings.Split(parts[1], "/")[0] + ")"
			}
		}
		log.Printf("📊 Database: %s", dbInfo)
	} else {
		log.Printf("📊 Database: SQLite (%s)", config.DatabasePath)
	}

	log.Printf("📧 Email: %s (Resend)", config.FromEmail)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
