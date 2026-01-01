package main

import (
	"context"
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
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/melihbirim/licensify/internal/tiers"
	"golang.org/x/time/rate"
	_ "modernc.org/sqlite"
)

const (
	DefaultPort = "8080"
	DBFile      = "licensify.db"
)

var (
	db           *sql.DB
	privateKey   ed25519.PrivateKey
	isPostgresDB bool // Track database type

	// Build information (set via ldflags)
	Version   = "1.1.0"
	GitCommit = "unknown"
	BuildTime = "unknown"

	// Rate limiting
	ipLimiters       = make(map[string]*rate.Limiter)
	ipLimitersMu     sync.RWMutex
	ipLimiterCleanup = 5 * time.Minute // Cleanup interval for rate limiters
)

// sqlPlaceholder returns the correct SQL placeholder for the database type
func sqlPlaceholder(n int) string {
	if isPostgresDB {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// redactPII returns a redacted version of sensitive data for logging
// Shows first 4 and last 4 characters for identification without full exposure
func redactPII(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// redactEmail redacts email addresses for logging
func redactEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		return "***@" + domain
	}
	return username[:min(2, len(username))] + "***@" + domain
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getIPLimiter returns rate limiter for IP address
func getIPLimiter(ip string) *rate.Limiter {
	ipLimitersMu.RLock()
	limiter, exists := ipLimiters[ip]
	ipLimitersMu.RUnlock()

	if !exists {
		ipLimitersMu.Lock()
		limiter = rate.NewLimiter(rate.Limit(10), 20) // 10 req/sec, burst 20
		ipLimiters[ip] = limiter
		ipLimitersMu.Unlock()
	}

	return limiter
}

// cleanupIPLimiters periodically removes inactive limiters to prevent memory leaks
func cleanupIPLimiters(ctx context.Context) {
	ticker := time.NewTicker(ipLimiterCleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ipLimitersMu.Lock()
			// Remove limiters that have had no recent activity
			for ip, limiter := range ipLimiters {
				// If limiter has full tokens (unused), remove it
				if limiter.Tokens() >= 20 {
					delete(ipLimiters, ip)
				}
			}
			ipLimitersMu.Unlock()
		}
	}
}

// rateLimitMiddleware enforces per-IP rate limiting
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr // Fallback if port parsing fails
		}

		// Check X-Forwarded-For header for proxied requests
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			ip = strings.TrimSpace(parts[0])
		}

		limiter := getIPLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			sendError(w, "Too many requests from this IP", http.StatusTooManyRequests)
			log.Printf("Rate limit exceeded for IP: %s", ip)
			return
		}

		next(w, r)
	}
}

// Config represents server configuration
type Config struct {
	Port            string
	PrivateKeyB64   string
	ProtectedAPIKey string
	DatabasePath    string
	DatabaseURL     string
	ResendAPIKey    string
	FromEmail       string
	ProxyMode       bool
	OpenAIKey       string
	AnthropicKey    string
	TiersConfigPath string
	ShutdownTimeout time.Duration
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
	proxyMode := getEnv("PROXY_MODE", "false") == "true"

	// Parse shutdown timeout with default of 30 seconds
	shutdownTimeout := 30 * time.Second
	if timeoutStr := getEnv("SHUTDOWN_TIMEOUT", ""); timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			shutdownTimeout = parsed
		} else {
			log.Printf("⚠️  Invalid SHUTDOWN_TIMEOUT format, using default 30s")
		}
	}

	return &Config{
		Port:            getEnv("PORT", DefaultPort),
		DatabasePath:    getEnv("DB_PATH", DBFile),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		PrivateKeyB64:   getEnv("PRIVATE_KEY", ""),
		ResendAPIKey:    getEnv("RESEND_API_KEY", ""),
		FromEmail:       getEnv("FROM_EMAIL", ""),
		ProtectedAPIKey: getEnv("PROTECTED_API_KEY", ""),
		ProxyMode:       proxyMode,
		OpenAIKey:       getEnv("OPENAI_API_KEY", ""),
		AnthropicKey:    getEnv("ANTHROPIC_API_KEY", ""),
		TiersConfigPath: getEnv("TIERS_CONFIG_PATH", "tiers.toml"),
		ShutdownTimeout: shutdownTimeout,
	}
}

