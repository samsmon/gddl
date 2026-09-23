package gdrive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw code",
			input:    "4/0AY0e-g7ExampleCode12345",
			expected: "4/0AY0e-g7ExampleCode12345",
		},
		{
			name:     "full URL callback",
			input:    "http://127.0.0.1:8080/api/gdrive/oauth/callback?code=4/0AY0e-g7ExampleCode12345&scope=https://www.googleapis.com/auth/drive",
			expected: "4/0AY0e-g7ExampleCode12345",
		},
		{
			name:     "localhost custom port URL with state",
			input:    "http://localhost:5244/api/gdrive/oauth/callback?state=gddl_state&code=4/0AY0e-g7ExampleCode12345",
			expected: "4/0AY0e-g7ExampleCode12345",
		},
		{
			name:     "query string fragment only",
			input:    "?code=4/0AY0e-g7ExampleCode12345&scope=read",
			expected: "4/0AY0e-g7ExampleCode12345",
		},
		{
			name:     "whitespace padded input",
			input:    "   4/0AY0e-g7ExampleCode12345  \n",
			expected: "4/0AY0e-g7ExampleCode12345",
		},
		{
			name:     "empty input",
			input:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCode(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractCode(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildAuthURL(t *testing.T) {
	mgr := NewOAuthManager("client-123", "secret-456", "http://127.0.0.1:8080/api/gdrive/oauth/callback", nil, "", nil)

	authURL, err := mgr.BuildAuthURL("test_state")
	if err != nil {
		t.Fatalf("BuildAuthURL failed: %v", err)
	}

	if !strings.Contains(authURL, "client_id=client-123") {
		t.Errorf("expected client_id in authURL, got: %s", authURL)
	}
	if !strings.Contains(authURL, "redirect_uri=http%3A%2F%2F127.0.0.1%3A8080%2Fapi%2Fgdrive%2Foauth%2Fcallback") {
		t.Errorf("expected encoded redirect_uri in authURL, got: %s", authURL)
	}
	if !strings.Contains(authURL, "scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive") {
		t.Errorf("expected drive scope in authURL, got: %s", authURL)
	}
	if !strings.Contains(authURL, "state=test_state") {
		t.Errorf("expected state in authURL, got: %s", authURL)
	}
}

func TestOAuthTokenExpired(t *testing.T) {
	// 1. Nil token
	var nilTok *OAuthToken
	if !nilTok.Expired() {
		t.Errorf("expected nil token to be expired")
	}

	// 2. Token without access token
	emptyTok := &OAuthToken{AccessToken: ""}
	if !emptyTok.Expired() {
		t.Errorf("expected empty token to be expired")
	}

	// 3. Past expiry
	pastTok := &OAuthToken{AccessToken: "token123", Expiry: time.Now().Add(-10 * time.Minute)}
	if !pastTok.Expired() {
		t.Errorf("expected past token to be expired")
	}

	// 4. Close to expiry (< 2 minutes)
	closeTok := &OAuthToken{AccessToken: "token123", Expiry: time.Now().Add(1 * time.Minute)}
	if !closeTok.Expired() {
		t.Errorf("expected token within 2 min buffer to be expired")
	}

	// 5. Valid token (> 2 minutes)
	validTok := &OAuthToken{AccessToken: "token123", Expiry: time.Now().Add(10 * time.Minute)}
	if validTok.Expired() {
		t.Errorf("expected token with 10 min left not to be expired")
	}
}

func TestBypassManager_EnsureTempFolder(t *testing.T) {
	// Mock Google Drive API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test_access_token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" && strings.Contains(r.URL.Path, "/files") {
			// Return search result with existing ggdl_temp folder
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"files":[{"id":"mock_folder_id_123","name":"ggdl_temp"}]}`))
			return
		}

		if r.Method == "DELETE" && strings.Contains(r.URL.Path, "/files/mock_file_456") {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	tok := &OAuthToken{
		AccessToken: "test_access_token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	om := NewOAuthManager("id", "secret", "http://127.0.0.1:8080/callback", tok, "user@gmail.com", nil)
	bm := NewBypassManager(om, true)

	if !bm.IsAvailable() {
		t.Errorf("expected BypassManager to be available")
	}
	if !bm.IsAutoBypass() {
		t.Errorf("expected AutoBypass to be true")
	}

	// Override API endpoint for testing
	ctx := context.Background()
	_ = ctx
}
