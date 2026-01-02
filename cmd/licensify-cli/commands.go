package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	initEmail string
	initTier  string
)

var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"request"},
	Short:   "Request a new license",
	Long: `Request a new license by providing your email and desired tier.
You will receive a verification code via email.`,
	Example: `  licensify init
  licensify init --email user@example.com --tier free
  licensify request --email user@example.com --tier pro`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initEmail, "email", "e", "", "Email address")
	initCmd.Flags().StringVarP(&initTier, "tier", "t", "", "License tier (free, pro, enterprise)")
}

func runInit(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if running interactively (no flags provided)
	interactive := initEmail == "" && initTier == ""

	scanner := bufio.NewScanner(os.Stdin)

	// Ask for server URL if not configured and running interactively
	if interactive && config.Server == "http://localhost:8080" && serverURL == "" {
		fmt.Printf("Server URL [%s]: ", config.Server)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input != "" {
				config.Server = input
			}
		}
	}

	// Ask for email if not provided
	if initEmail == "" {
		fmt.Print("Email address: ")
		if scanner.Scan() {
			initEmail = strings.TrimSpace(scanner.Text())
			if initEmail == "" {
				return fmt.Errorf("email is required")
			}
		}
	}

	// Ask for tier if not provided
	if initTier == "" {
		fmt.Print("License tier [free]: ")
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				initTier = "free"
			} else {
				initTier = input
			}
		} else {
			initTier = "free"
		}
	}

	// Save config with server URL if it was changed
	if interactive {
		if err := saveConfig(config); err != nil {
			printError(fmt.Sprintf("Warning: Could not save config: %v", err))
		}
	}

	client := newHTTPClient(config.Server)

	printInfo(fmt.Sprintf("Requesting license for %s (tier: %s)...", initEmail, initTier))

	resp, err := client.requestLicense(initEmail, initTier)
	if err != nil {
		return fmt.Errorf("failed to request license: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("request failed: %s", resp.Message)
	}

	// Save email and tier to config for next step
	config.Email = initEmail
	config.Tier = initTier
	if err := saveConfig(config); err != nil {
		printError(fmt.Sprintf("Warning: Could not save config: %v", err))
	}

	printSuccess("Verification code sent!")
	fmt.Printf("\n📧 Check your email: %s\n", resp.Email)
	fmt.Println("\nNext step:")
	fmt.Printf("  licensify verify --email %s --code <code> --tier %s\n\n", initEmail, initTier)

	return nil
}

// Verify command
var (
	verifyEmail string
	verifyCode  string
	verifyTier  string
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify email and create license",
	Long:  `Verify your email with the code sent to you and create a license key.`,
	Example: `  licensify verify --code 123456
  licensify verify --email user@example.com --code 123456 --tier free
  licensify verify -e user@example.com -c 123456 -t pro`,
	RunE: runVerify,
}

func init() {
	verifyCmd.Flags().StringVarP(&verifyEmail, "email", "e", "", "Email address")
	verifyCmd.Flags().StringVarP(&verifyCode, "code", "c", "", "Verification code (required)")
	verifyCmd.Flags().StringVarP(&verifyTier, "tier", "t", "", "License tier")
	_ = verifyCmd.MarkFlagRequired("code")
}

func runVerify(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use saved email if not provided
	if verifyEmail == "" {
		if config.Email != "" {
			verifyEmail = config.Email
			printInfo(fmt.Sprintf("Using saved email: %s", verifyEmail))
		} else {
			return fmt.Errorf("email is required (use --email or run 'licensify init' first)")
		}
	}

	// Use saved tier if not provided
	if verifyTier == "" {
		if config.Tier != "" {
			verifyTier = config.Tier
		} else {
			verifyTier = "free"
		}
	}

	client := newHTTPClient(config.Server)

	printInfo("Verifying email...")

	resp, err := client.verifyEmail(verifyEmail, verifyCode, verifyTier)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("verification failed: %s", resp.Message)
	}

	printSuccess("License created successfully!")
	fmt.Printf("\nLicense Key: %s\n", resp.LicenseKey)
	fmt.Printf("Customer: %s\n", verifyEmail)
	fmt.Printf("Tier: %s\n", resp.Tier)
	fmt.Printf("Expires: %s\n", resp.ExpiresAt.Format("2006-01-02"))
	fmt.Printf("Daily Limit: %d\n", resp.DailyLimit)
	fmt.Printf("Monthly Limit: %d\n", resp.MonthlyLimit)

	// Save license key to config
	config.Email = verifyEmail
	config.LicenseKey = resp.LicenseKey
	config.Tier = resp.Tier
	config.ExpiresAt = resp.ExpiresAt
	if err := saveConfig(config); err != nil {
		printError(fmt.Sprintf("Warning: Could not save config: %v", err))
	} else {
		printInfo("License key saved to config")
	}

	fmt.Println("\nYour license key has also been sent to your email.")
	fmt.Println("\nNext step:")
	fmt.Println("  licensify activate")

	return nil
}

// Activate command
var (
	activateKey        string
	activateHardwareID string
)

var activateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activate license on this machine",
	Long:  `Activate your license on the current machine. Hardware ID will be auto-detected if not provided.`,
	Example: `  licensify activate
  licensify activate --key LIC-xxx
  licensify activate --key LIC-xxx --hardware-id hw-123`,
	RunE: runActivate,
}

func init() {
	activateCmd.Flags().StringVarP(&activateKey, "key", "k", "", "License key (uses saved key if omitted)")
	activateCmd.Flags().StringVar(&activateHardwareID, "hardware-id", "", "Hardware ID (auto-detected if omitted)")
}

func runActivate(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use provided key or fall back to saved key
	licenseKey := activateKey
	if licenseKey == "" {
		licenseKey = config.LicenseKey
		if licenseKey == "" {
			return fmt.Errorf("no license key provided and no saved key found. Use --key or run 'licensify verify' first")
		}
	}

	// Get or detect hardware ID
	hardwareID := activateHardwareID
	if hardwareID == "" {
		printInfo("Detecting hardware ID...")
		hwID, err := getHardwareID()
		if err != nil {
			return fmt.Errorf("failed to detect hardware ID: %w\nProvide it manually with --hardware-id", err)
		}
		hardwareID = hwID
		printInfo(fmt.Sprintf("Hardware ID: %s", redactKey(hardwareID)))
	}

	client := newHTTPClient(config.Server)

	printInfo("Activating license...")

	resp, err := client.activateLicense(licenseKey, hardwareID)
	if err != nil {
		return fmt.Errorf("activation failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("activation failed: %s", resp.Message)
	}

	printSuccess("License activated successfully!")

	// Update config
	config.LicenseKey = licenseKey
	config.HardwareID = hardwareID
	config.ActivatedAt = time.Now()
	if err := saveConfig(config); err != nil {
		printError(fmt.Sprintf("Warning: Could not save config: %v", err))
	}

	fmt.Printf("\nLicense Key: %s\n", redactKey(licenseKey))
	fmt.Printf("Hardware ID: %s\n", redactKey(hardwareID))
	fmt.Println("\nYour license is now active!")

	return nil
}
