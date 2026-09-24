package warp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gdrive-downloader/pkg/logger"
)

// Status represents the current state of the WARP / Anti-Throttle controller for the Web UI.
type Status struct {
	Installed        bool      `json:"installed"`
	BinaryPath       string    `json:"binary_path,omitempty"`
	AutoEnabled      bool      `json:"auto_enabled"`
	ProxyActive      bool      `json:"proxy_active"`
	WarpConnected    bool      `json:"warp_connected"`
	IsRotating       bool      `json:"is_rotating"`
	MinSpeedMB       float64   `json:"min_speed_mb"`
	ProxyPort        int       `json:"proxy_port"`
	CustomProxyURL   string    `json:"custom_proxy_url"`
	ActiveProxyURL   string    `json:"active_proxy_url"`
	RotationCount    int       `json:"rotation_count"`
	RotationEpoch    uint64    `json:"rotation_epoch"`
	LastRotatedAt    time.Time `json:"last_rotated_at,omitempty"`
	LastReason       string    `json:"last_reason,omitempty"`
	LowSpeedDuration int       `json:"low_speed_duration_sec"`
}

// Controller manages Cloudflare WARP CLI (in local SOCKS5 proxy mode) and custom proxy rotation.
type Controller struct {
	mu               sync.RWMutex
	rotateMu         sync.Mutex
	binaryPath       string
	installed        bool
	autoEnabled      bool
	proxyActive      bool
	warpConnected    bool
	isRotating       bool
	minSpeedMB       float64
	proxyPort        int
	customProxyURL   string
	customProxyList  []string
	customProxyIdx   int
	rotationCount    int
	rotationEpoch    uint64
	lastRotatedAt    time.Time
	lastReason       string
	lowSpeedDuration int
	transports       []*http.Transport
}

// NewController initializes the WARP & Anti-Throttle controller.
func NewController(autoEnabled bool, minSpeedMB float64, proxyPort int, customProxyURL string) *Controller {
	if minSpeedMB <= 0 {
		minSpeedMB = 5.0
	}
	if proxyPort <= 0 {
		proxyPort = 40000
	}

	binPath := detectWarpBinary()
	installed := binPath != ""

	c := &Controller{
		binaryPath:     binPath,
		installed:      installed,
		autoEnabled:    autoEnabled,
		minSpeedMB:     minSpeedMB,
		proxyPort:      proxyPort,
		rotationEpoch:  1,
		transports:     make([]*http.Transport, 0),
	}
	c.SetCustomProxyURL(customProxyURL)

	if installed {
		logger.Infof("WARP", "Detected Cloudflare WARP CLI at '%s' (Auto-Bypass: %v, MinSpeed: %.1f MB/s, Port: %d)", binPath, autoEnabled, minSpeedMB, proxyPort)
		go c.refreshDaemonStatus()
	} else {
		logger.Infof("WARP", "Cloudflare WARP CLI not found on PATH (CustomProxy: '%s')", customProxyURL)
	}

	return c
}

func detectWarpBinary() string {
	// 1. Check standard PATH
	if p, err := exec.LookPath("warp-cli"); err == nil && p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("warp-cli.exe"); err == nil && p != "" {
			return p
		}
		candidates := []string{
			`C:\Program Files\Cloudflare\Cloudflare WARP\warp-cli.exe`,
			`C:\Program Files (x86)\Cloudflare\Cloudflare WARP\warp-cli.exe`,
		}
		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				return cand
			}
		}
	} else {
		candidates := []string{
			"/usr/bin/warp-cli",
			"/usr/local/bin/warp-cli",
			"/snap/bin/warp-cli",
		}
		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				return cand
			}
		}
	}
	return ""
}

// RegisterTransport registers an http.Transport so its Proxy function is wired to this Controller
// and its idle connections are flushed upon IP rotation.
func (c *Controller) RegisterTransport(tr *http.Transport) {
	if tr == nil {
		return
	}
	tr.Proxy = c.ProxyFunc
	c.mu.Lock()
	c.transports = append(c.transports, tr)
	c.mu.Unlock()
}