// validateConfig checks that required configuration is present and valid
func validateConfig(config *Config) error {
	var errors []string

	// Required: Private key for license signing
	if config.PrivateKeyB64 == "" {
		errors = append(errors, "PRIVATE_KEY is required for license signature verification")
	} else {
		// Validate it's valid base64 and correct length
		keyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKeyB64)
		if err != nil {
			errors = append(errors, fmt.Sprintf("PRIVATE_KEY is not valid base64: %v", err))
		} else if len(keyBytes) != ed25519.PrivateKeySize {
			errors = append(errors, fmt.Sprintf("PRIVATE_KEY has invalid length: got %d, want %d bytes", len(keyBytes), ed25519.PrivateKeySize))
		}
	}

	// Required for direct mode: Protected API key
	if !config.ProxyMode && config.ProtectedAPIKey == "" {
		errors = append(errors, "PROTECTED_API_KEY is required when PROXY_MODE=false")
	}

	// Required for proxy mode: At least one upstream API key
	if config.ProxyMode && config.OpenAIKey == "" && config.AnthropicKey == "" {
		errors = append(errors, "PROXY_MODE=true requires at least one of OPENAI_API_KEY or ANTHROPIC_API_KEY")
	}

	// Email configuration for verification (warn only, not fatal)
	if config.ResendAPIKey == "" {
		log.Printf("⚠️  RESEND_API_KEY not set - email verification will fail")
	}
	if config.FromEmail == "" {
		log.Printf("⚠️  FROM_EMAIL not set - email verification will fail")
	}

	// Database configuration
	if config.DatabaseURL == "" && config.DatabasePath == "" {
		errors = append(errors, "Either DATABASE_URL (PostgreSQL) or DB_PATH (SQLite) must be set")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
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

	// Detect database type
	if dbURL != "" {
		// PostgreSQL
		driverName = "postgres"
		dataSource = dbURL
		isPostgresDB = true
		log.Printf("📊 Using PostgreSQL database")
	} else {
		// SQLite
		driverName = "sqlite"
		dataSource = dbPath
		isPostgresDB = false
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

	// Enable WAL mode for SQLite for better concurrency and durability
	if !isPostgresDB {
		pragmas := []string{
			"PRAGMA journal_mode=WAL;",   // Write-Ahead Logging for better concurrency
			"PRAGMA synchronous=NORMAL;", // Balance between safety and performance
			"PRAGMA foreign_keys=ON;",    // Enforce foreign key constraints
			"PRAGMA busy_timeout=5000;",  // Wait up to 5s if database is locked
			"PRAGMA cache_size=-64000;",  // 64MB cache
		}
		for _, pragma := range pragmas {
			if _, err := db.Exec(pragma); err != nil {
				log.Printf("⚠️  Failed to set SQLite pragma: %v", err)
			}
		}
		log.Printf("📊 SQLite WAL mode enabled for better concurrency")
	}

	// Load and execute schema from SQL files
	var schemaPath string
	if isPostgresDB {
		schemaPath = "sql/postgres/init.sql"
	} else {
		schemaPath = "sql/sqlite/init.sql"
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	_, err = db.Exec(string(schema))
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

// TierInfo represents public tier information
type TierInfo struct {
	Name                      string   `json:"name"`
	DailyLimit                int      `json:"daily_limit"`
	MonthlyLimit              int      `json:"monthly_limit"`
	MaxDevices                int      `json:"max_devices"`
	Features                  []string `json:"features"`
	Description               string   `json:"description"`
	PriceMonthly              float64  `json:"price_monthly,omitempty"`
	OneTimePayment            float64  `json:"one_time_payment,omitempty"`
	CustomPricing             bool     `json:"custom_pricing,omitempty"`
	EmailVerificationRequired bool     `json:"email_verification_required"`
}

func handleTiers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all visible tiers
	allTiers := tiers.GetAllVisible()

	// Convert to response format
	response := make(map[string]TierInfo)
	for name, tier := range allTiers {
		response[name] = TierInfo{
			Name:                      tier.Name,
			DailyLimit:                tier.DailyLimit,
			MonthlyLimit:              tier.MonthlyLimit,
			MaxDevices:                tier.MaxDevices,
			Features:                  tier.Features,
			Description:               tier.Description,
			PriceMonthly:              tier.PriceMonthly,
			OneTimePayment:            tier.OneTimePayment,
			CustomPricing:             tier.CustomPricing,
			EmailVerificationRequired: tier.EmailVerificationRequired,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"tiers":   response,
	})
}

// CheckRequest from CLI to check license status
type CheckRequest struct {
	LicenseKey string `json:"license_key"`
}

// CheckResponse with current license status
type CheckResponse struct {
	Success       bool      `json:"success"`
	CustomerName  string    `json:"customer_name,omitempty"`
	CustomerEmail string    `json:"customer_email,omitempty"`
	Tier          string    `json:"tier,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	Active        bool      `json:"active"`
	Limits        struct {
		DailyLimit     int `json:"daily_limit"`
		MonthlyLimit   int `json:"monthly_limit"`
		MaxActivations int `json:"max_activations"`
	} `json:"limits,omitempty"`
	CurrentActivations int    `json:"current_activations,omitempty"`
	Error              string `json:"error,omitempty"`
}

func handleCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.LicenseKey == "" {
			sendError(w, "License key is required", http.StatusBadRequest)
			return
		}

		// Get license from database
		license, err := getLicense(req.LicenseKey)
		if err != nil {
			sendError(w, "Invalid license key", http.StatusUnauthorized)
			return
		}

		// Get activation count
		count, err := getActivationCount(req.LicenseKey)
		if err != nil {
			log.Printf("Error checking activations: %v", err)
			count = 0
		}

		resp := CheckResponse{
			Success:            true,
			CustomerName:       license.CustomerName,
			CustomerEmail:      license.CustomerEmail,
			Tier:               license.Tier,
			ExpiresAt:          license.ExpiresAt,
			Active:             license.Active,
			CurrentActivations: count,
		}
		resp.Limits.DailyLimit = license.Limits.DailyLimit
		resp.Limits.MonthlyLimit = license.Limits.MonthlyLimit
		resp.Limits.MaxActivations = license.Limits.MaxActivations

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

		log.Printf("License check for %s: tier=%s, active=%v", req.LicenseKey, license.Tier, license.Active)
	}
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
		_, _ = db.Exec(fmt.Sprintf(`DELETE FROM verification_codes WHERE email = %s`, sqlPlaceholder(1)), req.Email)

		// Insert new code
		_, err = db.Exec(fmt.Sprintf(`
			INSERT INTO verification_codes (email, code, created_at, expires_at) 
			VALUES (%s, %s, CURRENT_TIMESTAMP, %s)
		`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3)), req.Email, code, expiresAt.Format(time.RFC3339))
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

		log.Printf("Sent verification code to %s", redactEmail(req.Email))

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
		var expiresAtStr string
		err := db.QueryRow(fmt.Sprintf(`
			SELECT code, expires_at FROM verification_codes 
			WHERE email = %s
		`, sqlPlaceholder(1)), req.Email).Scan(&storedCode, &expiresAtStr)

		if err == sql.ErrNoRows {
			sendError(w, "No verification code found for this email", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("Database error: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			log.Printf("Failed to parse expiration time: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if time.Now().After(expiresAt) {
			sendError(w, "Verification code expired", http.StatusBadRequest)
			return
		}

		log.Printf("Verification attempt: email=%s, match=%v",
			redactEmail(req.Email), storedCode == req.Code)

		if storedCode != req.Code {
			sendError(w, "Invalid verification code", http.StatusUnauthorized)
			return
		}

		// Check if user already has a license
		var existingLicense string
		err = db.QueryRow(fmt.Sprintf(`
			SELECT license_id FROM licenses WHERE customer_email = %s
		`, sqlPlaceholder(1)), req.Email).Scan(&existingLicense)

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

		_, err = db.Exec(fmt.Sprintf(`
			INSERT INTO licenses (
license_id, customer_name, customer_email, tier, 
expires_at, daily_limit, monthly_limit, max_activations, active
) VALUES (%s, %s, %s, 'free', %s, 10, 10, 3, 1)
		`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3), sqlPlaceholder(4)), licenseKey, req.Email, req.Email, expiresAtLicense)

		if err != nil {
			log.Printf("Failed to create license: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Delete verification code
		db.Exec(fmt.Sprintf("DELETE FROM verification_codes WHERE email = %s", sqlPlaceholder(1)), req.Email)

		// Send license email
		if err := sendLicenseEmail(resendAPIKey, fromEmail, req.Email, licenseKey, "free", 10); err != nil {
			log.Printf("Failed to send license email: %v", err)
			// Don't fail - license is already created
		}

		log.Printf("Created FREE license for %s: %s", redactEmail(req.Email), redactPII(licenseKey))

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

// generateProxyKey creates a unique API key for proxy mode
func generateProxyKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "px_" + base64.URLEncoding.EncodeToString(b)[:43], nil
}

// storeProxyKey saves the proxy key mapping
func storeProxyKey(proxyKey, licenseID, hardwareID string) error {
	// Use transaction to ensure atomicity
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing proxy key for this license+hardware (if any)
	_, err = tx.Exec(fmt.Sprintf(`DELETE FROM proxy_keys WHERE license_id = %s AND hardware_id = %s`, sqlPlaceholder(1), sqlPlaceholder(2)), licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to delete old proxy key: %w", err)
	}

	// Insert new proxy key
	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO proxy_keys (proxy_key, license_id, hardware_id)
		VALUES (%s, %s, %s)
	`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3)), proxyKey, licenseID, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to insert proxy key: %w", err)
	}

	return tx.Commit()
}

