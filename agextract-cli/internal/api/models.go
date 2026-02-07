package api

// TokenResponse is returned from POST /api/v1/oauth/token/
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// UserResponse is returned from GET /api/v1/me/
type UserResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// SessionStep represents a single step in a session payload.
type SessionStep struct {
	Role     string `json:"role"`
	StepType string `json:"step_type"`
	Content  string `json:"content"`
	Order    int    `json:"order"`
}

// SessionCreateRequest is the payload for POST /api/v1/sessions/
type SessionCreateRequest struct {
	Title           string        `json:"title"`
	Source          string        `json:"source"`
	SourceSessionID string        `json:"source_session_id"`
	DurationSeconds *int          `json:"duration_seconds,omitempty"`
	TokenUsage      *int          `json:"token_usage,omitempty"`
	FileCount       *int          `json:"file_count,omitempty"`
	Steps           []SessionStep `json:"steps"`
}

// SessionResponse is returned from session endpoints.
type SessionResponse struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Source          string        `json:"source"`
	SourceSessionID string        `json:"source_session_id"`
	UploadedAt      string        `json:"uploaded_at"`
	DurationSeconds *int          `json:"duration_seconds"`
	TokenUsage      *int          `json:"token_usage"`
	FileCount       *int          `json:"file_count"`
	Steps           []SessionStep `json:"steps"`
}

// ErrorResponse is returned on API errors.
type ErrorResponse struct {
	Error string `json:"error"`
}
