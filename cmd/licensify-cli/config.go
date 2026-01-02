package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Server      string    `json:"server"`
	LicenseKey  string    `json:"license_key,omitempty"`
	HardwareID  string    `json:"hardware_id,omitempty"`
	Tier        string    `json:"tier,omitempty"`
	ActivatedAt time.Time `json:"activated_at,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	LastCheck   time.Time `json:"last_check,omitempty"`
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".licensify")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

func loadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &Config{
				Server: getEnv("LICENSIFY_SERVER", "http://localhost:8080"),
			}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	if server := os.Getenv("LICENSIFY_SERVER"); server != "" {
		config.Server = server
	}
	if key := os.Getenv("LICENSIFY_KEY"); key != "" {
		config.LicenseKey = key
	}

	return &config, nil
}

func saveConfig(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func redactKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func printSuccess(message string) {
	fmt.Printf("✓ %s\n", message)
}

func printError(message string) {
	fmt.Fprintf(os.Stderr, "✗ %s\n", message)
}

func printInfo(message string) {
	fmt.Printf("ℹ %s\n", message)
}
