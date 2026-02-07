package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/agextract/agextract-cli/internal/config"
)

type Client struct {
	httpClient *http.Client
	serverURL  string
	token      string
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{},
		serverURL:  cfg.ServerURL,
		token:      cfg.AccessToken,
	}
}

func (c *Client) doJSON(method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.serverURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// ExchangeToken exchanges an OAuth code for tokens.
func (c *Client) ExchangeToken(code string) (*TokenResponse, error) {
	payload := map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	}
	var resp TokenResponse
	err := c.doJSON("POST", "/api/v1/oauth/token/", payload, &resp)
	return &resp, err
}

// RefreshToken refreshes an expired token.
func (c *Client) RefreshToken(refreshToken string) (*TokenResponse, error) {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}
	var resp TokenResponse
	err := c.doJSON("POST", "/api/v1/oauth/token/", payload, &resp)
	return &resp, err
}

// RevokeToken revokes the current token.
func (c *Client) RevokeToken() error {
	return c.doJSON("POST", "/api/v1/oauth/revoke/", map[string]string{}, nil)
}

// Me returns the current user.
func (c *Client) Me() (*UserResponse, error) {
	var resp UserResponse
	err := c.doJSON("GET", "/api/v1/me/", nil, &resp)
	return &resp, err
}

// CreateSession creates a session from structured JSON.
func (c *Client) CreateSession(req *SessionCreateRequest) (*SessionResponse, error) {
	var resp SessionResponse
	err := c.doJSON("POST", "/api/v1/sessions/", req, &resp)
	return &resp, err
}

// UploadFile uploads a raw file to the server.
func (c *Client) UploadFile(filePath string, source string) (*SessionResponse, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copying file: %w", err)
	}

	if source != "" {
		writer.WriteField("source", source)
	}
	writer.Close()

	req, err := http.NewRequest("POST", c.serverURL+"/api/v1/sessions/upload/", &buf)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var sessionResp SessionResponse
	if err := json.Unmarshal(respBody, &sessionResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &sessionResp, nil
}
