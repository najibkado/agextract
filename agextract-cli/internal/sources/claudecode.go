package sources

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agextract/agextract-cli/internal/api"
)

type ClaudeCode struct{}

func (c *ClaudeCode) Name() string { return "claudecode" }

// jsonlEntry represents a single line in a Claude Code JSONL file.
// Real format: {"type":"user"|"assistant"|"system", "message":{"role":"...", "content": ...}, "sessionId":"...", ...}
type jsonlEntry struct {
	Type      string     `json:"type"`
	SessionID string     `json:"sessionId"`
	Message   jsonlMsg   `json:"message"`
}

type jsonlMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// contentBlock represents a block in the content array.
type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

func (c *ClaudeCode) ParseFile(filePath string) (*api.SessionCreateRequest, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	req := &api.SessionCreateRequest{
		Title:  filepath.Base(filePath),
		Source: "claudecode",
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line
	order := 1

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry jsonlEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Pick up sessionId for idempotency
		if entry.SessionID != "" && req.SourceSessionID == "" {
			req.SourceSessionID = entry.SessionID
		}

		// Skip non-conversation entries
		switch entry.Type {
		case "user", "human":
			// ok
		case "assistant", "agent":
			// ok
		case "system":
			// ok
		default:
			continue
		}

		// Extract text from message.content
		text, stepType := extractMessageContent(entry.Message.Content)
		if text == "" {
			continue
		}

		role := "agent"
		if entry.Type == "user" || entry.Type == "human" {
			role = "user"
			if stepType == "" {
				stepType = "prompt"
			}
		} else if entry.Type == "system" {
			role = "system"
		}
		if stepType == "" {
			stepType = "text"
		}

		req.Steps = append(req.Steps, api.SessionStep{
			Role:     role,
			StepType: stepType,
			Content:  text,
			Order:    order,
		})
		order++
	}

	return req, scanner.Err()
}

// extractMessageContent handles both string content and array-of-blocks content.
func extractMessageContent(raw json.RawMessage) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}

	// Try string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, ""
	}

	// Try array of content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", ""
	}

	var parts []string
	stepType := ""

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "tool_use":
			stepType = "tool_call"
			desc := fmt.Sprintf("[Tool: %s]", block.Name)
			if len(block.Input) > 0 {
				inputStr := string(block.Input)
				if len(inputStr) > 500 {
					inputStr = inputStr[:500] + "..."
				}
				desc += " " + inputStr
			}
			parts = append(parts, desc)
		case "tool_result":
			var resultContent string
			// tool_result content can be string or nested blocks
			if json.Unmarshal(block.Input, &resultContent) == nil {
				parts = append(parts, resultContent)
			}
		}
	}

	return strings.Join(parts, "\n"), stepType
}
