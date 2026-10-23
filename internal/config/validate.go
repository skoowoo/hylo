package config

import (
	"fmt"
	"strings"
	"time"
)

// Validate checks merged configuration for obvious invalid values.
func Validate(cfg *Config) error {
	var errs []string
	add := func(key, msg string) {
		errs = append(errs, key+": "+msg)
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Log.Level)) {
	case "", "debug", "info", "warn", "error":
	default:
		add("log.level", fmt.Sprintf("must be one of debug, info, warn, error; got %q", cfg.Log.Level))
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Log.Format)) {
	case "", "text", "json":
	default:
		add("log.format", fmt.Sprintf("must be text or json; got %q", cfg.Log.Format))
	}

	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		add("server.port", "must be between 0 and 65535 inclusive")
	}
	if cfg.Server.ReadTimeout < 0 {
		add("server.read_timeout", "must be >= 0")
	}
	if cfg.Server.WriteTimeout < 0 {
		add("server.write_timeout", "must be >= 0")
	}
	if _, err := ParseProxy(cfg.Server.Proxy); err != nil {
		add("server.proxy", err.Error())
	}

	tlsWant := cfg.Server.CertFile != "" || cfg.Server.KeyFile != ""
	if tlsWant && (cfg.Server.CertFile == "" || cfg.Server.KeyFile == "") {
		add("server.tls", "set both cert_file and key_file to enable TLS, or leave both empty")
	}

	g := cfg.Plugins.GitSync
	if g.Debounce != "" {
		if _, err := time.ParseDuration(g.Debounce); err != nil {
			add("plugins.git_sync.debounce", err.Error())
		}
	}
	if g.SyncInterval != "" && g.SyncInterval != "0" {
		if _, err := time.ParseDuration(g.SyncInterval); err != nil {
			add("plugins.git_sync.sync_interval", err.Error())
		}
	}

	if len(errs) != 0 {
		return ValidationError{Errors: errs}
	}
	return nil
}

// ValidationError aggregates field-level validation messages.
type ValidationError struct {
	Errors []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Errors, "; ")
}

// validateHHMM checks "HH:MM" local clock values (hours 0–23, minutes 0–59).
func validateHHMM(s string) error {
	if s == "" {
		return nil
	}
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return fmt.Errorf("must be HH:MM")
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return fmt.Errorf("hour must be 0–23 and minute 0–59")
	}
	return nil
}
