package server

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	errParam := r.URL.Query().Get("error")
	if errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		if errDesc == "" {
			errDesc = errParam
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Authentication Error</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
.card { background: #1e293b; border: 1px solid #ef4444; border-radius: 12px; padding: 28px; max-width: 480px; text-align: center; }
h2 { color: #f87171; margin-top: 0; }
p { color: #94a3b8; font-size: 14px; }
button { margin-top: 16px; padding: 8px 20px; background: #334155; color: white; border: none; border-radius: 6px; cursor: pointer; }
</style>
</head>
<body>
<div class="card">
  <h2>Google Authorization Failed</h2>
  <p>%s</p>
  <button onclick="window.close()">Close Window</button>
</div>
</body>
</html>`, html.EscapeString(errDesc))
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code parameter", http.StatusBadRequest)
		return
	}

	redirectURI := s.getOAuthRedirectURI(r)
	clientID, clientSecret, _, _, _, _ := s.authMgr.GetOAuthSettings()
	s.oauthMgr.UpdateConfig(clientID, clientSecret, redirectURI)

	_, email, err := s.oauthMgr.ExchangeCode(r.Context(), code)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Token Exchange Failed</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
.card { background: #1e293b; border: 1px solid #ef4444; border-radius: 12px; padding: 28px; max-width: 480px; text-align: center; }
h2 { color: #f87171; margin-top: 0; }
p { color: #94a3b8; font-size: 14px; word-break: break-all; }
button { margin-top: 16px; padding: 8px 20px; background: #334155; color: white; border: none; border-radius: 6px; cursor: pointer; }
</style>
</head>
<body>
<div class="card">
  <h2>Token Exchange Failed</h2>
  <p>%s</p>
  <button onclick="window.close()">Close Window</button>
</div>
</body>
</html>`, html.EscapeString(err.Error()))
		return
	}

	scheme := s.getRequestScheme(r)
	fullURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	htmlTmpl := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Google Drive Authenticated - GDDL</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: #090d16;
      color: #f1f5f9;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      padding: 20px;
      box-sizing: border-box;
    }
    .card {
      background: #111827;
      border: 1px solid #10b981;
      border-radius: 16px;
      padding: 32px;
      max-width: 560px;
      width: 100%;
      box-shadow: 0 20px 40px rgba(0,0,0,0.6);
      text-align: center;
    }
    .icon {
      width: 56px;
      height: 56px;
      background: rgba(16, 185, 129, 0.15);
      color: #10b981;
      border-radius: 50%;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 28px;
      margin-bottom: 16px;
    }
    h2 {
      margin: 0 0 8px 0;
      font-size: 22px;
      font-weight: 700;
      color: #ffffff;
    }
    .email {
      font-size: 15px;
      color: #10b981;
      font-weight: 600;
      background: rgba(16, 185, 129, 0.1);
      padding: 6px 14px;
      border-radius: 20px;
      display: inline-block;
      margin: 8px 0 16px 0;
    }
    p {
      color: #94a3b8;
      font-size: 14px;
      line-height: 1.5;
      margin: 0 0 20px 0;
    }
    .manual-box {
      background: #1e293b;
      border: 1px solid #334155;
      border-radius: 10px;
      padding: 16px;
      text-align: left;
      margin-top: 16px;
    }
    .manual-label {
      font-size: 11px;
      color: #94a3b8;
      font-weight: 600;
      text-transform: uppercase;
      margin-bottom: 8px;
      display: block;
      letter-spacing: 0.05em;
    }
    .code-input {
      width: 100%;
      background: #0f172a;
      border: 1px solid #334155;
      border-radius: 6px;
      color: #38bdf8;
      padding: 8px 10px;
      font-family: monospace;
      font-size: 12px;
      box-sizing: border-box;
      margin-bottom: 8px;
    }
    .btn-row {
      display: flex;
      gap: 10px;
      justify-content: center;
      margin-top: 20px;
    }
    .btn {
      padding: 9px 18px;
      border-radius: 8px;
      border: none;
      font-weight: 600;
      font-size: 13px;
      cursor: pointer;
      transition: all 0.2s;
    }
    .btn-primary {
      background: #10b981;
      color: #0f172a;
    }
    .btn-primary:hover {
      background: #059669;
    }
    .btn-secondary {
      background: #334155;
      color: #f1f5f9;
    }
    .btn-secondary:hover {
      background: #475569;
    }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">✓</div>
    <h2>Google Drive Connected!</h2>
    <div class="email">{{EMAIL}}</div>
    <p>Authentication was successful. GDDL can now automatically clone and bypass quota-exceeded files into <b>ggdl_temp</b>.</p>

    <div class="manual-box">
      <span class="manual-label">Callback URL / Authorization Code (rclone style)</span>
      <input type="text" id="urlInput" class="code-input" readonly value="{{FULL_URL}}" />
      <button class="btn btn-secondary" style="width: 100%;" onclick="copyURL()">Copy Callback URL</button>
    </div>

    <div class="btn-row">
      <button class="btn btn-primary" onclick="closeTab()">Close Window</button>
    </div>
  </div>

  <script>
    function copyURL() {
      const el = document.getElementById('urlInput');
      el.select();
      navigator.clipboard.writeText(el.value);
      alert('Callback URL copied to clipboard!');
    }
    function closeTab() {
      window.close();
    }
    try {
      if (window.opener) {
        window.opener.postMessage({ type: 'gdrive-oauth-success', email: '{{EMAIL}}' }, '*');
        setTimeout(function() { window.close(); }, 2500);
      }
    } catch(e) {}
  </script>
</body>
</html>`

	htmlOut := strings.ReplaceAll(htmlTmpl, "{{EMAIL}}", html.EscapeString(email))
	htmlOut = strings.ReplaceAll(htmlOut, "{{FULL_URL}}", html.EscapeString(fullURL))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(htmlOut))
}
