package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	shellEnvOnce sync.Once
	shellEnvVal  []string // nil → capture failed, ShellEnv falls back to os.Environ
)

// WarmShellEnv starts a background goroutine that captures the user's login-shell
// environment. Call once at server startup; subsequent calls are no-ops.
// ShellEnv blocks until capture completes, so calling WarmShellEnv early means
// the first agent spawn won't have to wait.
func WarmShellEnv() {
	go shellEnvOnce.Do(captureShellEnv)
}

// ShellEnv returns the merged login-shell environment. It blocks if WarmShellEnv
// has not yet finished, then returns instantly on every subsequent call.
// Falls back to os.Environ if the shell capture fails.
func ShellEnv() []string {
	shellEnvOnce.Do(captureShellEnv)
	if len(shellEnvVal) > 0 {
		return shellEnvVal
	}
	return os.Environ()
}

func captureShellEnv() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name := filepath.Base(shell)
	var raw []byte
	var err error

	slog.Debug("capturing login-shell env", "shell", shell)

	if name == "nu" || name == "nushell" {
		raw, err = runShell(ctx, shell, []string{"-l", "-c", "($env | to json)"})
		if err != nil {
			slog.Warn("shell env capture failed", "shell", shell, "error", err)
			return
		}
		shellEnvVal = mergeWithBase(parseNushellJSON(raw))
	} else {
		// -i makes it an interactive shell so ~/.zshrc / ~/.bashrc are sourced in
		// addition to the login files (~/.zprofile etc.).
		raw, err = runShell(ctx, shell, []string{"-i", "-l", "-c", "env -0"})
		if err != nil {
			// env -0 may not be supported on this system; fall back to newline-separated
			slog.Debug("env -0 failed, retrying with printenv", "shell", shell, "error", err)
			raw, err = runShell(ctx, shell, []string{"-i", "-l", "-c", "printenv"})
			if err != nil {
				slog.Warn("shell env capture failed", "shell", shell, "error", err)
				return
			}
			shellEnvVal = mergeWithBase(parseNewlineSeparated(raw))
		} else {
			shellEnvVal = mergeWithBase(parseNullSeparated(raw))
		}
	}

	if len(shellEnvVal) > 0 {
		var path, httpProxy, httpsProxy string
		for _, kv := range shellEnvVal {
			switch {
			case strings.HasPrefix(kv, "PATH="):
				path = kv[5:]
			case strings.HasPrefix(kv, "http_proxy="):
				httpProxy = kv[11:]
			case strings.HasPrefix(kv, "https_proxy="):
				httpsProxy = kv[12:]
			}
		}
		slog.Info("shell env captured", "shell", shell, "vars", len(shellEnvVal),
			"PATH", path, "http_proxy", httpProxy, "https_proxy", httpsProxy)
	} else {
		slog.Warn("shell env capture produced no vars", "shell", shell, "raw_bytes", len(raw))
	}
}

func runShell(ctx context.Context, shell string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Env = os.Environ()
	// -i makes zsh/bash take the controlling terminal (tcsetpgrp). Keep the
	// shell off that tty so the server stays in the foreground process group.
	cmd.Stderr = io.Discard
	if f, err := os.Open(os.DevNull); err == nil {
		defer f.Close()
		cmd.Stdin = f
	}
	detachShellProcess(cmd)
	return cmd.Output()
}

// mergeWithBase overlays captured vars onto os.Environ so that shell-set vars
// (PATH, http_proxy, tokens, …) win, while any server-process-specific vars
// not present in the shell are preserved.
func mergeWithBase(captured []string) []string {
	if len(captured) == 0 {
		return nil
	}
	m := envToMap(os.Environ())
	for _, kv := range captured {
		i := strings.IndexByte(kv, '=')
		if i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return mapToEnv(m)
}

func parseNullSeparated(data []byte) []string {
	var out []string
	for _, entry := range bytes.Split(data, []byte{0}) {
		s := string(entry)
		if strings.IndexByte(s, '=') > 0 {
			out = append(out, s)
		}
	}
	return out
}

// parseNewlineSeparated is a fallback for systems where env -0 is unavailable.
// It cannot preserve multiline values, but is better than nothing.
func parseNewlineSeparated(data []byte) []string {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.IndexByte(line, '=') > 0 {
			out = append(out, line)
		}
	}
	return out
}

func parseNushellJSON(data []byte) []string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	out := make([]string, 0, len(obj))
	for k, v := range obj {
		switch val := v.(type) {
		case string:
			out = append(out, k+"="+val)
		case []any:
			// PATH and similar list vars are stored as slices in nushell.
			parts := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				out = append(out, k+"="+strings.Join(parts, string(os.PathListSeparator)))
			}
		}
	}
	return out
}
