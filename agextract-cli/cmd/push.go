package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agextract/agextract-cli/internal/api"
	"github.com/agextract/agextract-cli/internal/auth"
	"github.com/agextract/agextract-cli/internal/config"
)

var pushCmd = &cobra.Command{
	Use:   "push <file>",
	Short: "Upload a session file (.md, .jsonl) to agextract",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if !cfg.IsLoggedIn() {
			return fmt.Errorf("not logged in — run 'agextract login' first")
		}

		if err := auth.RefreshIfNeeded(cfg); err != nil {
			fmt.Printf("Warning: token refresh failed: %v\n", err)
		}

		// Auto-detect source from file extension/content
		source := detectSource(filePath)

		fmt.Printf("Uploading %s (source: %s)...\n", filepath.Base(filePath), source)

		client := api.NewClient(cfg)
		resp, err := client.UploadFile(filePath, source)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		fmt.Printf("Session created: %s\n", resp.ID)
		fmt.Printf("Title: %s\n", resp.Title)
		fmt.Printf("View at: %s/session/%s/\n", cfg.ServerURL, resp.ID)
		return nil
	},
}

func detectSource(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	name := strings.ToLower(filepath.Base(filePath))

	switch {
	case ext == ".jsonl":
		return "claudecode"
	case strings.Contains(name, "cursor"):
		return "cursor"
	case strings.Contains(name, "windsurf"):
		return "windsurf"
	case strings.Contains(name, "copilot"):
		return "copilot"
	default:
		return "upload"
	}
}
