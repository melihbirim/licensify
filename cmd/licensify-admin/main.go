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
	tier := fs.String("tier", "pro", "License tier: free, pro, enterprise")
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

	// Validate tier
	validTiers := map[string]bool{"free": true, "pro": true, "enterprise": true}
	if !validTiers[*tier] {
		fmt.Printf("Error: Invalid tier '%s'. Must be: free, pro, enterprise\n", *tier)
		os.Exit(1)
	}

	// Connect to database
	if err := initDB(); err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer db.Close()

	// Set defaults based on tier if not specified
	if *dailyLimit == 0 {
		switch *tier {
		case "free":
			*dailyLimit = 10
		case "pro":
			*dailyLimit = 1000
		case "enterprise":
			*dailyLimit = -1
		}
	}

	if *monthlyLimit == 0 {
		switch *tier {
		case "free":
			*monthlyLimit = 100
		case "pro":
			*monthlyLimit = 30000
		case "enterprise":
			*monthlyLimit = -1
		}
	}

	if *maxActivations == 0 {
		switch *tier {
		case "free":
			*maxActivations = 1
		case "pro":
			*maxActivations = 3
		case "enterprise":
			*maxActivations = -1
		}
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
	newTier := fs.String("tier", "", "New tier: free, pro, enterprise (required)")
	months := fs.Int("months", 0, "Duration for new license in months (0 to keep same expiry)")
	sendEmail := fs.Bool("send-email", true, "Send email to customer with new license key")

	fs.Parse(os.Args[2:])

	if *oldLicense == "" || *newTier == "" {
		fmt.Println("Error: -license and -tier are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Validate tier
	validTiers := map[string]bool{"free": true, "pro": true, "enterprise": true}
	if !validTiers[*newTier] {
		fmt.Printf("Error: Invalid tier '%s'. Must be: free, pro, enterprise\n", *newTier)
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

	// Calculate new limits based on tier
	var dailyLimit, monthlyLimit, maxActivations int
	switch *newTier {
	case "free":
		dailyLimit, monthlyLimit, maxActivations = 10, 100, 1
	case "pro":
		dailyLimit, monthlyLimit, maxActivations = 1000, 30000, 3
	case "enterprise":
		dailyLimit, monthlyLimit, maxActivations = -1, -1, -1
	}

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
			// Extend by N months
			updates = append(updates, fmt.Sprintf("expires_at = expires_at + INTERVAL '%d months'", *months))
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
		argNum++
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

	return db.Ping()
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
		Subject: fmt.Sprintf("Your License Has Been %s to %s!", strings.Title(tierAction), strings.ToUpper(newTier)),
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
