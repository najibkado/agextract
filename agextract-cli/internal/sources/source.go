package sources

import "github.com/agextract/agextract-cli/internal/api"

// SessionSource defines the interface for parsing sessions from different AI tools.
type SessionSource interface {
	// Name returns the tool identifier (e.g., "claudecode", "cursor").
	Name() string

	// ParseFile reads a session file and returns a structured session request.
	ParseFile(filePath string) (*api.SessionCreateRequest, error)
}
