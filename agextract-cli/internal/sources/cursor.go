package sources

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/agextract/agextract-cli/internal/api"
	_ "modernc.org/sqlite"
)

type Cursor struct{}

func (c *Cursor) Name() string { return "cursor" }

type cursorChatData struct {
	Tabs []cursorTab `json:"tabs"`
}

type cursorTab struct {
	ChatTitle string            `json:"chatTitle"`
	Bubbles   []cursorBubble    `json:"bubbles"`
}

type cursorBubble struct {
	Type    string `json:"type"`    // "user" or "ai"
	Text    string `json:"text"`
	RawText string `json:"rawText"`
}

func (c *Cursor) ParseFile(filePath string) (*api.SessionCreateRequest, error) {
	return parseVSCDB(filePath, "cursor", "workbench.panel.aichat.v2")
}

func parseVSCDB(filePath, source, kvKey string) (*api.SessionCreateRequest, error) {
	db, err := sql.Open("sqlite", filePath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	var value string
	err = db.QueryRow(
		"SELECT value FROM cursorDiskKV WHERE key = ?", kvKey,
	).Scan(&value)
	if err != nil {
		return nil, fmt.Errorf("reading chat data: %w", err)
	}

	var chatData cursorChatData
	if err := json.Unmarshal([]byte(value), &chatData); err != nil {
		return nil, fmt.Errorf("parsing chat data: %w", err)
	}

	req := &api.SessionCreateRequest{
		Title:  filepath.Base(filepath.Dir(filePath)),
		Source: source,
	}

	order := 1
	for _, tab := range chatData.Tabs {
		if tab.ChatTitle != "" && req.Title == filepath.Base(filepath.Dir(filePath)) {
			req.Title = tab.ChatTitle
		}
		for _, bubble := range tab.Bubbles {
			step := api.SessionStep{Order: order}

			text := bubble.Text
			if text == "" {
				text = bubble.RawText
			}
			if text == "" {
				continue
			}
			step.Content = text

			switch bubble.Type {
			case "user":
				step.Role = "user"
				step.StepType = "prompt"
			case "ai":
				step.Role = "agent"
				step.StepType = "text"
			default:
				step.Role = "agent"
				step.StepType = "text"
			}

			req.Steps = append(req.Steps, step)
			order++
		}
	}

	return req, nil
}
