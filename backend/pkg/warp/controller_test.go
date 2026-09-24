package warp

import (
	"net/http"
	"testing"
)

func TestControllerCustomProxyRotationAndEpoch(t *testing.T) {
	c := NewController(true, 5.0, 40000, "socks5://127.0.0.1:1080, http://127.0.0.1:8080")

	tr := &http.Transport{}
	c.RegisterTransport(tr)

	// Initially proxyActive is false (Direct mode)
	req, _ := http.NewRequest("GET", "https://cdn.discordapp.com/test", nil)
	u, err := c.ProxyFunc(req)
	if err != nil || u != nil {
		t.Fatalf("expected nil proxy in Direct mode, got %v (err=%v)", u, err)
	}

	initialEpoch := c.RotationEpoch()

	// First trigger: switches from Direct -> Proxy #1
	rotated, err := c.TriggerAutoBypassOrRotate("speed dropped below 5MB/s", false)
	if err != nil || !rotated {
		t.Fatalf("expected rotation to succeed, got rotated=%v err=%v", rotated, err)
	}

	if c.RotationEpoch() <= initialEpoch {
		t.Errorf("expected RotationEpoch to increment after rotation")
	}

	u1, err := c.ProxyFunc(req)
	if err != nil || u1 == nil || u1.String() != "socks5://127.0.0.1:1080" {
		t.Errorf("expected socks5://127.0.0.1:1080, got %v", u1)
	}

	// Immediate second trigger without forceImmediate should be skipped by cooldown
	rotated2, _ := c.TriggerAutoBypassOrRotate("duplicate trigger", false)
	if rotated2 {
		t.Errorf("expected cooldown to skip immediate duplicate rotation")
	}

	// Forced rotation -> advances to Proxy #2
	rotated3, err := c.TriggerAutoBypassOrRotate("manual rotate", true)
	if err != nil || !rotated3 {
		t.Fatalf("expected forced rotation to succeed, got %v", err)
	}

	u2, err := c.ProxyFunc(req)
	if err != nil || u2 == nil || u2.String() != "http://127.0.0.1:8080" {
		t.Errorf("expected http://127.0.0.1:8080, got %v", u2)
	}

	// Disable proxy -> back to Direct
	_ = c.SetProxyActive(false)
	u3, _ := c.ProxyFunc(req)
	if u3 != nil {
		t.Errorf("expected nil proxy after disabling, got %v", u3)
	}
}
