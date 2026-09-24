package warp

import (
	"fmt"
	"net/http"
	"net/url"
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
		binaryPath:    binPath,
		installed:     installed,
		autoEnabled:   autoEnabled,
		minSpeedMB:    minSpeedMB,
		proxyPort:     proxyPort,
		rotationEpoch: 1,
		transports:    make([]*http.Transport, 0),
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