// validateProxyKey checks if proxy key is valid and returns license info
func validateProxyKey(proxyKey string) (licenseID, hardwareID string, err error) {
	err = db.QueryRow(fmt.Sprintf(`
		SELECT license_id, hardware_id 
		FROM proxy_keys 
		WHERE proxy_key = %s
	`, sqlPlaceholder(1)), proxyKey).Scan(&licenseID, &hardwareID)
	return
}

func handleActivation(protectedAPIKey string, proxyMode bool) http.HandlerFunc {
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

		// Normalize and validate inputs early to avoid panics and wasted work
		req.HardwareID = strings.TrimSpace(req.HardwareID)
		if len(req.HardwareID) < 8 {
			sendError(w, "hardware_id must be at least 8 characters", http.StatusBadRequest)
			return
		}

		req.LicenseKey = strings.TrimSpace(req.LicenseKey)
		if req.LicenseKey == "" {
			sendError(w, "License key is required", http.StatusBadRequest)
			return
		}

		hwPrefix := req.HardwareID
		if len(req.HardwareID) > 8 {
			hwPrefix = req.HardwareID[:8] + "..."
		}
		log.Printf("Activation request: license=%s, hardware=%s", redactPII(req.LicenseKey), hwPrefix)

		// Validate license key exists
		license, err := getLicense(req.LicenseKey)
		if err != nil {
			log.Printf("License not found: %v", err)
			sendError(w, "Invalid license key", http.StatusUnauthorized)
			return
		}

		// For FREE tier: Check if this hardware already has an active free license
		if license.Tier == "free" && isFreeHardwareAlreadyActive(req.HardwareID, req.LicenseKey) {
			hwPrefix := req.HardwareID
			if len(req.HardwareID) > 8 {
				hwPrefix = req.HardwareID[:8] + "..."
			}
			log.Printf("Hardware %s already has an active free license, blocking new free license %s", hwPrefix, redactPII(req.LicenseKey))
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
			log.Printf("New activation recorded for license %s", redactPII(req.LicenseKey))
		} else {
			log.Printf("Re-activation on existing hardware for license %s", redactPII(req.LicenseKey))
		}

		// Record check-in
		recordCheckIn(req.LicenseKey)

		// Generate response based on proxy mode
		var resp ActivationResponse
		if proxyMode {
			// Generate and store proxy key
			proxyKey, err := generateProxyKey()
			if err != nil {
				log.Printf("Error generating proxy key: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if err := storeProxyKey(proxyKey, req.LicenseKey, req.HardwareID); err != nil {
				log.Printf("Error storing proxy key: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Encrypt the proxy key for the client
			encryptedData, iv, err := encryptAPIKeyBundle(proxyKey, license, req.LicenseKey, req.HardwareID)
			if err != nil {
				log.Printf("Encryption error: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			resp = ActivationResponse{
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
			log.Printf("✅ Activation successful for %s (proxy mode - generated key: %s...)", redactPII(req.LicenseKey), proxyKey[:10])
		} else {
			// Normal mode: encrypt the protected API key
			encryptedData, iv, err := encryptAPIKeyBundle(protectedAPIKey, license, req.LicenseKey, req.HardwareID)
			if err != nil {
				log.Printf("Encryption error: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			resp = ActivationResponse{
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
			log.Printf("✅ Activation successful for %s", redactPII(req.LicenseKey))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
		_, err = db.Exec(fmt.Sprintf(`
INSERT INTO daily_usage (license_id, date, scans, hardware_id) 
VALUES (%s, %s, %s, %s)
ON CONFLICT(license_id, date) DO UPDATE SET 
scans = scans + excluded.scans
`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3), sqlPlaceholder(4)), req.LicenseKey, req.Date, req.Scans, req.HardwareID)

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

	err := db.QueryRow(fmt.Sprintf(`
SELECT customer_name, customer_email, tier, expires_at, 
       daily_limit, monthly_limit, max_activations, active
FROM licenses WHERE license_id = %s
`, sqlPlaceholder(1)), licenseID).Scan(
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
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM activations WHERE license_id = %s", sqlPlaceholder(1)), licenseID).Scan(&count)
	return count, err
}

func isHardwareActivated(licenseID, hardwareID string) (bool, error) {
	var count int
	err := db.QueryRow(fmt.Sprintf(`
SELECT COUNT(*) FROM activations 
WHERE license_id = %s AND hardware_id = %s
`, sqlPlaceholder(1), sqlPlaceholder(2)), licenseID, hardwareID).Scan(&count)
	return count > 0, err
}

func recordActivation(licenseID, hardwareID string) error {
	_, err := db.Exec(fmt.Sprintf(`
INSERT INTO activations (license_id, hardware_id) 
VALUES (%s, %s)
`, sqlPlaceholder(1), sqlPlaceholder(2)), licenseID, hardwareID)
	return err
}

func isFreeHardwareAlreadyActive(hardwareID, requestedLicenseID string) bool {
	var count int
	// Use boolean true for PostgreSQL compatibility, works with SQLite too
	err := db.QueryRow(fmt.Sprintf(`
SELECT COUNT(DISTINCT a.license_id) 
FROM activations a
JOIN licenses l ON a.license_id = l.license_id
WHERE a.hardware_id = %s 
  AND l.tier = 'free' 
  AND l.active = true 
  AND l.expires_at > CURRENT_TIMESTAMP
  AND a.license_id != %s
`, sqlPlaceholder(1), sqlPlaceholder(2)), hardwareID, requestedLicenseID).Scan(&count)

	if err != nil {
		log.Printf("Error checking free hardware: %v", err)
		return false
	}

	return count > 0
}

func recordCheckIn(licenseID string) {
	db.Exec(fmt.Sprintf(`
INSERT INTO check_ins (license_id, last_check_in) 
VALUES (%s, CURRENT_TIMESTAMP)
ON CONFLICT(license_id) DO UPDATE SET 
last_check_in = CURRENT_TIMESTAMP
`, sqlPlaceholder(1)), licenseID)
}

func getUsage(licenseID, date string) (int, int) {
	var dailyUsage int
	db.QueryRow(fmt.Sprintf(`
SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
WHERE license_id = %s AND date = %s
`, sqlPlaceholder(1), sqlPlaceholder(2)), licenseID, date).Scan(&dailyUsage)

	// Monthly usage (current month)
	var monthlyUsage int
	yearMonth := date[:7] // YYYY-MM
	db.QueryRow(fmt.Sprintf(`
SELECT COALESCE(SUM(scans), 0) FROM daily_usage 
WHERE license_id = %s AND date LIKE %s
`, sqlPlaceholder(1), sqlPlaceholder(2)), licenseID, yearMonth+"%").Scan(&monthlyUsage)

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error: %s", body)
	}

	return nil
}

// ProxyRequest handles proxying to external APIs
type ProxyRequest struct {
	ProxyKey string          `json:"proxy_key"` // Generated proxy key from activation
	Provider string          `json:"provider"`  // "openai" or "anthropic"
	Body     json.RawMessage `json:"body"`      // Original API request body
}

// handleProxy forwards requests to external APIs while validating license and rate limits
func handleProxy(openaiKey, anthropicKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate proxy key format
		if !strings.HasPrefix(req.ProxyKey, "px_") {
			sendError(w, "Invalid proxy key format", http.StatusBadRequest)
			return
		}

		// Validate proxy key and get license info
		licenseKey, hardwareID, err := validateProxyKey(req.ProxyKey)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("Proxy key not found: %s...", req.ProxyKey[:10])
				sendError(w, "Unauthorized", http.StatusUnauthorized)
			} else {
				log.Printf("Database error validating proxy key: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Check if license exists and is active
		var licenseID, tier, expiresAtStr string
		var dailyLimit int64

		if isPostgresDB {
			// PostgreSQL: use EXTRACT(EPOCH FROM expires_at)
			var expiresAtUnix int64
			err := db.QueryRow(fmt.Sprintf(`
				SELECT license_id, tier, daily_limit, EXTRACT(EPOCH FROM expires_at)::bigint
				FROM licenses 
				WHERE license_id = %s AND active = true
			`, sqlPlaceholder(1)), licenseKey).Scan(&licenseID, &tier, &dailyLimit, &expiresAtUnix)

			if err == sql.ErrNoRows {
				sendError(w, "License not found or inactive", http.StatusUnauthorized)
				return
			} else if err != nil {
				log.Printf("Database error: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			expiresAtStr = time.Unix(expiresAtUnix, 0).Format(time.RFC3339)
		} else {
			// SQLite: expires_at is stored as TEXT in RFC3339 format
			err := db.QueryRow(fmt.Sprintf(`
				SELECT license_id, tier, daily_limit, expires_at
				FROM licenses 
				WHERE license_id = %s AND active = true
			`, sqlPlaceholder(1)), licenseKey).Scan(&licenseID, &tier, &dailyLimit, &expiresAtStr)

			if err == sql.ErrNoRows {
				sendError(w, "License not found or inactive", http.StatusUnauthorized)
				return
			} else if err != nil {
				log.Printf("Database error: %v", err)
				sendError(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		// Parse expiration time
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			log.Printf("Failed to parse expiration time: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if time.Now().After(expiresAt) {
			sendError(w, "License has expired", http.StatusUnauthorized)
			return
		}

		// Verify hardware ID is activated
		var count int
		err = db.QueryRow(fmt.Sprintf(`
			SELECT COUNT(*) FROM activations 
			WHERE license_id = %s AND hardware_id = %s
		`, sqlPlaceholder(1), sqlPlaceholder(2)), licenseID, hardwareID).Scan(&count)

		if err != nil {
			log.Printf("Database error: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if count == 0 {
			sendError(w, "Hardware ID not activated for this license", http.StatusUnauthorized)
			return
		}

		// Check rate limits
		today := time.Now().Format("2006-01-02")
		var currentUsage int
		err = db.QueryRow(fmt.Sprintf(`
			SELECT scans FROM daily_usage 
			WHERE license_id = %s AND date = %s AND hardware_id = %s
		`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3)), licenseID, today, hardwareID).Scan(&currentUsage)

		if err != nil && err != sql.ErrNoRows {
			log.Printf("Database error checking usage: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Check if limit exceeded
		if currentUsage >= int(dailyLimit) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("Daily limit of %d requests exceeded. Current usage: %d", dailyLimit, currentUsage),
					"type":    "rate_limit_exceeded",
					"code":    "rate_limit_exceeded",
				},
			})
			return
		}

		// Determine API endpoint and key
		var apiURL, apiKey string
		var headers map[string]string

		switch req.Provider {
		case "openai":
			if openaiKey == "" {
				sendError(w, "OpenAI API key not configured", http.StatusServiceUnavailable)
				return
			}
			// Extract path from request
			path := strings.TrimPrefix(r.URL.Path, "/proxy/openai")
			if path == "" || path == "/" {
				path = "/v1/chat/completions" // Default endpoint
			}
			apiURL = "https://api.openai.com" + path
			apiKey = openaiKey
			headers = map[string]string{
				"Authorization": "Bearer " + apiKey,
				"Content-Type":  "application/json",
			}

		case "anthropic":
			if anthropicKey == "" {
				sendError(w, "Anthropic API key not configured", http.StatusServiceUnavailable)
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/proxy/anthropic")
			if path == "" || path == "/" {
				path = "/v1/messages" // Default endpoint
			}
			apiURL = "https://api.anthropic.com" + path
			apiKey = anthropicKey
			headers = map[string]string{
				"x-api-key":         apiKey,
				"anthropic-version": "2023-06-01",
				"Content-Type":      "application/json",
			}

		default:
			sendError(w, "Unsupported provider. Supported: openai, anthropic", http.StatusBadRequest)
			return
		}

		// Validate request body size (max 1MB)
		if len(req.Body) > 1024*1024 {
			sendError(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		// Forward request to actual API
		proxyReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(req.Body)))
		if err != nil {
			log.Printf("Failed to create proxy request: %v", err)
			sendError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Set headers
		for key, value := range headers {
			proxyReq.Header.Set(key, value)
		}

		// Execute request with timeout
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("Proxy request timeout: %v", err)
				sendError(w, "Request timeout", http.StatusGatewayTimeout)
			} else {
				log.Printf("Failed to execute proxy request: %v", err)
				sendError(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			}
			return
		}
		defer resp.Body.Close()

		// Increment usage counter (only on successful requests)
		if resp.StatusCode == http.StatusOK {
			_, err = db.Exec(fmt.Sprintf(`
				INSERT INTO daily_usage (license_id, date, scans, hardware_id)
				VALUES (%s, %s, 1, %s)
				ON CONFLICT (license_id, date)
				DO UPDATE SET scans = daily_usage.scans + 1
			`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3)), licenseID, today, hardwareID)

			if err != nil {
				log.Printf("Failed to update usage: %v", err)
				// Don't fail the request, just log the error
			}
		}

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Add rate limit info headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", dailyLimit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int(dailyLimit)-currentUsage-1))
		w.Header().Set("X-RateLimit-Reset", time.Now().Add(24*time.Hour).Format(time.RFC3339))

		// Set status code and stream response body
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)

		log.Printf("Proxied %s request for license %s (usage: %d/%d)", req.Provider, redactPII(licenseID), currentUsage+1, dailyLimit)
	}
}

func main() {
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("Licensify - API Key & License Management Server")
		fmt.Printf("Version:    %s\n", Version)
		if GitCommit != "unknown" {
			fmt.Printf("Commit:     %s\n", GitCommit)
		}
		if BuildTime != "unknown" {
			fmt.Printf("Built:      %s\n", BuildTime)
		}
		fmt.Println()
		fmt.Println("Repository: https://github.com/melihbirim/licensify")
		fmt.Println("License:    GNU AGPL-3.0")
		fmt.Println("Copyright:  © 2025 Melih Birim")
		os.Exit(0)
	}

	// Load .env file (ignore error if doesn't exist)
	_ = godotenv.Load()

	// Load configuration
	config := loadConfig()

	// Validate configuration before proceeding
	if err := validateConfig(config); err != nil {
		log.Fatalf("❌ Configuration error:\n%v\n\nPlease check your environment variables and try again.", err)
	}

	// Load tier configuration
	if err := tiers.LoadWithFallback(config.TiersConfigPath); err != nil {
		log.Fatalf("Failed to load tier configuration: %v", err)
	}
	log.Printf("📋 Loaded tiers: %v", tiers.List())

	// Initialize database
	if err := initDB(config.DatabasePath, config.DatabaseURL); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Load private key (already validated in validateConfig)
	privKeyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKeyB64)
	if err != nil {
		log.Fatalf("Failed to decode private key: %v", err)
	}
	privateKey = ed25519.PrivateKey(privKeyBytes)

	// Start background cleanup for rate limiters
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cleanupIPLimiters(ctx)

	// Setup HTTP routes with rate limiting
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/tiers", handleTiers)
	http.HandleFunc("/init", rateLimitMiddleware(handleInit(config.ResendAPIKey, config.FromEmail)))
	http.HandleFunc("/verify", rateLimitMiddleware(handleVerify(config.ResendAPIKey, config.FromEmail)))
	http.HandleFunc("/activate", rateLimitMiddleware(handleActivation(config.ProtectedAPIKey, config.ProxyMode)))
	http.HandleFunc("/check", rateLimitMiddleware(handleCheck()))
	http.HandleFunc("/usage", rateLimitMiddleware(handleUsageReport()))

	// Setup proxy routes if proxy mode is enabled
	if config.ProxyMode {
		http.HandleFunc("/proxy/", rateLimitMiddleware(handleProxy(config.OpenAIKey, config.AnthropicKey)))
		log.Printf("🔀 Proxy mode: ENABLED")
		if config.OpenAIKey != "" {
			log.Printf("   ✓ OpenAI proxy available at /proxy/openai/*")
		}
		if config.AnthropicKey != "" {
			log.Printf("   ✓ Anthropic proxy available at /proxy/anthropic/*")
		}
	}

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

	// Create HTTP server instance for graceful shutdown
	server := &http.Server{
		Addr:         addr,
		Handler:      nil, // Using DefaultServeMux
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("✅ Server ready to accept connections")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
		serverErr <- nil
	}()

	// Setup signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for either error or shutdown signal
	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-quit:
		log.Printf("🛑 Received shutdown signal: %v", sig)
	}

	// Graceful shutdown
	log.Printf("🔄 Shutting down server gracefully (timeout: %v)...", config.ShutdownTimeout)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
		return
	}

	log.Println("✅ Server stopped gracefully")
}
