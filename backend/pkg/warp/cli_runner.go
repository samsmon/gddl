package warp

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gdrive-downloader/pkg/logger"
)

func detectWarpBinary() string {
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

	if !wasActive {
		logger.Warnf("WARP", "[Auto-Bypass] Speed/Rate limit detected (%s). Activating Cloudflare WARP Proxy on 127.0.0.1:%d...", reason, port)
		if err := c.ensureWarpProxyConnected(port); err != nil {
			logger.Errorf("WARP", "Failed to activate WARP proxy: %v", err)
			return false, err
		}
	} else {
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
	regStatus, _ := c.runWarpCmd("registration", "show")
	if strings.Contains(strings.ToLower(regStatus), "missing") || strings.Contains(strings.ToLower(regStatus), "error") || strings.Contains(strings.ToLower(regStatus), "not registered") {
		_, _ = c.runWarpCmd("registration", "new")
		_, _ = c.runWarpCmd("register")
	}

	_, _ = c.runWarpCmd("mode", "proxy")
	_, _ = c.runWarpCmd("proxy", "port", fmt.Sprintf("%d", port))

	if _, err := c.runWarpCmd("connect"); err != nil {
		_, _ = c.runWarpCmd("registration", "new")
		_, _ = c.runWarpCmd("register")
		if _, err2 := c.runWarpCmd("connect"); err2 != nil {
			return fmt.Errorf("warp-cli connect failed: %w", err)
		}
	}

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
