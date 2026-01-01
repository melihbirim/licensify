package tiers

import (
	"fmt"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

// TierConfig represents the entire tier configuration
type TierConfig struct {
	Tiers map[string]*TierDetails `toml:"tiers"`
}

// TierDetails represents the configuration for a single tier
type TierDetails struct {
	Name                      string   `toml:"name"`
	DailyLimit                int      `toml:"daily_limit"`
	MonthlyLimit              int      `toml:"monthly_limit"`
	MaxDevices                int      `toml:"max_devices"`
	Features                  []string `toml:"features"`
	EmailVerificationRequired bool     `toml:"email_verification_required"`
	PriceMonthly              float64  `toml:"price_monthly,omitempty"`
	OneTimePayment            float64  `toml:"one_time_payment,omitempty"`
	CustomPricing             bool     `toml:"custom_pricing,omitempty"`
	Hidden                    bool     `toml:"hidden,omitempty"`
	Deprecated                bool     `toml:"deprecated,omitempty"`
	MigrateTo                 string   `toml:"migrate_to,omitempty"`
	Description               string   `toml:"description"`
}

var (
	// Global tier configuration
	config *TierConfig
)

// Load loads the tier configuration from a TOML file
func Load(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("tier configuration file not found: %s", path)
	}

	var cfg TierConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return fmt.Errorf("failed to parse tier configuration: %w", err)
	}

	// Validate configuration
	if len(cfg.Tiers) == 0 {
		return fmt.Errorf("no tiers defined in configuration")
	}

	// Validate each tier
	for name, tier := range cfg.Tiers {
		if tier.Name == "" {
			return fmt.Errorf("tier '%s' is missing a display name", name)
		}
		if tier.DailyLimit < -1 {
			return fmt.Errorf("tier '%s' has invalid daily_limit (must be >= -1)", name)
		}
		if tier.MonthlyLimit < -1 {
			return fmt.Errorf("tier '%s' has invalid monthly_limit (must be >= -1)", name)
		}
		if tier.MaxDevices < -1 {
			return fmt.Errorf("tier '%s' has invalid max_devices (must be >= -1)", name)
		}
		// Validate migration target if deprecated
		if tier.Deprecated && tier.MigrateTo != "" {
			if tier.MigrateTo == name {
				return fmt.Errorf("tier '%s' cannot migrate to itself", name)
			}
			// Check if migration target will exist (after loading all tiers)
		}
		// Also validate standalone migrate_to without deprecated flag
		if tier.MigrateTo != "" && !tier.Deprecated {
			return fmt.Errorf("tier '%s' has migrate_to but is not marked as deprecated", name)
		}
	}

	// Second pass: validate migration targets exist
	for name, tier := range cfg.Tiers {
		if tier.MigrateTo != "" {
			if _, exists := cfg.Tiers[tier.MigrateTo]; !exists {
				return fmt.Errorf("tier '%s' has invalid migrate_to target '%s' (tier does not exist)", name, tier.MigrateTo)
			}
		}
	}

	// Validate migration targets exist
	for name, tier := range cfg.Tiers {
		if tier.Deprecated && tier.MigrateTo != "" {
			if _, exists := cfg.Tiers[tier.MigrateTo]; !exists {
				return fmt.Errorf("tier '%s' has invalid migrate_to target '%s' (tier not found)", name, tier.MigrateTo)
			}
		}
	}

	config = &cfg
	return nil
}

// Get returns the tier details for a given tier name
// If the tier is deprecated, returns the migration target tier
func Get(tierName string) (*TierDetails, error) {
	if config == nil {
		return nil, fmt.Errorf("tier configuration not loaded")
	}

	tier, exists := config.Tiers[tierName]
	if !exists {
		return nil, fmt.Errorf("tier '%s' not found", tierName)
	}

	// If tier is deprecated and has migration target, return target tier
	if tier.Deprecated && tier.MigrateTo != "" {
		targetTier, exists := config.Tiers[tier.MigrateTo]
		if exists {
			return targetTier, nil
		}
	}

	return tier, nil
}

// GetRaw returns the tier details without following migration targets
// This is useful for admin operations that need the actual tier data
func GetRaw(tierName string) (*TierDetails, error) {
	if config == nil {
		return nil, fmt.Errorf("tier configuration not loaded")
	}

	tier, exists := config.Tiers[tierName]
	if !exists {
		return nil, fmt.Errorf("tier '%s' not found", tierName)
	}

	return tier, nil
}

