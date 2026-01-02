package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/melihbirim/licensify/internal/tiers"
	_ "modernc.org/sqlite"
)

const (
	Version = "1.0.0"
)

var (
	db           *sql.DB
	isPostgresDB bool
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Define commands
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version", "--version", "-v":
		fmt.Printf("licensify-admin version %s\n", Version)
	case "create":
		handleCreate()
	case "upgrade":
		handleUpgrade()
	case "fix":
		handleFix()
	case "list":
		handleList()
	case "get":
		handleGet()
	case "deactivate":
		handleDeactivate()
	case "activate":
		handleActivate()
	case "tiers":
		handleTiers()
	case "migrate":
		handleMigrate()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Licensify Admin - License Management CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  licensify-admin <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create       Create a new license")
	fmt.Println("  upgrade      Upgrade/downgrade a license (creates new key, emails customer)")
	fmt.Println("  fix          Fix an existing license (silent corrections, no email)")
	fmt.Println("  list         List all licenses")
	fmt.Println("  get          Get license details")
	fmt.Println("  activate     Activate a license")
	fmt.Println("  deactivate   Deactivate a license")
	fmt.Println("  tiers        Manage tier configuration")
	fmt.Println("  migrate      Migrate licenses from deprecated tiers")
	fmt.Println("  version      Show version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Create a pro license")
	fmt.Println("  licensify-admin create -email user@example.com -name 'John Doe' -tier pro")
	fmt.Println()
	fmt.Println("  # Upgrade a license (sends email with new key)")
	fmt.Println("  licensify-admin upgrade -license LIC-xxx -tier enterprise")
	fmt.Println()
	fmt.Println("  # Fix license details (no email)")
	fmt.Println("  licensify-admin fix -license LIC-xxx -months 6")
	fmt.Println()
	fmt.Println("  # List all licenses")
	fmt.Println("  licensify-admin list")
	fmt.Println()
	fmt.Println("  # Get specific license details")
	fmt.Println("  licensify-admin get -license LIC-xxx")
}

