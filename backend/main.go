package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gdrive-downloader/pkg/auth"
	"gdrive-downloader/pkg/queue"
	"gdrive-downloader/pkg/server"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func findWebDist(customDir string) string {
	if customDir != "" {
		if info, err := os.Stat(customDir); err == nil && info.IsDir() {
			return customDir
		}
	}
	candidates := []string{
		"dist",
		filepath.Join("..", "frontend", "dist"),
		filepath.Join("frontend", "dist"),
		filepath.Join(".", "dist"),
		"/app/frontend/dist",
		"/app/dist",
	}
	for _, c := range candidates {
		indexFile := filepath.Join(c, "index.html")
		if _, err := os.Stat(indexFile); err == nil {
			return c
		}
	}
	return filepath.Join("..", "frontend", "dist")
}

func main() {
	// Parse CLI flags with fallback to Environment Variables
	defaultPort := getEnvInt("PORT", 8080)
	defaultHost := getEnv("HOST", "0.0.0.0")
	defaultConfigDir := getEnv("CONFIG_DIR", ".")
	defaultDownloadDir := getEnv("DOWNLOAD_DIR", "")
	defaultWebDir := getEnv("WEB_DIR", "")
	defaultAuthEnabled := getEnv("AUTH_ENABLED", "")
	defaultAuthUser := getEnv("AUTH_USER", "")
	defaultAuthPass := getEnv("AUTH_PASS", "")

	portFlag := flag.Int("port", defaultPort, "Port to listen on (or PORT env)")
	hostFlag := flag.String("host", defaultHost, "Host / IP to bind to (or HOST env)")
	dirFlag := flag.String("dir", defaultDownloadDir, "Default download target folder (or DOWNLOAD_DIR env)")
	configFlag := flag.String("config", defaultConfigDir, "Directory to store config.json and downloads.json (or CONFIG_DIR env)")
	webFlag := flag.String("web", defaultWebDir, "Path to frontend dist directory (or WEB_DIR env)")
	authFlag := flag.String("auth", defaultAuthEnabled, "Enable or disable Web UI authentication: 'true' or 'false' (or AUTH_ENABLED env)")
	userFlag := flag.String("user", defaultAuthUser, "Initial Web UI username (or AUTH_USER env)")
	passFlag := flag.String("pass", defaultAuthPass, "Initial Web UI password (or AUTH_PASS env)")
	flag.Parse()

	// Resolve directories
	configDir := *configFlag
	if configDir != "." && configDir != "" {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			log.Printf("Warning: could not create config dir %s: %v", configDir, err)
		}
	}

	configFile := filepath.Join(configDir, "config.json")
	dataFile := filepath.Join(configDir, "downloads.json")

	// Determine default download folder
	downloadFolder := *dirFlag
	if downloadFolder == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			downloadFolder = filepath.Join(userHome, "Downloads")
		} else {
			downloadFolder = "."
		}
	}

	distPath := findWebDist(*webFlag)

	// Initialize Auth & Config Manager
	authMgr, err := auth.NewManager(configFile, downloadFolder, 2)
	if err != nil {
		log.Fatalf("Failed to initialize auth manager: %v", err)
	}

	// Apply bootstrap flags / env vars for headless servers & containers
	if *authFlag != "" {
		enabled := strings.ToLower(*authFlag) == "true" || *authFlag == "1"
		_ = authMgr.SetAuthEnabled(enabled)
	}
	if *userFlag != "" || *passFlag != "" {
		_ = authMgr.SetInitialCredentials(*userFlag, *passFlag)
	}

	cfg := authMgr.GetConfig()

	// Initialize Queue Manager with persisted config & persistent downloads.json
	mgr, err := queue.NewManager(cfg.DownloadFolder, cfg.MaxConcurrency, dataFile)
	if err != nil {
		log.Fatalf("Failed to initialize queue manager: %v", err)
	}

	// Restore Google cookie if saved
	if cfg.GoogleCookie != "" {
		mgr.Downloader().SetGoogleCookie(cfg.GoogleCookie)
	}

	srv := server.NewServer(mgr, authMgr, distPath)

	listenAddr := fmt.Sprintf("%s:%d", *hostFlag, *portFlag)
	httpServer := &http.Server{
		Addr:         listenAddr,
		Handler:      srv.Routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No write timeout for SSE streams
	}

	// Graceful shutdown channel
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Println("==================================================================")
		fmt.Println("   Google Drive Downloader Server (High-Performance Daemon)       ")
		fmt.Println("==================================================================")
		fmt.Printf("  Network Address:      http://%s\n", listenAddr)
		if *hostFlag == "0.0.0.0" || *hostFlag == "" {
			fmt.Printf("  Local Access:         http://localhost:%d\n", *portFlag)
		}
		fmt.Println("  Web UI Directory:    ", distPath)
		fmt.Println("  Config & State:      ", configDir)
		fmt.Println("  Download Folder:     ", cfg.DownloadFolder)
		fmt.Println("  Max Concurrency:     ", cfg.MaxConcurrency)
		if cfg.AuthEnabled {
			fmt.Println("  Web UI Auth:          Enabled (qBittorrent style)")
			fmt.Println("  Admin User:          ", cfg.Username)
		} else {
			fmt.Println("  Web UI Auth:          Disabled (Public / Local network)")
		}
		fmt.Println("==================================================================")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	fmt.Println("Server exited cleanly.")
}
