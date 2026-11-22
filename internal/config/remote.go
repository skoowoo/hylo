package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// applyEnv overlays HYLO_API_KEY, which wins over the config file.
func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("HYLO_API_KEY")); v != "" {
		cfg.Server.APIKey = v
		cfg.Server.APIKeyFromEnv = true
	}
}

// IsLoopbackHost reports whether host only accepts local connections.
// An empty host binds every interface and so is not loopback.
func IsLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// IsRemote reports whether the listener is reachable from other machines.
func (s ServerConfig) IsRemote() bool {
	return s.TCPEnabled() && !IsLoopbackHost(s.Host)
}

// ClientBaseURL returns the origin the local CLI uses to reach this server.
func (s ServerConfig) ClientBaseURL() (string, error) {
	if !s.TCPEnabled() {
		return "", fmt.Errorf(
			"cannot reach hylo server: set server.port (e.g. 54321) in your config.toml",
		)
	}
	host := s.Host
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	scheme := "http"
	if s.TLSEnabled() {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(s.Port))), nil
}

// GenerateAPIKey returns a new random API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "hylo_" + hex.EncodeToString(b), nil
}

// PersistAPIKey writes key to server.api_key in the config file.
func PersistAPIKey(loadedPath, key string) (string, error) {
	path, err := ResolveConfigWritePath(loadedPath)
	if err != nil {
		return "", err
	}
	base, err := MergedFromOptionalFile(path)
	if err != nil {
		return "", err
	}
	m, err := ConfigToMap(base)
	if err != nil {
		return "", err
	}
	srv, _ := m["server"].(map[string]any)
	if srv == nil {
		srv = map[string]any{}
		m["server"] = srv
	}
	srv["api_key"] = key
	if err := WriteConfigToml(path, m); err != nil {
		return "", err
	}
	return path, nil
}

// EnsureAPIKey generates and persists a key when none is configured, so the
// server never runs unauthenticated, local or remote.
func EnsureAPIKey(cfg *Config, loadedPath string) (path string, generated bool, err error) {
	if cfg.Server.APIKey != "" {
		return "", false, nil
	}
	key, err := GenerateAPIKey()
	if err != nil {
		return "", false, err
	}
	path, err = PersistAPIKey(loadedPath, key)
	if err != nil {
		return "", false, fmt.Errorf("persist generated api key: %w", err)
	}
	cfg.Server.APIKey = key
	return path, true, nil
}

// PopulateRelayIdentity fills missing internal Relay credentials in memory when
// Relay is enabled with a URL. It never invents a default Relay URL.
func PopulateRelayIdentity(r *RelayConfig) (bool, error) {
	if !r.Active() {
		return false, nil
	}
	changed := false
	if strings.TrimSpace(r.ServerID) == "" {
		v, err := relaySecret("srv_")
		if err != nil {
			return false, err
		}
		r.ServerID = v
		changed = true
	}
	if strings.TrimSpace(r.ConnectorToken) == "" {
		v, err := relaySecret("rlyc_")
		if err != nil {
			return false, err
		}
		r.ConnectorToken = v
		changed = true
	}
	if strings.TrimSpace(r.AccessToken) == "" {
		v, err := relaySecret("rlya_")
		if err != nil {
			return false, err
		}
		r.AccessToken = v
		changed = true
	}
	return changed, nil
}

// EnsureRelayIdentity generates and persists missing private Relay credentials
// only when Relay is explicitly enabled with a URL.
func EnsureRelayIdentity(cfg *Config, loadedPath string) (string, bool, error) {
	changed, err := PopulateRelayIdentity(&cfg.Relay)
	if err != nil || !changed {
		return "", false, err
	}
	path, err := ResolveConfigWritePath(loadedPath)
	if err != nil {
		return "", false, err
	}
	m, err := ConfigToMap(cfg)
	if err != nil {
		return "", false, err
	}
	if err := WriteConfigToml(path, m); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func relaySecret(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
