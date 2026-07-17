# pmaas-plugin-xfinitywifi

A PMAAS plugin that keeps a host authenticated against an `xfinitywifi` captive portal.

The plugin periodically probes well-known connectivity-check endpoints to detect a captive portal. When the
connection is hijacked or down, it drives the multi-step Xfinity login flow (trigger redirect, parse the login
form, submit credentials) and re-verifies connectivity.

## Status page

The plugin serves a status page at `/plugins/xfinitywifi/` that shows:

- **Current status** — connectivity, captive-portal detection, monitor state, check counts and re-auth totals.
- **Last probe attempt** — timestamp, result, and per-target outcomes.
- **Last login attempt** — timestamp, result, resolved action URL, and any message/error.

## Usage

```go
conf := xfinitywifi.NewPluginConfig()
conf.Username = "your-xfinity-username-or-email"
conf.Password = "your-xfinity-password"
// Optional overrides:
// conf.CheckInterval = 30 * time.Second
// conf.RequiredSuccesses = 2
// conf.ProbeTargets = config.DefaultProbeTargets

coreConfig.AddPlugin(xfinitywifi.NewPlugin(conf), config.PluginConfig{})
```
