package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultServerURL = "http://localhost:8000"
	ConfigDirName    = ".agextract"
	ConfigFileName   = "config.json"
	UploadedFileName = "uploaded.json"
)

type Config struct {
	ServerURL    string `json:"server_url"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Username     string `json:"username,omitempty"`
}

type UploadedLedger struct {
	Hashes map[string]time.Time `json:"hashes"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ConfigDirName)
}

func Path() string {
	return filepath.Join(Dir(), ConfigFileName)
}

func UploadedPath() string {
	return filepath.Join(Dir(), UploadedFileName)
}

func EnsureDir() error {
	return os.MkdirAll(Dir(), 0700)
}

func Load() (*Config, error) {
	cfg := &Config{ServerURL: DefaultServerURL}

	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultServerURL
	}
	return cfg, nil
}

func (c *Config) Save() error {
	if err := EnsureDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(Path(), data, 0600)
}

func (c *Config) IsLoggedIn() bool {
	return c.AccessToken != ""
}

func (c *Config) ClearAuth() {
	c.AccessToken = ""
	c.RefreshToken = ""
	c.ExpiresAt = ""
	c.Username = ""
}

func LoadUploadedLedger() (*UploadedLedger, error) {
	ledger := &UploadedLedger{Hashes: make(map[string]time.Time)}

	data, err := os.ReadFile(UploadedPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ledger, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, ledger); err != nil {
		return nil, err
	}
	return ledger, nil
}

func (l *UploadedLedger) Save() error {
	if err := EnsureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(UploadedPath(), data, 0600)
}

func (l *UploadedLedger) HasHash(hash string) bool {
	_, exists := l.Hashes[hash]
	return exists
}

func (l *UploadedLedger) AddHash(hash string) {
	l.Hashes[hash] = time.Now()
}
