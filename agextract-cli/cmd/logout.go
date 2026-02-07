package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agextract/agextract-cli/internal/api"
	"github.com/agextract/agextract-cli/internal/config"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke token and clear local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if !cfg.IsLoggedIn() {
			fmt.Println("Not currently logged in.")
			return nil
		}

		// Try to revoke server-side (best effort)
		client := api.NewClient(cfg)
		if err := client.RevokeToken(); err != nil {
			fmt.Printf("Warning: could not revoke token server-side: %v\n", err)
		}

		cfg.ClearAuth()
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Println("Logged out successfully.")
		return nil
	},
}
