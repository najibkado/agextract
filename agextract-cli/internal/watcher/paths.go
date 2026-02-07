package watcher

import (
	"os"
	"path/filepath"
	"runtime"
)

// SourcePath describes a tool's session directory.
type SourcePath struct {
	Tool string
	Path string
}

// DetectSources returns all existing AI tool session directories.
func DetectSources() []SourcePath {
	var found []SourcePath

	for _, sp := range allSourcePaths() {
		if info, err := os.Stat(sp.Path); err == nil && info.IsDir() {
			found = append(found, sp)
		}
	}

	return found
}

func allSourcePaths() []SourcePath {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return []SourcePath{
			{Tool: "claudecode", Path: filepath.Join(home, ".claude", "projects")},
			{Tool: "cursor", Path: filepath.Join(home, "Library", "Application Support", "Cursor", "User", "workspaceStorage")},
			{Tool: "windsurf", Path: filepath.Join(home, "Library", "Application Support", "Windsurf", "User", "workspaceStorage")},
			{Tool: "copilot", Path: filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")},
		}
	case "linux":
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			configDir = filepath.Join(home, ".config")
		}
		return []SourcePath{
			{Tool: "claudecode", Path: filepath.Join(home, ".claude", "projects")},
			{Tool: "cursor", Path: filepath.Join(configDir, "Cursor", "User", "workspaceStorage")},
			{Tool: "windsurf", Path: filepath.Join(configDir, "Windsurf", "User", "workspaceStorage")},
			{Tool: "copilot", Path: filepath.Join(configDir, "Code", "User", "workspaceStorage")},
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		return []SourcePath{
			{Tool: "claudecode", Path: filepath.Join(home, ".claude", "projects")},
			{Tool: "cursor", Path: filepath.Join(appData, "Cursor", "User", "workspaceStorage")},
			{Tool: "windsurf", Path: filepath.Join(appData, "Windsurf", "User", "workspaceStorage")},
			{Tool: "copilot", Path: filepath.Join(appData, "Code", "User", "workspaceStorage")},
		}
	default:
		return []SourcePath{
			{Tool: "claudecode", Path: filepath.Join(home, ".claude", "projects")},
		}
	}
}
