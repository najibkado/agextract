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
		text, stepType, hasToolResult := extractMessageContent(entry.Message.Content)
		if text == "" {
			continue
		}

		role := "agent"
		if entry.Type == "user" || entry.Type == "human" {
			if hasToolResult {
				// tool_result relays are sent as type=user by the API
				// but they're not real human inputs
				role = "system"
				if stepType == "" {
					stepType = "text"
				}
			} else {
				role = "user"
				if stepType == "" {
					stepType = "prompt"
				}
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
// Returns (text, stepType, hasToolResult).
func extractMessageContent(raw json.RawMessage) (string, string, bool) {
	if len(raw) == 0 {
		return "", "", false
	}

	// Try string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, "", false
	}

	// Try array of content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", "", false
	}

	var parts []string
	stepType := ""
	hasToolResult := false

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
			hasToolResult = true
			// Extract content from tool_result block
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
			// tool_result may have nested content in a 'content' field
			// which we stored in Input for simplicity — try to parse it
			if len(block.Input) > 0 {
				var resultStr string
				if json.Unmarshal(block.Input, &resultStr) == nil {
					parts = append(parts, resultStr)
				}
			}
		}
	}

	return strings.Join(parts, "\n"), stepType, hasToolResult
}