// ProxyFunc is the dynamic http.Transport Proxy callback.
func (c *Controller) ProxyFunc(req *http.Request) (*url.URL, error) {
	c.mu.RLock()
	active := c.proxyActive
	proxyURI := c.getActiveProxyURLLocked()
	c.mu.RUnlock()

	if !active || proxyURI == "" {
		return nil, nil
	}
	return url.Parse(proxyURI)
}

func (c *Controller) getActiveProxyURLLocked() string {
	if len(c.customProxyList) > 0 {
		idx := c.customProxyIdx % len(c.customProxyList)
		return c.customProxyList[idx]
	}
	if c.installed {
		return fmt.Sprintf("socks5://127.0.0.1:%d", c.proxyPort)
	}
	return ""
}

// RotationEpoch returns the current rotation epoch counter (atomic).
// Download readers compare this against their start epoch to abort throttled connections immediately.
func (c *Controller) RotationEpoch() uint64 {
	return atomic.LoadUint64(&c.rotationEpoch)
}

func (c *Controller) bumpEpochAndFlushTransports() uint64 {
	newEpoch := atomic.AddUint64(&c.rotationEpoch, 1)
	c.mu.RLock()
	trs := make([]*http.Transport, len(c.transports))
	copy(trs, c.transports)
	c.mu.RUnlock()

	for _, tr := range trs {
		if tr != nil {
			tr.CloseIdleConnections()
		}
	}
	return newEpoch
}

// UpdateConfig updates the controller's settings at runtime.
func (c *Controller) UpdateConfig(autoEnabled bool, minSpeedMB float64, proxyPort int, customProxyURL string) {
	c.mu.Lock()
	c.autoEnabled = autoEnabled
	if minSpeedMB > 0 {
		c.minSpeedMB = minSpeedMB
	}
	if proxyPort > 0 && proxyPort != c.proxyPort {
		c.proxyPort = proxyPort
	}
	c.parseCustomProxiesLocked(customProxyURL)
	c.mu.Unlock()
}

// SetCustomProxyURL parses a single or comma/newline-separated list of proxy URLs.
func (c *Controller) SetCustomProxyURL(raw string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parseCustomProxiesLocked(raw)
}

func (c *Controller) parseCustomProxiesLocked(raw string) {
	c.customProxyURL = strings.TrimSpace(raw)
	c.customProxyList = nil
	if c.customProxyURL == "" {
		return
	}
	parts := strings.FieldsFunc(c.customProxyURL, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			if !strings.Contains(p, "://") {
				p = "socks5://" + p
			}
			c.customProxyList = append(c.customProxyList, p)
		}
	}
}

// GetStatus returns a snapshot of the current WARP / Proxy status.
func (c *Controller) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	activeURL := ""
	if c.proxyActive {
		activeURL = c.getActiveProxyURLLocked()
	}

	return Status{
		Installed:        c.installed,
		BinaryPath:       c.binaryPath,
		AutoEnabled:      c.autoEnabled,
		ProxyActive:      c.proxyActive,
		WarpConnected:    c.warpConnected,
		IsRotating:       c.isRotating,
		MinSpeedMB:       c.minSpeedMB,
		ProxyPort:        c.proxyPort,
		CustomProxyURL:   c.customProxyURL,
		ActiveProxyURL:   activeURL,
		RotationCount:    c.rotationCount,
		RotationEpoch:    atomic.LoadUint64(&c.rotationEpoch),
		LastRotatedAt:    c.lastRotatedAt,
		LastReason:       c.lastReason,
		LowSpeedDuration: c.lowSpeedDuration,
	}
}

// SetLowSpeedDuration updates the current low-speed watchdog counter (for UI visibility).
func (c *Controller) SetLowSpeedDuration(sec int) {
	c.mu.Lock()
	c.lowSpeedDuration = sec
	c.mu.Unlock()
}

