package sources

import "github.com/agextract/agextract-cli/internal/api"

type Windsurf struct{}

func (w *Windsurf) Name() string { return "windsurf" }

func (w *Windsurf) ParseFile(filePath string) (*api.SessionCreateRequest, error) {
	// Windsurf uses the same vscdb format as Cursor
	return parseVSCDB(filePath, "windsurf", "workbench.panel.aichat.v2")
}
