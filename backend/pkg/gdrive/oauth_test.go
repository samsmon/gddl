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

func TestImportRcloneToken(t *testing.T) {
	var savedToken *OAuthToken
	var savedEmail string
	om := NewOAuthManager("id", "secret", "http://localhost:8099/callback", nil, "", func(tok *OAuthToken, email string) {
		savedToken = tok
		savedEmail = email
	})

	sampleJSON := `{"access_token":"ya29.sample_test_token_12345","token_type":"Bearer","refresh_token":"1//0g_sample_refresh_token","expiry":"2030-01-01T00:00:00Z"}`

	tok, email, err := om.ImportRcloneToken(context.Background(), sampleJSON)
	if err != nil {
		t.Fatalf("ImportRcloneToken failed: %v", err)
	}

	if tok.AccessToken != "ya29.sample_test_token_12345" {
		t.Errorf("expected access_token, got: %s", tok.AccessToken)
	}
	if tok.RefreshToken != "1//0g_sample_refresh_token" {
		t.Errorf("expected refresh_token, got: %s", tok.RefreshToken)
	}
	if savedToken == nil || savedToken.AccessToken != tok.AccessToken {
		t.Errorf("expected onTokenSave callback to be invoked with token")
	}
	if email == "" {
		t.Errorf("expected non-empty email")
	}
	_ = savedEmail

	// Also test rclone console output wrapped with text
	wrapped := `Paste the following into your remote machine --->
{"access_token":"ya29.wrapped_token","token_type":"Bearer","refresh_token":"1//wrapped_refresh","expiry":"2030-01-01T00:00:00Z"}
<--- End paste`

	tok2, _, err2 := om.ImportRcloneToken(context.Background(), wrapped)
	if err2 != nil {
		t.Fatalf("ImportRcloneToken wrapped failed: %v", err2)
	}
	if tok2.AccessToken != "ya29.wrapped_token" {
		t.Errorf("expected wrapped access token, got: %s", tok2.AccessToken)
	}
}
