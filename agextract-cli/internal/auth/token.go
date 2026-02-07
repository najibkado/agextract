package auth

import (
	"time"

	"github.com/agextract/agextract-cli/internal/api"
	"github.com/agextract/agextract-cli/internal/config"
)

// ExchangeCode exchanges an OAuth authorization code for tokens and saves them.
func ExchangeCode(cfg *config.Config, code string) error {
	client := api.NewClient(cfg)
	resp, err := client.ExchangeToken(code)
	if err != nil {
		return err
	}

	cfg.AccessToken = resp.AccessToken
	cfg.RefreshToken = resp.RefreshToken
	cfg.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).Format(time.RFC3339)

	// Fetch user info (need new client since token was just set)
	authedClient := api.NewClient(cfg)
	user, err := authedClient.Me()
	if err == nil {
		cfg.Username = user.Username
	}

	return cfg.Save()
}

// RefreshIfNeeded checks token expiry and refreshes if within 24 hours of expiry.
func RefreshIfNeeded(cfg *config.Config) error {
	if cfg.ExpiresAt == "" {
		return nil
	}

	expires, err := time.Parse(time.RFC3339, cfg.ExpiresAt)
	if err != nil {
		return nil
	}

	// Refresh if expiring within 24 hours
	if time.Until(expires) > 24*time.Hour {
		return nil
	}

	client := api.NewClient(cfg)
	resp, err := client.RefreshToken(cfg.RefreshToken)
	if err != nil {
		return err
	}

	cfg.AccessToken = resp.AccessToken
	cfg.RefreshToken = resp.RefreshToken
	cfg.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).Format(time.RFC3339)
	return cfg.Save()
}
