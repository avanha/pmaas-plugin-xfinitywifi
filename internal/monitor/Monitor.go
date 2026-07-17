package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/avanha/pmaas-plugin-xfinitywifi/config"
	"github.com/avanha/pmaas-plugin-xfinitywifi/data"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/common"
)

// Monitor probes for connectivity and, when a captive portal is detected, attempts to authenticate against the
// Xfinity portal.  It keeps track of the current status and the most recent probe and login attempts.  All
// accessible state is guarded by a mutex so that it can be safely read from HTTP request goroutines.
type Monitor struct {
	pluginConfig config.PluginConfig

	mu                   sync.Mutex
	running              bool
	connected            bool
	captivePortal        bool
	lastCheckTime        time.Time
	totalChecks          int
	totalConnectedChecks int
	totalReauthAttempts  int
	totalReauthSuccesses int
	lastErrorMessage     string
	lastErrorTime        time.Time
	lastProbe            *data.ProbeAttempt
	lastLogin            *data.LoginAttempt
}

func NewMonitor(pluginConfig config.PluginConfig) *Monitor {
	return &Monitor{pluginConfig: pluginConfig}
}

// SetRunning updates the running flag reported on the status page.
func (m *Monitor) SetRunning(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = running
}

// Run drives the periodic connectivity checks until the context is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	// Run an immediate check on startup.
	m.checkAndReauth()

	ticker := time.NewTicker(m.pluginConfig.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAndReauth()
		}
	}
}

func (m *Monitor) checkAndReauth() {
	if !m.checkConnectivity() {
		m.handleReauth()
	}
}

// checkConnectivity probes multiple targets, records the attempt, and returns true if we have open internet.
func (m *Monitor) checkConnectivity() bool {
	// Create a client that DOES NOT follow redirects so we can detect captive portals.
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	attempt := &data.ProbeAttempt{
		Time:              time.Now(),
		RequiredSuccesses: m.pluginConfig.RequiredSuccesses,
		Targets:           make([]data.ProbeTargetResult, 0, len(m.pluginConfig.ProbeTargets)),
	}

	successes := 0
	captivePortal := false

	for _, target := range m.pluginConfig.ProbeTargets {
		result := data.ProbeTargetResult{Target: target}

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			cancel()
			result.Error = err.Error()
			attempt.Targets = append(attempt.Targets, result)
			continue
		}

		resp, err := client.Do(req)
		cancel()
		if err != nil {
			// Network error (no IP, DNS resolution failed, etc.).
			result.Error = err.Error()
			attempt.Targets = append(attempt.Targets, result)
			continue
		}
		resp.Body.Close()

		result.StatusCode = resp.StatusCode

		// Captive portals hijack requests with a 302/307 redirect, or rewrite the body with a 200 OK
		// containing their login page.
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			result.Redirected = true
			captivePortal = true
		} else if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
			result.Success = true
			successes++
		}

		attempt.Targets = append(attempt.Targets, result)
	}

	connected := successes >= m.pluginConfig.RequiredSuccesses
	attempt.SuccessCount = successes
	attempt.Connected = connected
	attempt.CaptivePortalDetected = captivePortal

	m.recordProbe(attempt, connected, captivePortal)

	return connected
}

func (m *Monitor) recordProbe(attempt *data.ProbeAttempt, connected, captivePortal bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastProbe = attempt
	m.lastCheckTime = attempt.Time
	m.connected = connected
	m.captivePortal = captivePortal
	m.totalChecks++
	if connected {
		m.totalConnectedChecks++
	}
}

