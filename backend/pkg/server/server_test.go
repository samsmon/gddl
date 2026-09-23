package server

import (
	"net/http/httptest"
	"testing"

	"gdrive-downloader/pkg/auth"
	"gdrive-downloader/pkg/gdrive"
)

func TestGetOAuthRedirectURI(t *testing.T) {
	authMgr, err := auth.NewManager(t.TempDir()+"/config.json", t.TempDir(), 3)
	if err != nil {
		t.Fatalf("failed to init auth manager: %v", err)
	}

	oauthMgr := gdrive.NewOAuthManager("id", "sec", "", nil, "", nil)
	s := &Server{
		authMgr:  authMgr,
		oauthMgr: oauthMgr,
	}

	tests := []struct {
		name     string
		host     string
		scheme   string
		override string
		expected string
	}{
		{
			name:     "LAN private IP 192.168.x.x with custom port",
			host:     "192.168.18.228:8099",
			scheme:   "http",
			expected: "http://localhost:8099/api/gdrive/oauth/callback",
		},
		{
			name:     "LAN private IP 10.x.x.x with custom port",
			host:     "10.0.0.15:3000",
			scheme:   "http",
			expected: "http://localhost:3000/api/gdrive/oauth/callback",
		},
		{
			name:     "LAN private IP 172.16.x.x with port",
			host:     "172.20.10.4:8080",
			scheme:   "http",
			expected: "http://localhost:8080/api/gdrive/oauth/callback",
		},
		{
			name:     "127.0.0.1 normalized to localhost",
			host:     "127.0.0.1:8099",
			scheme:   "http",
			expected: "http://localhost:8099/api/gdrive/oauth/callback",
		},
		{
			name:     "localhost with port",
			host:     "localhost:8099",
			scheme:   "http",
			expected: "http://localhost:8099/api/gdrive/oauth/callback",
		},
		{
			name:     "public domain name without IP",
			host:     "gddl.mycompany.org",
			scheme:   "https",
			expected: "https://gddl.mycompany.org/api/gdrive/oauth/callback",
		},
		{
			name:     "explicit override configured",
			host:     "192.168.18.228:8099",
			scheme:   "http",
			override: "https://tunnel.example.com/api/gdrive/oauth/callback",
			expected: "https://tunnel.example.com/api/gdrive/oauth/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = authMgr.SetOAuthCredentials("id", "sec", tt.override, true)
			req := httptest.NewRequest("GET", "http://"+tt.host+"/api/gdrive/oauth/auth-url", nil)
			req.Host = tt.host
			if tt.scheme == "https" {
				req.Header.Set("X-Forwarded-Proto", "https")
			}

			got := s.getOAuthRedirectURI(req)
			if got != tt.expected {
				t.Errorf("getOAuthRedirectURI() = %q; want %q", got, tt.expected)
			}
		})
	}
}
