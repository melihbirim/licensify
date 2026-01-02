package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	gitCommit = "none"
	buildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "licensify",
	Short: "Licensify CLI - Manage your licenses",
	Long: `Licensify CLI is a command-line tool for managing your Licensify licenses.
	
It allows you to request, verify, activate, and check licenses from your terminal.`,
	Version: version,
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("licensify version %s (commit: %s, built: %s)\n", version, gitCommit, buildTime))

	// Add all commands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