// handleReauth coordinates the multi-step Comcast login sequence.
func (m *Monitor) handleReauth() {
	attempt := &data.LoginAttempt{Time: time.Now()}

	m.mu.Lock()
	m.totalReauthAttempts++
	m.mu.Unlock()

	// Create a cookie jar to persist session cookies across the auth flow.
	jar, err := cookiejar.New(nil)
	if err != nil {
		m.recordLogin(attempt, false, fmt.Sprintf("failed to create cookie jar: %v", err))
		return
	}

	client := &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // captive portals use self-signed certs
		},
	}

	// Step 1: Hit a non-SSL target to force the captive portal intercept redirect.
	resp, err := client.Get(m.pluginConfig.TriggerURL)
	if err != nil {
		m.recordLogin(attempt, false, fmt.Sprintf("failed to hit trigger URL: %v", err))
		return
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	bodyStr := string(bodyBytes)

	// Step 2: Parse necessary hidden tokens from the landing page.
	actionURL, payload, err := m.parseLoginPage(bodyStr)
	if err != nil {
		m.recordLogin(attempt, false, fmt.Sprintf("failed to parse login page elements: %v", err))
		return
	}
	attempt.ActionURL = actionURL

	// Add the actual credentials to the parsed payload.
	payload.Set("username", m.pluginConfig.Username)
	payload.Set("password", m.pluginConfig.Password)

	// Step 3: POST the authentication payload back to the portal.
	req, err := http.NewRequest(http.MethodPost, actionURL, strings.NewReader(payload.Encode()))
	if err != nil {
		m.recordLogin(attempt, false, fmt.Sprintf("failed to create login request: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	authResp, err := client.Do(req)
	if err != nil {
		m.recordLogin(attempt, false, fmt.Sprintf("authentication post failed: %v", err))
		return
	}
	authResp.Body.Close()

	// Step 4: Verify connection recovery.
	time.Sleep(3 * time.Second) // Let DHCP/routing settle if necessary.
	if m.checkConnectivity() {
		m.recordLoginSuccess(attempt, "portal login successful; internet access restored")
	} else {
		m.recordLogin(attempt, false, "auth submitted, but keep-alive checks are still failing")
	}
}

func (m *Monitor) recordLogin(attempt *data.LoginAttempt, success bool, message string) {
	attempt.Success = success
	if success {
		attempt.Message = message
	} else {
		attempt.Error = message
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastLogin = attempt
	if !success {
		m.lastErrorMessage = message
		m.lastErrorTime = attempt.Time
	}
}

func (m *Monitor) recordLoginSuccess(attempt *data.LoginAttempt, message string) {
	attempt.Success = true
	attempt.Message = message

	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastLogin = attempt
	m.totalReauthSuccesses++
}

// parseLoginPage extracts the target form action and hidden fields from the portal landing page.
func (m *Monitor) parseLoginPage(html string) (string, url.Values, error) {
	payload := url.Values{}

	var actionURL string
	if strings.Contains(html, "action=\"") {
		// Scrape the form destination.
		parts := strings.Split(html, "action=\"")
		if len(parts) > 1 {
			actionURL = strings.Split(parts[1], "\"")[0]
		}
	}

	if actionURL == "" {
		// Fallback to the standard Comcast entry point if the parser misses it.
		actionURL = m.pluginConfig.LoginURL
	}

	// Example parsing of standard Spring Security 'execution' tokens.
	if strings.Contains(html, "name=\"execution\"") {
		parts := strings.Split(html, "name=\"execution\"")
		if len(parts) > 1 {
			valParts := strings.Split(parts[1], "value=\"")
			if len(valParts) > 1 {
				executionVal := strings.Split(valParts[1], "\"")[0]
				payload.Set("execution", executionVal)
			}
		}
	}

	if actionURL == "" {
		return "", nil, fmt.Errorf("could not isolate form action URL")
	}

	return actionURL, payload, nil
}

// Snapshot returns a consistent, point-in-time view of the plugin status and recent attempts.
func (m *Monitor) Snapshot() common.StatusAndEntities {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := common.StatusAndEntities{
		Status: data.PluginStatus{
			Running:               m.running,
			Connected:             m.connected,
			CaptivePortalDetected: m.captivePortal,
			Username:              m.pluginConfig.Username,
			CheckInterval:         m.pluginConfig.CheckInterval,
			LastCheckTime:         m.lastCheckTime,
			TotalChecks:           m.totalChecks,
			TotalConnectedChecks:  m.totalConnectedChecks,
			TotalReauthAttempts:   m.totalReauthAttempts,
			TotalReauthSuccesses:  m.totalReauthSuccesses,
			LastErrorMessage:      m.lastErrorMessage,
			LastErrorTime:         m.lastErrorTime,
		},
	}

	if m.lastProbe != nil {
		probeCopy := *m.lastProbe
		result.LastProbe = &probeCopy
	}

	if m.lastLogin != nil {
		loginCopy := *m.lastLogin
		result.LastLogin = &loginCopy
	}

	return result
}
