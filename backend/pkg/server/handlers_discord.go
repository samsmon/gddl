package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

func (s *Server) handleGetUnfinishedDiscordItems(w http.ResponseWriter, r *http.Request) {
	items := s.manager.GetUnfinishedDiscordItems()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (s *Server) handleRefreshDiscordURLs(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024*10)) // up to 10MB input
	if err != nil {
		http.Error(w, `{"error":"failed reading request"}`, http.StatusBadRequest)
		return
	}

	var urls []string

	// 1. Try parsing as JSON object { "urls": [...] } or { "raw": "..." }
	var objReq struct {
		URLs []string `json:"urls"`
		Raw  string   `json:"raw"`
	}
	if err := json.Unmarshal(bodyBytes, &objReq); err == nil {
		if len(objReq.URLs) > 0 {
			urls = append(urls, objReq.URLs...)
		}
		if objReq.Raw != "" {
			for _, line := range strings.Split(objReq.Raw, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					urls = append(urls, line)
				}
			}
		}
	} else {
		// 2. Try parsing as JSON array of strings ["https://...", ...]
		var arrReq []string
		if err := json.Unmarshal(bodyBytes, &arrReq); err == nil {
			urls = append(urls, arrReq...)
		} else {
			// 3. Try parsing as JSON array of objects [{"url": "...", "filename": "..."}, ...]
			var objArr []struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(bodyBytes, &objArr); err == nil && len(objArr) > 0 {
				for _, item := range objArr {
					if item.URL != "" {
						urls = append(urls, item.URL)
					}
				}
			} else {
				// 4. Fallback: Parse raw plain text lines
				lines := strings.Split(string(bodyBytes), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						urls = append(urls, line)
					}
				}
			}
		}
	}

	// Also extract any Discord URLs from lines if line contains extra text/markdown
	discordURLPattern := regexp.MustCompile(`https?://(?:cdn\.discordapp\.com|media\.discordapp\.net)/attachments/[^\s"'<>\\]+`)
	var cleanURLs []string
	seen := make(map[string]bool)

	for _, u := range urls {
		matches := discordURLPattern.FindAllString(u, -1)
		if len(matches) > 0 {
			for _, m := range matches {
				if !seen[m] {
					seen[m] = true
					cleanURLs = append(cleanURLs, m)
				}
			}
		} else if strings.TrimSpace(u) != "" && !seen[u] {
			seen[u] = true
			cleanURLs = append(cleanURLs, strings.TrimSpace(u))
		}
	}

	if len(cleanURLs) == 0 {
		http.Error(w, `{"error":"no valid Discord URLs provided"}`, http.StatusBadRequest)
		return
	}

	updatedCount, notFoundCount, err := s.manager.BatchUpdateDiscordURLs(cleanURLs)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"updated":    updatedCount,
		"not_found":  notFoundCount,
		"total_urls": len(cleanURLs),
	})
}
