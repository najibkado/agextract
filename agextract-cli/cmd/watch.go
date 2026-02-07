package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/agextract/agextract-cli/internal/api"
	"github.com/agextract/agextract-cli/internal/auth"
	"github.com/agextract/agextract-cli/internal/config"
	"github.com/agextract/agextract-cli/internal/queue"
	"github.com/agextract/agextract-cli/internal/sources"
	"github.com/agextract/agextract-cli/internal/watcher"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch AI tool session files and auto-upload on completion",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		ledger, err := config.LoadUploadedLedger()
		if err != nil {
			return fmt.Errorf("loading upload ledger: %w", err)
		}

		retryQueue, err := queue.Open()
		if err != nil {
			return fmt.Errorf("opening retry queue: %w", err)
		}
		defer retryQueue.Close()

		// Detect watch paths
		sources := watcher.DetectSources()
		if len(sources) == 0 {
			return fmt.Errorf("no AI tool session directories found")
		}

		fmt.Println("Watching for session changes:")
		for _, s := range sources {
			fmt.Printf("  %s: %s\n", s.Tool, s.Path)
		}

		// Create watcher with upload callback
		w, err := watcher.New(sources, func(filePath, tool string) {
			handleFileReady(cfg, ledger, retryQueue, filePath, tool)
		})
		if err != nil {
			return fmt.Errorf("creating watcher: %w", err)
		}
		defer w.Close()

		// Process retry queue in background
		go retryQueue.ProcessLoop(func(item queue.RetryItem) error {
			return uploadFile(cfg, item.FilePath, item.Tool)
		})

		fmt.Println("\nWatching... Press Ctrl+C to stop.")

		// Wait for interrupt
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println("\nStopping watcher...")
		return nil
	},
}

func handleFileReady(cfg *config.Config, ledger *config.UploadedLedger, retryQueue *queue.RetryQueue, filePath, tool string) {
	// Compute hash for dedup
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", filePath, err)
		return
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	if ledger.HasHash(hashStr) {
		return // Already uploaded
	}

	fmt.Printf("Session ready: %s (%s)\n", filePath, tool)

	if err := uploadFile(cfg, filePath, tool); err != nil {
		fmt.Printf("Upload failed, queuing for retry: %v\n", err)
		retryQueue.Add(filePath, tool)
		return
	}

	// Mark as uploaded
	ledger.AddHash(hashStr)
	if err := ledger.Save(); err != nil {
		fmt.Printf("Warning: could not save upload ledger: %v\n", err)
	}
}

func uploadFile(cfg *config.Config, filePath, tool string) error {
	if err := auth.RefreshIfNeeded(cfg); err != nil {
		return fmt.Errorf("token refresh: %w", err)
	}

	client := api.NewClient(cfg)

	// Try structured parsing first (Go-side parsers)
	parser := getParser(tool)
	if parser != nil {
		req, err := parser.ParseFile(filePath)
		if err == nil && len(req.Steps) > 0 {
			// Use the filename stem as source_session_id for idempotency
			base := filepath.Base(filePath)
			req.SourceSessionID = strings.TrimSuffix(base, filepath.Ext(base))

			resp, err := client.CreateSession(req)
			if err != nil {
				return err
			}
			fmt.Printf("Uploaded: %s → session %s (%d steps)\n", filePath, resp.ID, len(req.Steps))
			return nil
		}
		// Fall through to raw upload if parsing failed
		if err != nil {
			fmt.Printf("Warning: structured parse failed, falling back to raw upload: %v\n", err)
		}
	}

	// Fallback: raw file upload
	resp, err := client.UploadFile(filePath, tool)
	if err != nil {
		return err
	}

	fmt.Printf("Uploaded: %s → session %s\n", filePath, resp.ID)
	return nil
}

func getParser(tool string) sources.SessionSource {
	switch tool {
	case "claudecode":
		return &sources.ClaudeCode{}
	case "cursor":
		return &sources.Cursor{}
	case "windsurf":
		return &sources.Windsurf{}
	case "copilot":
		return &sources.Copilot{}
	default:
		return nil
	}
}