// IsAutoEnabled returns whether automatic speed-based WARP rotation is enabled.
func (c *Controller) IsAutoEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.autoEnabled && (c.installed || len(c.customProxyList) > 0)
}

// MinSpeedBytesPerSec returns the minimum speed threshold in bytes/second.
func (c *Controller) MinSpeedBytesPerSec() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return int64(c.minSpeedMB * 1024 * 1024)
}

// SetProxyActive manually enables or disables the WARP/Custom proxy.
func (c *Controller) SetProxyActive(enable bool) error {
	if enable {
		return c.enableProxy("Manual activation from UI")
	}
	return c.disableProxy()
}

func (c *Controller) enableProxy(reason string) error {
	c.rotateMu.Lock()
	defer c.rotateMu.Unlock()

	c.mu.Lock()
	hasCustom := len(c.customProxyList) > 0
	installed := c.installed
	port := c.proxyPort
	c.isRotating = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.isRotating = false
		c.mu.Unlock()
	}()

	if hasCustom {
		c.mu.Lock()
		c.proxyActive = true
		c.lastReason = reason
		c.lastRotatedAt = time.Now()
		c.mu.Unlock()
		c.bumpEpochAndFlushTransports()
		logger.Infof("WARP", "Activated custom proxy (%s): %s", c.GetStatus().ActiveProxyURL, reason)
		return nil
	}

	if !installed {
		return fmt.Errorf("warp-cli is not installed and no custom proxy is configured")
	}

	if err := c.ensureWarpProxyConnected(port); err != nil {
		return err
	}

	c.mu.Lock()
	c.proxyActive = true
	c.warpConnected = true
	c.lastReason = reason
	c.lastRotatedAt = time.Now()
	c.mu.Unlock()

	c.bumpEpochAndFlushTransports()
	logger.Successf("WARP", "Cloudflare WARP SOCKS5 Proxy active on 127.0.0.1:%d (%s)", port, reason)
	return nil
}

func (c *Controller) disableProxy() error {
	c.rotateMu.Lock()
	defer c.rotateMu.Unlock()

	c.mu.Lock()
	c.proxyActive = false
	c.warpConnected = false
	c.lastReason = "Switched to Direct Mode"
	installed := c.installed
	c.mu.Unlock()

	if installed {
		_, _ = c.runWarpCmd("disconnect")
	}

	c.bumpEpochAndFlushTransports()
	logger.Infof("WARP", "Disabled WARP proxy; switched back to Direct connection")
	return nil
}

