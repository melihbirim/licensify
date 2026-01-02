package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	checkKey string
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check license validity with server",
	Long:  `Verify license status with the server and display usage information.`,
	Example: `  licensify check
  licensify check --key LIC-xxx`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().StringVarP(&checkKey, "key", "k", "", "License key (uses saved key if omitted)")
}

func runCheck(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use provided key or fall back to saved key
	licenseKey := checkKey
	if licenseKey == "" {
		licenseKey = config.LicenseKey
		if licenseKey == "" {
			return fmt.Errorf("no license key provided and no saved key found. Use --key or run 'licensify verify' first")
		}
	}

	client := newHTTPClient(config.Server)

	printInfo("Checking license with server...")

	resp, err := client.checkLicense(licenseKey)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	if !resp.Valid {
		printError("License is NOT valid")
		return fmt.Errorf("license validation failed")
	}

	printSuccess("License is valid!")

	fmt.Println("\n📊 License Details")
	fmt.Println("───────────────────")
	fmt.Printf("Status:        ✅ Active\n")
	fmt.Printf("Tier:          %s\n", resp.Tier)

	if resp.CustomerName != "" {
		fmt.Printf("Customer:      %s\n", resp.CustomerName)
	}

	if !resp.ExpiresAt.IsZero() {
		fmt.Printf("Expires:       %s", resp.ExpiresAt.Format("2006-01-02"))
		daysLeft := int(time.Until(resp.ExpiresAt).Hours() / 24)
		if daysLeft > 0 {
			fmt.Printf(" (%d days left)", daysLeft)
		} else if daysLeft == 0 {
			fmt.Print(" (expires today)")
		}
		fmt.Println()
	}

	fmt.Println("\n📈 Usage")
	fmt.Println("────────")
	fmt.Printf("Daily:         %d / %d", resp.DailyUsage, resp.DailyLimit)
	if resp.DailyLimit > 0 {
		percentage := float64(resp.DailyUsage) / float64(resp.DailyLimit) * 100
		fmt.Printf(" (%.0f%%)", percentage)
		if percentage >= 90 {
			fmt.Print(" ⚠️")
		}
	}
	fmt.Println()

	fmt.Printf("Monthly:       %d / %d", resp.MonthlyUsage, resp.MonthlyLimit)
	if resp.MonthlyLimit > 0 {
		percentage := float64(resp.MonthlyUsage) / float64(resp.MonthlyLimit) * 100
		fmt.Printf(" (%.0f%%)", percentage)
		if percentage >= 90 {
			fmt.Print(" ⚠️")
		}
	}
	fmt.Println()

	// Update last check time
	config.LastCheck = time.Now()
	if err := saveConfig(config); err != nil {
		// Don't fail on config save error
		printError(fmt.Sprintf("Warning: Could not save config: %v", err))
	}

	return nil
}