// Exists checks if a tier exists
func Exists(tierName string) bool {
	if config == nil {
		return false
	}
	_, exists := config.Tiers[tierName]
	return exists
}

// List returns all tier names
func List() []string {
	if config == nil {
		return []string{}
	}

	names := make([]string, 0, len(config.Tiers))
	for name := range config.Tiers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListVisible returns all non-hidden tier names
func ListVisible() []string {
	if config == nil {
		return []string{}
	}

	names := make([]string, 0, len(config.Tiers))
	for name, tier := range config.Tiers {
		if !tier.Hidden {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// GetAll returns all tier configurations
func GetAll() map[string]*TierDetails {
	if config == nil {
		return map[string]*TierDetails{}
	}
	return config.Tiers
}

// GetAllVisible returns all non-hidden tier configurations
func GetAllVisible() map[string]*TierDetails {
	if config == nil {
		return map[string]*TierDetails{}
	}

	visible := make(map[string]*TierDetails)
	for name, tier := range config.Tiers {
		if !tier.Hidden && !tier.Deprecated {
			visible[name] = tier
		}
	}
	return visible
}

// LoadWithFallback loads tier configuration with fallback to defaults
func LoadWithFallback(path string) error {
	err := Load(path)
	if err != nil {
		// If file doesn't exist, create default configuration
		if os.IsNotExist(err) {
			config = getDefaultConfig()
			return nil
		}
		return err
	}
	return nil
}

// getDefaultConfig returns hardcoded default tier configuration
func getDefaultConfig() *TierConfig {
	return &TierConfig{
		Tiers: map[string]*TierDetails{
			"tier-1": {
				Name:                      "Free",
				DailyLimit:                10,
				MonthlyLimit:              100,
				MaxDevices:                1,
				Features:                  []string{"basic_api_access"},
				EmailVerificationRequired: true,
				Description:               "Perfect for trying out the service",
			},
			"tier-2": {
				Name:                      "Professional",
				DailyLimit:                1000,
				MonthlyLimit:              30000,
				MaxDevices:                3,
				Features:                  []string{"basic_api_access", "priority_support"},
				EmailVerificationRequired: false,
				PriceMonthly:              29.99,
				Description:               "For individual developers and small teams",
			},
			"tier-3": {
				Name:                      "Enterprise",
				DailyLimit:                -1,
				MonthlyLimit:              -1,
				MaxDevices:                -1,
				Features:                  []string{"basic_api_access", "priority_support", "api_analytics", "custom_endpoints", "sla"},
				EmailVerificationRequired: false,
				PriceMonthly:              299.99,
				CustomPricing:             true,
				Description:               "Unlimited access with dedicated support and SLA",
			},
		},
	}
}

// IsDeprecated checks if a tier is marked as deprecated
func IsDeprecated(tierName string) bool {
	if config == nil {
		return false
	}
	tier, exists := config.Tiers[tierName]
	if !exists {
		return false
	}
	return tier.Deprecated
}

// GetMigrationTarget returns the migration target for a deprecated tier
func GetMigrationTarget(tierName string) (string, error) {
	if config == nil {
		return "", fmt.Errorf("tier configuration not loaded")
	}

	tier, exists := config.Tiers[tierName]
	if !exists {
		return "", fmt.Errorf("tier '%s' not found", tierName)
	}

	if !tier.Deprecated {
		return "", fmt.Errorf("tier '%s' is not deprecated", tierName)
	}

	if tier.MigrateTo == "" {
		return "", fmt.Errorf("tier '%s' is deprecated but has no migration target", tierName)
	}

	// Validate migration target exists
	if _, exists := config.Tiers[tier.MigrateTo]; !exists {
		return "", fmt.Errorf("migration target '%s' does not exist", tier.MigrateTo)
	}

	return tier.MigrateTo, nil
}

// ListDeprecated returns all deprecated tier names
func ListDeprecated() []string {
	if config == nil {
		return []string{}
	}

	names := make([]string, 0)
	for name, tier := range config.Tiers {
		if tier.Deprecated {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ListActive returns all non-deprecated tier names
func ListActive() []string {
	if config == nil {
		return []string{}
	}

	names := make([]string, 0)
	for name, tier := range config.Tiers {
		if !tier.Deprecated {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