// TriggerAutoBypassOrRotate is called by the speed watchdog or HTTP 429 handler.
// It obeys a minimum cooldown (18 seconds) so multiple workers don't trigger overlapping rotations.
func (c *Controller) TriggerAutoBypassOrRotate(reason string, forceImmediate bool) (bool, error) {
	if !c.rotateMu.TryLock() {
		// Another rotation is already in progress
		return false, nil
	}
	defer c.rotateMu.Unlock()

	c.mu.RLock()
	lastRot := c.lastRotatedAt
	wasActive := c.proxyActive
	hasCustom := len(c.customProxyList) > 0
	installed := c.installed
	port := c.proxyPort
	c.mu.RUnlock()

	if !forceImmediate && !lastRot.IsZero() && time.Since(lastRot) < 18*time.Second {
		return false, nil
	}

	if !installed && !hasCustom {
		return false, fmt.Errorf("neither warp-cli nor custom proxy is available")
	}

	c.mu.Lock()
	c.isRotating = true
	c.lowSpeedDuration = 0
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.isRotating = false
		c.mu.Unlock()
	}()

	// Case 1: Custom Proxy Pool Rotation
	if hasCustom {
		c.mu.Lock()
		if wasActive && len(c.customProxyList) > 1 {
			c.customProxyIdx = (c.customProxyIdx + 1) % len(c.customProxyList)
		}
		c.proxyActive = true
		c.rotationCount++
		c.lastRotatedAt = time.Now()
		c.lastReason = reason
		activeProxy := c.getActiveProxyURLLocked()
		rotNum := c.rotationCount
		c.mu.Unlock()

		c.bumpEpochAndFlushTransports()
		logger.Warnf("WARP", "[Auto-Bypass #%d] Switched to proxy '%s' (Reason: %s)", rotNum, activeProxy, reason)
		return true, nil
	}

	// Case 2: Cloudflare WARP CLI
	if !wasActive {
		// First time hitting limit -> Switch from Direct to WARP Proxy!
		logger.Warnf("WARP", "[Auto-Bypass] Speed/Rate limit detected (%s). Activating Cloudflare WARP Proxy on 127.0.0.1:%d...", reason, port)
		if err := c.ensureWarpProxyConnected(port); err != nil {
			logger.Errorf("WARP", "Failed to activate WARP proxy: %v", err)
			return false, err
		}
	} else {
		// Already on WARP Proxy and hit rate limit again -> Rotate WARP keys & reconnect for fresh Cloudflare IP!
		logger.Warnf("WARP", "[Auto-Bypass] Rate limit detected on current WARP IP (%s). Rotating WARP keys & reconnecting...", reason)
		_, _ = c.runWarpCmd("disconnect")
		time.Sleep(300 * time.Millisecond)
		_, _ = c.runWarpCmd("tunnel", "rotate-keys")
		time.Sleep(300 * time.Millisecond)
		if err := c.ensureWarpProxyConnected(port); err != nil {
			logger.Errorf("WARP", "Failed to reconnect WARP proxy after key rotation: %v. Falling back to Direct mode temporarily.", err)
			c.mu.Lock()
			c.proxyActive = false
			c.warpConnected = false
			c.lastRotatedAt = time.Now()
			c.lastReason = "Fallback to Direct (WARP reconnect failed)"
			c.mu.Unlock()
			c.bumpEpochAndFlushTransports()
			return true, nil
		}
	}

	c.mu.Lock()
	c.proxyActive = true
	c.warpConnected = true
	c.rotationCount++
	c.lastRotatedAt = time.Now()
	c.lastReason = reason
	rotNum := c.rotationCount
	c.mu.Unlock()

	c.bumpEpochAndFlushTransports()
	logger.Successf("WARP", "[Auto-Bypass #%d] Fresh WARP IP ready on socks5://127.0.0.1:%d! Reconnecting active streams...", rotNum, port)
	return true, nil
}

func (c *Controller) ensureWarpProxyConnected(port int) error {
	// Configure proxy mode and port
	_, _ = c.runWarpCmd("mode", "proxy")
	_, _ = c.runWarpCmd("proxy", "port", fmt.Sprintf("%d", port))

	if _, err := c.runWarpCmd("connect"); err != nil {
		return fmt.Errorf("warp-cli connect failed: %w", err)
	}

	// Wait up to 6 seconds for local SOCKS5 port to accept connections
	deadline := time.Now().Add(6 * time.Second)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for WARP SOCKS5 proxy at %s", addr)
}

func (c *Controller) refreshDaemonStatus() {
	out, err := c.runWarpCmd("status")
	if err != nil {
		return
	}
	connected := strings.Contains(strings.ToLower(out), "connected") && !strings.Contains(strings.ToLower(out), "disconnected")
	c.mu.Lock()
	c.warpConnected = connected
	c.mu.Unlock()
}

func (c *Controller) runWarpCmd(args ...string) (string, error) {
	c.mu.RLock()
	bin := c.binaryPath
	c.mu.RUnlock()

	if bin == "" {
		return "", fmt.Errorf("warp-cli binary not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	fullArgs := append([]string{"--accept-tos"}, args...)
	cmd := exec.CommandContext(ctx, bin, fullArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
