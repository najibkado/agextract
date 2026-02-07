package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agextract/agextract-cli/internal/auth"
	"github.com/agextract/agextract-cli/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with agextract via browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.IsLoggedIn() {
			fmt.Printf("Already logged in as %s. Use 'agextract logout' first.\n", cfg.Username)
			return nil
		}

		fmt.Println("Opening browser for login...")

		result, err := auth.StartOAuthFlow(cfg.ServerURL)
		if err != nil {
			return fmt.Errorf("OAuth flow failed: %w", err)
		}

		if result.Error != "" {
			return fmt.Errorf("login failed: %s", result.Error)
		}

		fmt.Println("Exchanging code for token...")

		if err := auth.ExchangeCode(cfg, result.Code); err != nil {
			return fmt.Errorf("token exchange failed: %w", err)
		}

		// Reload config to get username
		cfg, _ = config.Load()
		fmt.Printf("Logged in as %s\n", cfg.Username)
		return nil
	},
}
