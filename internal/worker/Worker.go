package worker

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

// Worker probes for connectivity and, when a captive portal is detected, attempts to authenticate against the
// Xfinity portal.  It keeps track of the current status and the most recent probe and login attempts.  All
// accessible state is guarded by a mutex so that it can be safely read from HTTP request goroutines.
type Worker struct {
	pluginConfig         config.PluginConfig
	statsTracker         common.StatsTracker
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

func NewWorker(pluginConfig config.PluginConfig, statsTracker common.StatsTracker) *Worker {
	return &Worker{pluginConfig: pluginConfig, statsTracker: statsTracker}
}

// Run makes periodic connectivity checks until the context is cancelled.
func (w *Worker) Run(ctx context.Context) {
	w.running = true
	// Run an immediate check on startup.
	w.checkAndReauth()

	ticker := time.NewTicker(w.pluginConfig.CheckInterval)
	defer ticker.Stop()

	for run := true; run; {
		select {
		case <-ctx.Done():
			run = false
			break
		case <-ticker.C:
			w.checkAndReauth()
		}
	}

	w.running = false
}

func (w *Worker) checkAndReauth() {
	if !w.checkConnectivity() {
		//w.handleReauth()
		fmt.Printf("%T Connectivity appear down\n", w)
	}
}

// checkConnectivity probes multiple targets, records the attempt, and returns true if we have open internet.
func (w *Worker) checkConnectivity() bool {
	// Create a client that DOES NOT follow redirects so we can detect captive portals.
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	attempt := &data.ProbeAttempt{
		Time:              time.Now(),
		RequiredSuccesses: w.pluginConfig.RequiredSuccesses,
		Targets:           make([]data.ProbeTargetResult, 0, len(w.pluginConfig.ProbeTargets)),
	}

	successes := 0
	captivePortal := false

	for _, target := range w.pluginConfig.ProbeTargets {
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

		err = resp.Body.Close()

		if err != nil {
			fmt.Printf("Error closing connection: %v\n", err)
		}

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

	connected := successes >= w.pluginConfig.RequiredSuccesses
	attempt.SuccessCount = successes
	attempt.Connected = connected
	attempt.CaptivePortalDetected = captivePortal

	w.recordProbeAttempt(attempt, connected, captivePortal)

	return connected
}

func (w *Worker) recordProbeAttempt(attempt *data.ProbeAttempt, connected, captivePortal bool) {
	_, err := w.statsTracker.TrackProbeAttempt(attempt, connected, captivePortal)

	if err != nil {
		fmt.Println("Error updating stats: ", err)
	}
}

// handleReauth coordinates the multi-step Comcast login sequence.
func (w *Worker) handleReauth() {
	attempt := &data.LoginAttempt{Time: time.Now()}

	w.mu.Lock()
	w.totalReauthAttempts++
	w.mu.Unlock()

	// Create a cookie jar to persist session cookies across the auth flow.
	jar, err := cookiejar.New(nil)
	if err != nil {
		w.recordLogin(attempt, false, fmt.Sprintf("failed to create cookie jar: %v", err))
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
	resp, err := client.Get(w.pluginConfig.TriggerURL)
	if err != nil {
		w.recordLogin(attempt, false, fmt.Sprintf("failed to hit trigger URL: %v", err))
		return
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	bodyStr := string(bodyBytes)

	// Step 2: Parse necessary hidden tokens from the landing page.
	actionURL, payload, err := w.parseLoginPage(bodyStr)
	if err != nil {
		w.recordLogin(attempt, false, fmt.Sprintf("failed to parse login page elements: %v", err))
		return
	}
	attempt.ActionURL = actionURL

	// Add the actual credentials to the parsed payload.
	payload.Set("username", w.pluginConfig.Username)
	payload.Set("password", w.pluginConfig.Password)

	// Step 3: POST the authentication payload back to the portal.
	req, err := http.NewRequest(http.MethodPost, actionURL, strings.NewReader(payload.Encode()))
	if err != nil {
		w.recordLogin(attempt, false, fmt.Sprintf("failed to create login request: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	authResp, err := client.Do(req)
	if err != nil {
		w.recordLogin(attempt, false, fmt.Sprintf("authentication post failed: %v", err))
		return
	}

	err = authResp.Body.Close()
	if err != nil {
		fmt.Printf("Error closing connection: %v\n", err)
	}

	// Step 4: Verify connection recovery.
	time.Sleep(3 * time.Second) // Let DHCP/routing settle if necessary.
	if w.checkConnectivity() {
		w.recordLoginSuccess(attempt, "portal login successful; internet access restored")
	} else {
		w.recordLogin(attempt, false, "auth submitted, but keep-alive checks are still failing")
	}
}

func (w *Worker) recordLogin(attempt *data.LoginAttempt, success bool, message string) {
	attempt.Success = success
	if success {
		attempt.Message = message
	} else {
		attempt.Error = message
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastLogin = attempt
	if !success {
		w.lastErrorMessage = message
		w.lastErrorTime = attempt.Time
	}
}

func (w *Worker) recordLoginSuccess(attempt *data.LoginAttempt, message string) {
	attempt.Success = true
	attempt.Message = message

	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastLogin = attempt
	w.totalReauthSuccesses++
}

// parseLoginPage extracts the target form action and hidden fields from the portal landing page.
func (w *Worker) parseLoginPage(html string) (string, url.Values, error) {
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
		actionURL = w.pluginConfig.LoginURL
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
