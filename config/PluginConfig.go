package config

import "time"

// PluginConfig holds the configuration for the xfinitywifi plugin.
type PluginConfig struct {
	// Username is the Comcast/Xfinity username or email used to authenticate against the captive portal.
	Username string
	// Password is the Comcast/Xfinity password used to authenticate against the captive portal.
	Password string
	// CheckInterval is how often the plugin probes for connectivity.
	CheckInterval time.Duration
	// ProbeTargets is the list of endpoints used to detect a captive portal.
	ProbeTargets []string
	// RequiredSuccesses is the number of probe targets that must succeed for the connection to be considered healthy.
	RequiredSuccesses int
	// TriggerURL is a non-SSL URL used to force the captive portal intercept redirect.
	TriggerURL string
	// LoginURL is the fallback Xfinity login endpoint used when the form action cannot be parsed.
	LoginURL string
}

// DefaultProbeTargets is the list of reliable endpoints used by major OSes to detect captive portals.
var DefaultProbeTargets = []string{
	"http://connectivitycheck.gstatic.com/generate_204", // Returns 204
	"http://detectportal.firefox.com/success.txt",       // Returns 200 "success\n"
	"http://www.apple.com/library/test/success.html",    // Returns 200 with small HTML
}

func NewPluginConfig() PluginConfig {
	targets := make([]string, len(DefaultProbeTargets))
	copy(targets, DefaultProbeTargets)

	return PluginConfig{
		CheckInterval:     30 * time.Second,
		ProbeTargets:      targets,
		RequiredSuccesses: 2,
		TriggerURL:        "http://neverssl.com",
		LoginURL:          "https://login.xfinity.com/login",
	}
}
