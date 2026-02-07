package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

const (
	CallbackPort = 19284
	CallbackPath = "/callback"
)

type OAuthResult struct {
	Code  string
	State string
	Error string
}

// StartOAuthFlow opens the browser for login and waits for the callback.
func StartOAuthFlow(serverURL string) (*OAuthResult, error) {
	state, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	resultCh := make(chan *OAuthResult, 1)

	// Start local callback server
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		returnedState := r.URL.Query().Get("state")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h2>Login failed: %s</h2><p>You can close this tab.</p></body></html>", errParam)
			resultCh <- &OAuthResult{Error: errParam}
			return
		}

		if returnedState != state {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h2>State mismatch — possible CSRF attack.</h2><p>Please try again.</p></body></html>")
			resultCh <- &OAuthResult{Error: "state mismatch"}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Login successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		resultCh <- &OAuthResult{Code: code, State: returnedState}
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", CallbackPort))
	if err != nil {
		return nil, fmt.Errorf("starting callback server: %w", err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	// Build authorize URL and open browser
	redirectURI := fmt.Sprintf("http://localhost:%d%s", CallbackPort, CallbackPath)
	authorizeURL := fmt.Sprintf(
		"%s/api/v1/oauth/authorize/?redirect_uri=%s&state=%s",
		serverURL, redirectURI, state,
	)

	if err := openBrowser(authorizeURL); err != nil {
		fmt.Printf("Could not open browser automatically.\nPlease visit: %s\n", authorizeURL)
	}

	// Wait for callback (timeout after 5 minutes)
	select {
	case result := <-resultCh:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		return result, nil
	case <-time.After(5 * time.Minute):
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		return nil, fmt.Errorf("login timed out after 5 minutes")
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	return cmd.Start()
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
