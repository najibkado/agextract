package sources

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agextract/agextract-cli/internal/api"
)

type Copilot struct{}

func (c *Copilot) Name() string { return "copilot" }

type copilotSession struct {
	Requester   string           `json:"requester"`
	Responder   string           `json:"responder"`
	Turns       []copilotTurn    `json:"turns"`
	ChatTitle   string           `json:"chatTitle"`
}

type copilotTurn struct {
	Request  copilotMessage   `json:"request"`
	Response copilotMessage   `json:"response"`
}

type copilotMessage struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (c *Copilot) ParseFile(filePath string) (*api.SessionCreateRequest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var session copilotSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}

	req := &api.SessionCreateRequest{
		Title:  session.ChatTitle,
		Source: "copilot",
	}
	if req.Title == "" {
		req.Title = filepath.Base(filePath)
	}

	order := 1
	for _, turn := range session.Turns {
		if turn.Request.Message != "" {
			req.Steps = append(req.Steps, api.SessionStep{
				Role:     "user",
				StepType: "prompt",
				Content:  turn.Request.Message,
				Order:    order,
			})
			order++
		}
		if turn.Response.Message != "" {
			req.Steps = append(req.Steps, api.SessionStep{
				Role:     "agent",
				StepType: "text",
				Content:  turn.Response.Message,
				Order:    order,
			})
			order++
		}
	}

	return req, nil
}
