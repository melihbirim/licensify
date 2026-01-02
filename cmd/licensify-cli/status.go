package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show local license status",
	Long:    `Display the current license configuration stored locally.`,
	Example: `  licensify status`,
	RunE:    runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.LicenseKey == "" {
		printInfo("No license configured.")
		fmt.Println("\nTo get started:")
		fmt.Println("  1. licensify init --email your@email.com")
		fmt.Println("  2. licensify verify --email your@email.com --code <code>")
		fmt.Println("  3. licensify activate")
		return nil
	}

	fmt.Println("📋 License Status")
	fmt.Println("─────────────────")
	fmt.Printf("License Key:  %s\n", redactKey(config.LicenseKey))
	fmt.Printf("Tier:         %s\n", config.Tier)
	fmt.Printf("Server:       %s\n", config.Server)

	if config.HardwareID != "" {
		fmt.Printf("Hardware ID:  %s\n", redactKey(config.HardwareID))
	} else {
		fmt.Println("Hardware ID:  (not activated)")
	}

	if !config.ExpiresAt.IsZero() {
		fmt.Printf("Expires:      %s", config.ExpiresAt.Format("2006-01-02"))

		if time.Now().After(config.ExpiresAt) {
			fmt.Print(" ⚠️  EXPIRED")
		}
		fmt.Println()
	}

	if !config.ActivatedAt.IsZero() {
		fmt.Printf("Activated:    %s\n", config.ActivatedAt.Format("2006-01-02 15:04:05"))
	}

	if !config.LastCheck.IsZero() {
		fmt.Printf("Last Check:   %s\n", config.LastCheck.Format("2006-01-02 15:04:05"))
	}

	fmt.Println("\nTo check license validity with server:")
	fmt.Println("  licensify check")

	return nil
}