func handleCreate() {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	email := fs.String("email", "", "Customer email (required)")
	name := fs.String("name", "", "Customer name (required)")
	tier := fs.String("tier", "pro", "License tier (use 'tiers list' to see available tiers)")
	months := fs.Int("months", 12, "License duration in months (0 for lifetime)")
	dailyLimit := fs.Int("daily", 0, "Daily API limit (0 for tier default, -1 unlimited)")
	monthlyLimit := fs.Int("monthly", 0, "Monthly API limit (0 for tier default, -1 unlimited)")
	maxActivations := fs.Int("activations", 0, "Max device activations (0 for tier default, -1 unlimited)")

	fs.Parse(os.Args[2:])

	if *email == "" || *name == "" {
		fmt.Println("Error: -email and -name are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Load tier configuration
	tiersPath := os.Getenv("TIERS_CONFIG_PATH")
	if tiersPath == "" {
		tiersPath = "tiers.toml"
	}
	if err := tiers.LoadWithFallback(tiersPath); err != nil {
		log.Fatalf("Failed to load tier configuration: %v", err)
	}

	// Validate tier exists
	if !tiers.Exists(*tier) {
		fmt.Printf("Error: Invalid tier '%s'. Available tiers: %v\n", *tier, tiers.List())
		fmt.Println("Use 'licensify-admin tiers list' to see tier details")
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Get tier configuration
	tierConfig, _ := tiers.Get(*tier)

	// Set defaults based on tier if not specified
	if *dailyLimit == 0 {
		*dailyLimit = tierConfig.DailyLimit
	}
	if *monthlyLimit == 0 {
		*monthlyLimit = tierConfig.MonthlyLimit
	}
	if *maxActivations == 0 {
		*maxActivations = tierConfig.MaxDevices
	}

	// Generate license key
	licenseKey := generateLicenseKey(*tier)

	// Calculate expiry
	var expiresAt time.Time
	if *months == 0 {
		expiresAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		expiresAt = time.Now().AddDate(0, *months, 0)
	}

	// Insert license
	query := fmt.Sprintf(`
		INSERT INTO licenses (
			license_id, customer_name, customer_email, tier,
			expires_at, daily_limit, monthly_limit, max_activations, active
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, true)
	`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3), sqlPlaceholder(4),
		sqlPlaceholder(5), sqlPlaceholder(6), sqlPlaceholder(7), sqlPlaceholder(8))

	_, err := db.Exec(query, licenseKey, *name, *email, *tier, expiresAt, *dailyLimit, *monthlyLimit, *maxActivations)
	if err != nil {
		log.Fatalf("Failed to create license: %v", err)
	}

	fmt.Println("✅ License created successfully!")
	fmt.Println()
	fmt.Printf("License Key:     %s\n", licenseKey)
	fmt.Printf("Customer:        %s (%s)\n", *name, *email)
	fmt.Printf("Tier:            %s\n", *tier)
	fmt.Printf("Daily Limit:     %s\n", formatLimit(*dailyLimit))
	fmt.Printf("Monthly Limit:   %s\n", formatLimit(*monthlyLimit))
	fmt.Printf("Max Activations: %s\n", formatLimit(*maxActivations))
	fmt.Printf("Expires:         %s\n", expiresAt.Format("2006-01-02"))
}

func handleUpgrade() {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	oldLicense := fs.String("license", "", "Current license key to upgrade (required)")
	newTier := fs.String("tier", "", "New tier (required - use 'tiers list' to see available)")
	months := fs.Int("months", 0, "Duration for new license in months (0 to keep same expiry)")
	sendEmail := fs.Bool("send-email", true, "Send email to customer with new license key")

	fs.Parse(os.Args[2:])

	if *oldLicense == "" || *newTier == "" {
		fmt.Println("Error: -license and -tier are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Load tier configuration
	tiersPath := os.Getenv("TIERS_CONFIG_PATH")
	if tiersPath == "" {
		tiersPath = "tiers.toml"
	}
	if err := tiers.LoadWithFallback(tiersPath); err != nil {
		log.Fatalf("Failed to load tier configuration: %v", err)
	}

	// Validate tier exists
	if !tiers.Exists(*newTier) {
		fmt.Printf("Error: Invalid tier '%s'. Available tiers: %v\n", *newTier, tiers.List())
		fmt.Println("Use 'licensify-admin tiers list' to see tier details")
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Get current license details
	var oldName, oldEmail, oldTier string
	var oldExpiresAt time.Time
	query := fmt.Sprintf(`
		SELECT customer_name, customer_email, tier, expires_at
		FROM licenses WHERE license_id = %s
	`, sqlPlaceholder(1))

	err := db.QueryRow(query, *oldLicense).Scan(&oldName, &oldEmail, &oldTier, &oldExpiresAt)
	if err == sql.ErrNoRows {
		fmt.Printf("❌ License not found: %s\n", *oldLicense)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to get license: %v", err)
	}

	// Get tier configuration
	tierConfig, _ := tiers.Get(*newTier)
	dailyLimit := tierConfig.DailyLimit
	monthlyLimit := tierConfig.MonthlyLimit
	maxActivations := tierConfig.MaxDevices

	// Calculate new expiry
	var newExpiresAt time.Time
	if *months == 0 {
		// Keep same expiry as old license
		newExpiresAt = oldExpiresAt
	} else if *months < 0 {
		// Lifetime
		newExpiresAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		// New duration from now
		newExpiresAt = time.Now().AddDate(0, *months, 0)
	}

	// Generate new license key
	newLicenseKey := generateLicenseKey(*newTier)

	// Insert new license
	insertQuery := fmt.Sprintf(`
		INSERT INTO licenses (
			license_id, customer_name, customer_email, tier,
			expires_at, daily_limit, monthly_limit, max_activations, active
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, true)
	`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3), sqlPlaceholder(4),
		sqlPlaceholder(5), sqlPlaceholder(6), sqlPlaceholder(7), sqlPlaceholder(8))

	_, err = db.Exec(insertQuery, newLicenseKey, oldName, oldEmail, *newTier, newExpiresAt, dailyLimit, monthlyLimit, maxActivations)
	if err != nil {
		log.Fatalf("Failed to create new license: %v", err)
	}

	// Deactivate old license
	_, err = db.Exec(fmt.Sprintf("UPDATE licenses SET active = false WHERE license_id = %s", sqlPlaceholder(1)), *oldLicense)
	if err != nil {
		log.Printf("Warning: Failed to deactivate old license: %v", err)
	}

	fmt.Println("✅ License upgraded successfully!")
	fmt.Println()
	fmt.Printf("Old License:     %s (%s) - DEACTIVATED\n", *oldLicense, oldTier)
	fmt.Printf("New License:     %s (%s)\n", newLicenseKey, *newTier)
	fmt.Printf("Customer:        %s (%s)\n", oldName, oldEmail)
	fmt.Printf("Daily Limit:     %s\n", formatLimit(dailyLimit))
	fmt.Printf("Monthly Limit:   %s\n", formatLimit(monthlyLimit))
	fmt.Printf("Max Activations: %s\n", formatLimit(maxActivations))
	fmt.Printf("Expires:         %s\n", newExpiresAt.Format("2006-01-02"))
	fmt.Println()

	// Send email if enabled
	if *sendEmail {
		resendAPIKey := os.Getenv("RESEND_API_KEY")
		fromEmail := os.Getenv("FROM_EMAIL")

		if resendAPIKey == "" || fromEmail == "" {
			fmt.Println("⚠️  Email not sent: RESEND_API_KEY or FROM_EMAIL not configured")
			fmt.Println("    Add these to your .env file to enable email notifications")
		} else {
			if err := sendUpgradeEmail(resendAPIKey, fromEmail, oldEmail, oldName, oldTier, *newTier, newLicenseKey, dailyLimit); err != nil {
				fmt.Printf("⚠️  Failed to send email: %v\n", err)
			} else {
				fmt.Printf("✅ Upgrade notification sent to %s\n", oldEmail)
			}
		}
	}
}

func handleFix() {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	license := fs.String("license", "", "License key (required)")
	tier := fs.String("tier", "", "New tier: free, pro, enterprise")
	months := fs.Int("months", 0, "Extend license by N months (0 for lifetime)")
	dailyLimit := fs.Int("daily", -999, "Daily API limit (-1 unlimited)")
	monthlyLimit := fs.Int("monthly", -999, "Monthly API limit (-1 unlimited)")
	maxActivations := fs.Int("activations", -999, "Max device activations (-1 unlimited)")

	fs.Parse(os.Args[2:])

	if *license == "" {
		fmt.Println("Error: -license is required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if *tier != "" {
		updates = append(updates, fmt.Sprintf("tier = %s", sqlPlaceholder(argNum)))
		args = append(args, *tier)
		argNum++
	}

	if *dailyLimit != -999 {
		updates = append(updates, fmt.Sprintf("daily_limit = %s", sqlPlaceholder(argNum)))
		args = append(args, *dailyLimit)
		argNum++
	}

	if *monthlyLimit != -999 {
		updates = append(updates, fmt.Sprintf("monthly_limit = %s", sqlPlaceholder(argNum)))
		args = append(args, *monthlyLimit)
		argNum++
	}

	if *maxActivations != -999 {
		updates = append(updates, fmt.Sprintf("max_activations = %s", sqlPlaceholder(argNum)))
		args = append(args, *maxActivations)
		argNum++
	}

	if *months != 0 {
		if *months > 0 {
			// Extend by N months - use cross-DB compatible approach
			if isPostgresDB {
				updates = append(updates, fmt.Sprintf("expires_at = expires_at + INTERVAL '%d months'", *months))
			} else {
				// SQLite: use datetime function
				updates = append(updates, fmt.Sprintf("expires_at = datetime(expires_at, '+%d months')", *months))
			}
		} else {
			// Lifetime
			updates = append(updates, fmt.Sprintf("expires_at = %s", sqlPlaceholder(argNum)))
			args = append(args, time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC))
			argNum++
		}
	}

	if len(updates) == 0 {
		fmt.Println("Error: No updates specified")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Add license key to args
	args = append(args, *license)

	query := fmt.Sprintf("UPDATE licenses SET %s WHERE license_id = %s",
		strings.Join(updates, ", "), sqlPlaceholder(argNum))

	result, err := db.Exec(query, args...)
	if err != nil {
		log.Fatalf("Failed to update license: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Printf("❌ License not found: %s\n", *license)
		os.Exit(1)
	}

	fmt.Printf("✅ License updated: %s\n", *license)

	// Show updated license
	showLicense(*license)
}

func handleList() {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	tier := fs.String("tier", "", "Filter by tier")
	activeOnly := fs.Bool("active", false, "Show only active licenses")

	fs.Parse(os.Args[2:])

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Build query
	query := "SELECT license_id, customer_name, customer_email, tier, expires_at, active FROM licenses WHERE 1=1"
	args := []interface{}{}
	argNum := 1

	if *tier != "" {
		query += fmt.Sprintf(" AND tier = %s", sqlPlaceholder(argNum))
		args = append(args, *tier)
		_ = argNum // argNum is used in sqlPlaceholder above
	}

	if *activeOnly {
		query += " AND active = true"
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Fatalf("Failed to list licenses: %v", err)
	}
	defer rows.Close()

	fmt.Println("Licenses:")
	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("%-30s %-20s %-30s %-12s %-12s %-6s\n", "License Key", "Name", "Email", "Tier", "Expires", "Active")
	fmt.Println(strings.Repeat("-", 100))

	count := 0
	for rows.Next() {
		var licenseID, name, email, tier string
		var expiresAt time.Time
		var active bool

		if err := rows.Scan(&licenseID, &name, &email, &tier, &expiresAt, &active); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		activeStr := "✓"
		if !active {
			activeStr = "✗"
		}

		fmt.Printf("%-30s %-20s %-30s %-12s %-12s %-6s\n",
			licenseID, truncate(name, 20), truncate(email, 30), tier,
			expiresAt.Format("2006-01-02"), activeStr)
		count++
	}

	fmt.Println(strings.Repeat("-", 100))
	fmt.Printf("Total: %d licenses\n", count)
}

func handleGet() {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	license := fs.String("license", "", "License key (required)")

	fs.Parse(os.Args[2:])

	if *license == "" {
		fmt.Println("Error: -license is required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	showLicense(*license)
}

func handleDeactivate() {
	fs := flag.NewFlagSet("deactivate", flag.ExitOnError)
	license := fs.String("license", "", "License key (required)")

	fs.Parse(os.Args[2:])

	if *license == "" {
		fmt.Println("Error: -license is required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	result, err := db.Exec(fmt.Sprintf("UPDATE licenses SET active = false WHERE license_id = %s", sqlPlaceholder(1)), *license)
	if err != nil {
		log.Fatalf("Failed to deactivate license: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Printf("❌ License not found: %s\n", *license)
		os.Exit(1)
	}

	fmt.Printf("✅ License deactivated: %s\n", *license)
}

func handleActivate() {
	fs := flag.NewFlagSet("activate", flag.ExitOnError)
	license := fs.String("license", "", "License key (required)")

	fs.Parse(os.Args[2:])

	if *license == "" {
		fmt.Println("Error: -license is required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	result, err := db.Exec(fmt.Sprintf("UPDATE licenses SET active = true WHERE license_id = %s", sqlPlaceholder(1)), *license)
	if err != nil {
		log.Fatalf("Failed to activate license: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Printf("❌ License not found: %s\n", *license)
		os.Exit(1)
	}

	fmt.Printf("✅ License activated: %s\n", *license)
}

// Helper functions

func initDB() error {
	dbURL := os.Getenv("DATABASE_URL")
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "licensify.db"
	}

	var err error
	if dbURL != "" {
		// PostgreSQL
		db, err = sql.Open("postgres", dbURL)
		isPostgresDB = true
	} else {
		// SQLite
		db, err = sql.Open("sqlite", dbPath)
		isPostgresDB = false
	}

	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Initialize schema if tables don't exist
	if err := initSchema(); err != nil {
		return fmt.Errorf("failed to initialize schema: %v", err)
	}

	return nil
}

func initSchema() error {
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
	return err
}

func sqlPlaceholder(n int) string {
	if isPostgresDB {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func generateLicenseKey(tier string) string {
	timestamp := time.Now().Format("200601")
	tierPrefix := strings.ToUpper(tier[:min(4, len(tier))])
	return fmt.Sprintf("LIC-%s-%s-%06d", timestamp, tierPrefix, time.Now().Unix()%1000000)
}

func showLicense(licenseID string) {
	var name, email, tier string
	var expiresAt, createdAt time.Time
	var dailyLimit, monthlyLimit, maxActivations int
	var active bool

	query := fmt.Sprintf(`
		SELECT customer_name, customer_email, tier, expires_at, 
		       daily_limit, monthly_limit, max_activations, active, created_at
		FROM licenses WHERE license_id = %s
	`, sqlPlaceholder(1))

	err := db.QueryRow(query, licenseID).Scan(&name, &email, &tier, &expiresAt,
		&dailyLimit, &monthlyLimit, &maxActivations, &active, &createdAt)

	if err == sql.ErrNoRows {
		fmt.Printf("❌ License not found: %s\n", licenseID)
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("Failed to get license: %v", err)
	}

	// Get activation count
	var activationCount int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM activations WHERE license_id = %s", sqlPlaceholder(1))
	db.QueryRow(countQuery, licenseID).Scan(&activationCount)

	// Display
	fmt.Println()
	fmt.Println("License Details:")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("License Key:       %s\n", licenseID)
	fmt.Printf("Customer Name:     %s\n", name)
	fmt.Printf("Customer Email:    %s\n", email)
	fmt.Printf("Tier:              %s\n", strings.ToUpper(tier))
	fmt.Printf("Status:            %s\n", formatActive(active))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Daily Limit:       %s\n", formatLimit(dailyLimit))
	fmt.Printf("Monthly Limit:     %s\n", formatLimit(monthlyLimit))
	fmt.Printf("Max Activations:   %s\n", formatLimit(maxActivations))
	fmt.Printf("Current Activations: %d\n", activationCount)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Created:           %s\n", createdAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Expires:           %s\n", expiresAt.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
}

func formatLimit(limit int) string {
	if limit == -1 {
		return "Unlimited"
	}
	return fmt.Sprintf("%d", limit)
}

func formatActive(active bool) string {
	if active {
		return "✅ Active"
	}
	return "❌ Inactive"
}

func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sendUpgradeEmail(resendAPIKey, fromEmail, toEmail, customerName, oldTier, newTier, newLicenseKey string, dailyLimit int) error {
	type EmailRequest struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
	}

	tierAction := "upgraded"
	if newTier == "free" {
		tierAction = "changed"
	}

	limitText := fmt.Sprintf("%d requests/day", dailyLimit)
	if dailyLimit == -1 {
		limitText = "unlimited requests"
	}

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .license-box { background: white; border: 2px solid #667eea; border-radius: 8px; padding: 20px; margin: 20px 0; text-align: center; }
        .license-key { font-size: 24px; font-weight: bold; color: #667eea; font-family: monospace; letter-spacing: 1px; word-break: break-all; }
        .tier-badge { display: inline-block; padding: 8px 16px; border-radius: 20px; font-weight: bold; margin: 10px 0; }
        .tier-free { background: #e3f2fd; color: #1976d2; }
        .tier-pro { background: #f3e5f5; color: #7b1fa2; }
        .tier-enterprise { background: #fff3e0; color: #e65100; }
        .feature-list { list-style: none; padding: 0; }
        .feature-list li { padding: 10px 0; border-bottom: 1px solid #eee; }
        .feature-list li:before { content: "✓ "; color: #4caf50; font-weight: bold; margin-right: 10px; }
        .cta-button { display: inline-block; background: #667eea; color: white; padding: 15px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
        .footer { text-align: center; color: #999; font-size: 12px; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 License %s!</h1>
        </div>
        <div class="content">
            <p>Hi %s,</p>
            
            <p>Great news! Your license has been %s from <strong>%s</strong> to:</p>
            
            <div style="text-align: center;">
                <span class="tier-badge tier-%s">%s Tier</span>
            </div>
            
            <div class="license-box">
                <p style="margin: 0 0 10px 0; color: #666;">Your New License Key:</p>
                <div class="license-key">%s</div>
            </div>
            
            <h3>📊 Your New Limits:</h3>
            <ul class="feature-list">
                <li>%s</li>
                <li>Priority support</li>
                <li>Full API access</li>
            </ul>
            
            <h3>🚀 Next Steps:</h3>
            <ol>
                <li>Save your new license key in a secure location</li>
                <li>Update your application with the new license key</li>
                <li>Activate your license to start using the new features</li>
            </ol>
            
            <p><strong>Note:</strong> Your previous license key has been deactivated and will no longer work.</p>
            
            <p>If you have any questions or need assistance, please don't hesitate to reach out to our support team.</p>
            
            <p>Best regards,<br>
            The Licensify Team</p>
        </div>
        
        <div class="footer">
            <p>This is an automated email from Licensify License Management System.</p>
        </div>
    </div>
</body>
</html>
	`, tierAction, customerName, tierAction, oldTier, newTier, strings.ToUpper(newTier), newLicenseKey, limitText)

	emailReq := EmailRequest{
		From:    fromEmail,
		To:      []string{toEmail},
		Subject: fmt.Sprintf("Your License Has Been %s to %s!", strings.ToUpper(tierAction[:1])+tierAction[1:], strings.ToUpper(newTier)),
		HTML:    htmlBody,
	}

	jsonData, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+resendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	return nil
}

func handleTiers() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: licensify-admin tiers <subcommand>")
		fmt.Println()
		fmt.Println("Subcommands:")
		fmt.Println("  list      List all available tiers with details")
		fmt.Println("  get       Get specific tier configuration")
		fmt.Println("  validate  Validate tiers.toml configuration")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  licensify-admin tiers list")
		fmt.Println("  licensify-admin tiers get -name tier-2")
		fmt.Println("  licensify-admin tiers validate")
		fmt.Println()
		fmt.Println("Tier Naming Convention:")
		fmt.Println("  Use numeric IDs: tier-1, tier-2, tier-3, tier-100, etc.")
		fmt.Println("  Allows easy tier management and migration paths")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	// Load tier configuration
	tiersPath := os.Getenv("TIERS_CONFIG_PATH")
	if tiersPath == "" {
		tiersPath = "tiers.toml"
	}

	switch subcommand {
	case "list":
		if err := tiers.LoadWithFallback(tiersPath); err != nil {
			log.Fatalf("Failed to load tier configuration: %v", err)
		}

		allTiers := tiers.GetAll()
		if len(allTiers) == 0 {
			fmt.Println("No tiers configured")
			return
		}

		fmt.Println("Available Tiers:")
		fmt.Println(strings.Repeat("=", 100))
		for name, tier := range allTiers {
			deprecatedMarker := ""
			if tier.Deprecated {
				deprecatedMarker = " [DEPRECATED]"
			}
			fmt.Printf("\n📦 %s (%s)%s\n", strings.ToUpper(name), tier.Name, deprecatedMarker)
			fmt.Println(strings.Repeat("-", 100))
			fmt.Printf("  Daily Limit:       %s\n", formatLimit(tier.DailyLimit))
			fmt.Printf("  Monthly Limit:     %s\n", formatLimit(tier.MonthlyLimit))
			fmt.Printf("  Max Devices:       %s\n", formatLimit(tier.MaxDevices))
			fmt.Printf("  Features:          %s\n", strings.Join(tier.Features, ", "))
			fmt.Printf("  Email Verification: %v\n", tier.EmailVerificationRequired)
			if tier.PriceMonthly > 0 {
				fmt.Printf("  Price (Monthly):   $%.2f\n", tier.PriceMonthly)
			}
			if tier.OneTimePayment > 0 {
				fmt.Printf("  Price (Lifetime):  $%.2f\n", tier.OneTimePayment)
			}
			if tier.CustomPricing {
				fmt.Printf("  Custom Pricing:    Yes\n")
			}
			if tier.Hidden {
				fmt.Printf("  Hidden:            Yes (not visible in public listings)\n")
			}
			if tier.Deprecated {
				fmt.Printf("  ⚠️  DEPRECATED:      Yes")
				if tier.MigrateTo != "" {
					fmt.Printf(" → Migrate to: %s\n", tier.MigrateTo)
				} else {
					fmt.Printf("\n")
				}
			}
			fmt.Printf("  Description:       %s\n", tier.Description)
		}
		fmt.Println(strings.Repeat("=", 100))
		fmt.Printf("\nTotal: %d tiers\n", len(allTiers))

	case "get":
		fs := flag.NewFlagSet("get", flag.ExitOnError)
		tierName := fs.String("name", "", "Tier name (required)")
		fs.Parse(os.Args[3:])

		if *tierName == "" {
			fmt.Println("Error: -name is required")
			fs.PrintDefaults()
			os.Exit(1)
		}

		if err := tiers.LoadWithFallback(tiersPath); err != nil {
			log.Fatalf("Failed to load tier configuration: %v", err)
		}

		tier, err := tiers.Get(*tierName)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			fmt.Printf("Available tiers: %v\n", tiers.List())
			os.Exit(1)
		}

		fmt.Printf("\n📦 %s (%s)\n", strings.ToUpper(*tierName), tier.Name)
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("Daily Limit:           %s\n", formatLimit(tier.DailyLimit))
		fmt.Printf("Monthly Limit:         %s\n", formatLimit(tier.MonthlyLimit))
		fmt.Printf("Max Devices:           %s\n", formatLimit(tier.MaxDevices))
		fmt.Printf("Features:              %s\n", strings.Join(tier.Features, ", "))
		fmt.Printf("Email Verification:    %v\n", tier.EmailVerificationRequired)
		if tier.PriceMonthly > 0 {
			fmt.Printf("Price (Monthly):       $%.2f\n", tier.PriceMonthly)
		}
		if tier.OneTimePayment > 0 {
			fmt.Printf("Price (Lifetime):      $%.2f\n", tier.OneTimePayment)
		}
		if tier.CustomPricing {
			fmt.Printf("Custom Pricing:        Yes\n")
		}
		if tier.Hidden {
			fmt.Printf("Hidden:                Yes\n")
		}
		if tier.Deprecated {
			fmt.Printf("⚠️  DEPRECATED:         Yes")
			if tier.MigrateTo != "" {
				fmt.Printf(" → Migrate to: %s\n", tier.MigrateTo)
			} else {
				fmt.Printf("\n")
			}
		}
		fmt.Printf("Description:           %s\n", tier.Description)
		fmt.Println(strings.Repeat("=", 60))

	case "validate":
		fmt.Printf("Validating tier configuration: %s\n", tiersPath)

		if err := tiers.Load(tiersPath); err != nil {
			fmt.Printf("❌ Validation failed: %v\n", err)
			os.Exit(1)
		}

		allTiers := tiers.GetAll()
		fmt.Printf("✅ Configuration is valid!\n")
		fmt.Printf("   Found %d tier(s): %v\n", len(allTiers), tiers.List())

		// Check for common issues and deprecations
		warnings := []string{}
		deprecatedCount := 0
		for name, tier := range allTiers {
			if tier.DailyLimit > tier.MonthlyLimit && tier.MonthlyLimit != -1 {
				warnings = append(warnings, fmt.Sprintf("tier '%s': daily_limit (%d) > monthly_limit (%d)", name, tier.DailyLimit, tier.MonthlyLimit))
			}
			if len(tier.Features) == 0 {
				warnings = append(warnings, fmt.Sprintf("tier '%s': no features defined", name))
			}
			if tier.Deprecated {
				deprecatedCount++
				if tier.MigrateTo == "" {
					warnings = append(warnings, fmt.Sprintf("tier '%s': deprecated but no migrate_to target specified", name))
				}
			}
		}

		if deprecatedCount > 0 {
			fmt.Printf("   ⚠️  %d deprecated tier(s) found\n", deprecatedCount)
		}

		if len(warnings) > 0 {
			fmt.Println("\n⚠️  Warnings:")
			for _, warning := range warnings {
				fmt.Printf("   - %s\n", warning)
			}
		}

	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleMigrate() {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	fromTier := fs.String("from", "", "Source tier to migrate from (required)")
	toTier := fs.String("to", "", "Target tier to migrate to (optional - uses tier config if not specified)")
	dryRun := fs.Bool("dry-run", false, "Show what would be migrated without making changes")
	sendEmail := fs.Bool("send-email", true, "Send email notifications to migrated customers")

	fs.Parse(os.Args[2:])

	if *fromTier == "" {
		fmt.Println("Error: -from is required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Load tier configuration
	tiersPath := os.Getenv("TIERS_CONFIG_PATH")
	if tiersPath == "" {
		tiersPath = "tiers.toml"
	}
	if err := tiers.LoadWithFallback(tiersPath); err != nil {
		log.Fatalf("Failed to load tier configuration: %v", err)
	}

	// Validate source tier exists
	if !tiers.Exists(*fromTier) {
		fmt.Printf("❌ Source tier '%s' not found. Available tiers: %v\n", *fromTier, tiers.List())
		os.Exit(1)
	}

	// Determine target tier
	targetTier := *toTier
	if targetTier == "" {
		// Check if source tier has a migration target
		migrationTarget, err := tiers.GetMigrationTarget(*fromTier)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			fmt.Println("Please specify -to flag to set the migration target manually")
			os.Exit(1)
		}
		targetTier = migrationTarget
		fmt.Printf("ℹ️  Using configured migration target: %s → %s\n", *fromTier, targetTier)
	} else {
		// Validate target tier exists
		if !tiers.Exists(targetTier) {
			fmt.Printf("❌ Target tier '%s' not found. Available tiers: %v\n", targetTier, tiers.List())
			os.Exit(1)
		}
	}

	if *fromTier == targetTier {
		fmt.Println("❌ Source and target tiers cannot be the same")
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Get source and target tier configurations (use GetRaw to get actual tier data, not migration target)
	sourceTierConfig, _ := tiers.GetRaw(*fromTier)
	targetTierConfig, _ := tiers.GetRaw(targetTier)

	// Find all licenses on the source tier
	query := fmt.Sprintf("SELECT license_id, customer_name, customer_email, expires_at FROM licenses WHERE tier = %s AND active = true", sqlPlaceholder(1))
	rows, err := db.Query(query, *fromTier)
	if err != nil {
		log.Fatalf("Failed to query licenses: %v", err)
	}
	defer rows.Close()

	type LicenseInfo struct {
		LicenseID string
		Name      string
		Email     string
		ExpiresAt time.Time
	}

	licenses := []LicenseInfo{}
	for rows.Next() {
		var lic LicenseInfo
		if err := rows.Scan(&lic.LicenseID, &lic.Name, &lic.Email, &lic.ExpiresAt); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		licenses = append(licenses, lic)
	}

	if len(licenses) == 0 {
		fmt.Printf("✅ No active licenses found on tier '%s'\n", *fromTier)
		return
	}

	fmt.Printf("\n📋 Migration Plan: %s → %s\n", *fromTier, targetTier)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Source Tier:  %s (%s)\n", *fromTier, sourceTierConfig.Name)
	fmt.Printf("Target Tier:  %s (%s)\n", targetTier, targetTierConfig.Name)
	fmt.Printf("Licenses:     %d active licenses will be migrated\n", len(licenses))
	fmt.Println()
	fmt.Printf("Limit Changes:\n")
	fmt.Printf("  Daily:      %s → %s\n", formatLimit(sourceTierConfig.DailyLimit), formatLimit(targetTierConfig.DailyLimit))
	fmt.Printf("  Monthly:    %s → %s\n", formatLimit(sourceTierConfig.MonthlyLimit), formatLimit(targetTierConfig.MonthlyLimit))
	fmt.Printf("  Max Devices: %s → %s\n", formatLimit(sourceTierConfig.MaxDevices), formatLimit(targetTierConfig.MaxDevices))
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	if *dryRun {
		fmt.Println("🔍 DRY RUN - No changes will be made")
		fmt.Println("\nLicenses that would be migrated:")
		for i, lic := range licenses {
			fmt.Printf("  %d. %s - %s (%s) - Expires: %s\n",
				i+1, lic.LicenseID, lic.Name, lic.Email, lic.ExpiresAt.Format("2006-01-02"))
		}
		fmt.Println("\nRun without -dry-run to perform the migration")
		return
	}

	// Confirm migration
	fmt.Print("\n⚠️  This will update licenses in the database. Continue? (yes/no): ")
	var confirmation string
	fmt.Scanln(&confirmation)
	if strings.ToLower(confirmation) != "yes" {
		fmt.Println("Migration cancelled")
		return
	}

	// Perform migration
	fmt.Println("\n🔄 Migrating licenses...")
	updateQuery := fmt.Sprintf(`
		UPDATE licenses 
		SET tier = %s, 
		    daily_limit = %s, 
		    monthly_limit = %s, 
		    max_activations = %s
		WHERE license_id = %s
	`, sqlPlaceholder(1), sqlPlaceholder(2), sqlPlaceholder(3), sqlPlaceholder(4), sqlPlaceholder(5))

	successCount := 0
	failCount := 0

	for i, lic := range licenses {
		_, err := db.Exec(updateQuery,
			targetTier,
			targetTierConfig.DailyLimit,
			targetTierConfig.MonthlyLimit,
			targetTierConfig.MaxDevices,
			lic.LicenseID)

		if err != nil {
			fmt.Printf("  ❌ %d. %s - Failed: %v\n", i+1, lic.LicenseID, err)
			failCount++
			continue
		}

		fmt.Printf("  ✅ %d. %s - %s (%s)\n", i+1, lic.LicenseID, lic.Name, lic.Email)
		successCount++

		// Send email notification if enabled
		if *sendEmail {
			resendAPIKey := os.Getenv("RESEND_API_KEY")
			fromEmail := os.Getenv("FROM_EMAIL")

			if resendAPIKey != "" && fromEmail != "" {
				if err := sendMigrationEmail(resendAPIKey, fromEmail, lic.Email, lic.Name,
					*fromTier, sourceTierConfig.Name, targetTier, targetTierConfig.Name,
					targetTierConfig.DailyLimit, lic.LicenseID); err != nil {
					fmt.Printf("     ⚠️  Failed to send email: %v\n", err)
				} else {
					fmt.Printf("     📧 Email sent\n")
				}
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("✅ Migration completed: %d succeeded, %d failed\n", successCount, failCount)
	fmt.Println(strings.Repeat("=", 80))
}

func sendMigrationEmail(resendAPIKey, fromEmail, toEmail, customerName, oldTierID, oldTierName, newTierID, newTierName string, newDailyLimit int, licenseKey string) error {
	type EmailRequest struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
	}

	limitText := fmt.Sprintf("%d requests/day", newDailyLimit)
	if newDailyLimit == -1 {
		limitText = "unlimited requests"
	}

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .tier-box { background: white; border: 2px solid #667eea; border-radius: 8px; padding: 20px; margin: 20px 0; }
        .migration-arrow { text-align: center; font-size: 24px; color: #667eea; margin: 10px 0; }
        .footer { text-align: center; color: #999; font-size: 12px; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📦 Your License Tier Has Been Updated</h1>
        </div>
        <div class="content">
            <p>Hi %s,</p>
            
            <p>We're writing to inform you that your license tier has been migrated to a new plan:</p>
            
            <div class="tier-box">
                <h3>Previous Tier</h3>
                <p><strong>%s</strong> (%s)</p>
            </div>
            
            <div class="migration-arrow">↓</div>
            
            <div class="tier-box">
                <h3>New Tier</h3>
                <p><strong>%s</strong> (%s)</p>
                <p><strong>New Limits:</strong> %s</p>
            </div>
            
            <h3>What This Means:</h3>
            <ul>
                <li>Your license key remains the same: <code>%s</code></li>
                <li>No action is required from you</li>
                <li>Your new limits are now active</li>
            </ul>
            
            <p>If you have any questions about this migration, please don't hesitate to reach out to our support team.</p>
            
            <p>Best regards,<br>
            The Licensify Team</p>
        </div>
        
        <div class="footer">
            <p>This is an automated email from Licensify License Management System.</p>
        </div>
    </div>
</body>
</html>
	`, customerName, oldTierName, oldTierID, newTierName, newTierID, limitText, licenseKey)

	emailReq := EmailRequest{
		From:    fromEmail,
		To:      []string{toEmail},
		Subject: fmt.Sprintf("Your License Has Been Migrated to %s", newTierName),
		HTML:    htmlBody,
	}

	jsonData, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+resendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	return nil
}
