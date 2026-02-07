package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/agextract/agextract-cli/internal/config"
	"github.com/agextract/agextract-cli/internal/queue"
	"github.com/agextract/agextract-cli/internal/watcher"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status, detected sources, and queue info",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Auth status
		fmt.Println("=== Authentication ===")
		if cfg.IsLoggedIn() {
			fmt.Printf("  Logged in as: %s\n", cfg.Username)
			fmt.Printf("  Server: %s\n", cfg.ServerURL)
			if cfg.ExpiresAt != "" {
				if t, err := time.Parse(time.RFC3339, cfg.ExpiresAt); err == nil {
					fmt.Printf("  Token expires: %s\n", t.Format("2006-01-02 15:04"))
				}
			}
		} else {
			fmt.Println("  Not logged in. Run 'agextract login' to authenticate.")
		}

		// Detected sources
		fmt.Println("\n=== Detected Sources ===")
		paths := watcher.DetectSources()
		if len(paths) == 0 {
			fmt.Println("  No AI tool session directories found.")
		}
		for _, sp := range paths {
			fmt.Printf("  %s: %s\n", sp.Tool, sp.Path)
		}

		// Upload ledger
		fmt.Println("\n=== Upload History ===")
		ledger, err := config.LoadUploadedLedger()
		if err != nil {
			fmt.Printf("  Could not load upload ledger: %v\n", err)
		} else {
			fmt.Printf("  Sessions uploaded: %d\n", len(ledger.Hashes))
		}

		// Retry queue
		fmt.Println("\n=== Retry Queue ===")
		q, err := queue.Open()
		if err != nil {
			fmt.Printf("  Could not open retry queue: %v\n", err)
		} else {
			count := q.Count()
			q.Close()
			fmt.Printf("  Pending retries: %d\n", count)
		}

		return nil
	},
}
