package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hardhacker/hylo/internal/agent/installer"
)

// userToolchainDirs appends common user-level bin locations so agent binaries
// are found when the HTTP server is launched with a minimal PATH (mirrors
// agents.ts userToolchainDirs + wellKnownUserToolchainBins intent).
func userToolchainDirs(home string) []string {
	if home == "" {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, d := range installer.AllBinDirs(home) {
		add(d)
	}
	add(filepath.Join(home, ".local", "bin"))
	add(filepath.Join(home, "bin"))
	if runtime.GOOS == "darwin" {
		add("/opt/homebrew/bin")
		add("/usr/local/bin")
	}
	if runtime.GOOS == "linux" {
		add(filepath.Join(home, ".npm-global", "bin"))
		add("/usr/local/bin")
	}
	return out
}

// ResolvePathDirs merges PATH with toolchain dirs (deduped, order preserved).
func ResolvePathDirs() []string {
	pathEnv := os.Getenv("PATH")
	parts := filepath.SplitList(pathEnv)
	home, _ := os.UserHomeDir()
	if ov := os.Getenv("OD_AGENT_HOME"); ov != "" {
		home = ov
	}
	var merged []string
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		merged = append(merged, p)
	}
	for _, p := range userToolchainDirs(home) {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		merged = append(merged, p)
	}
	return merged
}

// ResolveOnPath finds an executable named bin in PATH + toolchain dirs.
func ResolveOnPath(bin string) string {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		pat := os.Getenv("PATHEXT")
		if strings.TrimSpace(pat) == "" {
			pat = ".EXE;.CMD;.BAT"
		}
		exts = strings.Split(strings.ToLower(pat), ";")
		for i := range exts {
			exts[i] = strings.TrimSpace(exts[i])
		}
	}
	for _, dir := range ResolvePathDirs() {
		for _, ext := range exts {
			full := filepath.Join(dir, bin+ext)
			if st, err := os.Stat(full); err == nil && !st.IsDir() {
				return full
			}
		}
	}
	return ""
}
