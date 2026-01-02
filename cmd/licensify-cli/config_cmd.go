package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and manage licensify configuration.`,
	Example: `  licensify config show
  licensify config set server http://localhost:8080
  licensify config reset`,
}

var configShowCmd = &cobra.Command{
	Use:     "show",
	Short:   "Show current configuration",
	Long:    `Display the current configuration including server URL and license key (redacted).`,
	Example: `  licensify config show`,
	RunE:    runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set configuration value",
	Long:  `Set a configuration value. Supported keys: server, key`,
	Example: `  licensify config set server http://localhost:8080
  licensify config set key LIC-xxx-yyy-zzz`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configResetCmd = &cobra.Command{
	Use:     "reset",
	Short:   "Reset configuration",
	Long:    `Delete the configuration file and reset all settings.`,
	Example: `  licensify config reset`,
	RunE:    runConfigReset,
}

var configPathCmd = &cobra.Command{
	Use:     "path",
	Short:   "Show config file path",
	Long:    `Display the path to the configuration file.`,
	Example: `  licensify config path`,
	RunE:    runConfigPath,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("🔧 Configuration")
	fmt.Println("─────────────────")
	fmt.Printf("Server:       %s\n", config.Server)

	if config.LicenseKey != "" {
		fmt.Printf("License Key:  %s\n", redactKey(config.LicenseKey))
	} else {
		fmt.Println("License Key:  (not set)")
	}

	if config.HardwareID != "" {
		fmt.Printf("Hardware ID:  %s\n", redactKey(config.HardwareID))
	} else {
		fmt.Println("Hardware ID:  (not set)")
	}

	if config.Tier != "" {
		fmt.Printf("Tier:         %s\n", config.Tier)
	}

	configPath, _ := getConfigPath()
	fmt.Printf("\nConfig file:  %s\n", configPath)

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch key {
	case "server", "url":
		config.Server = value
		printSuccess(fmt.Sprintf("Server URL set to: %s", value))
	case "key", "license-key", "license_key":
		config.LicenseKey = value
		printSuccess(fmt.Sprintf("License key set to: %s", redactKey(value)))
	case "hardware-id", "hardware_id":
		config.HardwareID = value
		printSuccess(fmt.Sprintf("Hardware ID set to: %s", redactKey(value)))
	case "tier":
		config.Tier = value
		printSuccess(fmt.Sprintf("Tier set to: %s", value))
	default:
		return fmt.Errorf("unknown config key: %s (supported: server, key, hardware-id, tier)", key)
	}

	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		printInfo("No configuration file found, nothing to reset")
		return nil
	}

	// Delete config file
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}

	printSuccess("Configuration reset successfully")
	fmt.Printf("Deleted: %s\n", configPath)

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	fmt.Println(configPath)

	// Show if file exists
	if _, err := os.Stat(configPath); err == nil {
		printInfo("(file exists)")

		// Show file size
		if info, err := os.Stat(configPath); err == nil {
			fmt.Printf("Size: %d bytes\n", info.Size())
		}
	} else {
		printInfo("(file does not exist)")
	}

	return nil
}

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export configuration as JSON",
	Long:  `Export the current configuration as JSON (useful for backup or debugging).`,
	Example: `  licensify config export
  licensify config export > backup.json`,
	RunE: runConfigExport,
}

func init() {
	configCmd.AddCommand(configExportCmd)
}

func runConfigExport(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}
