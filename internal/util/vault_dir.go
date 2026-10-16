package util

import (
	"fmt"
	"path"
	"strings"
)

// CleanVaultDir normalizes a vault directory such as /_news/; empty means the vault root.
func CleanVaultDir(dir string) (string, error) {
	if dir == "" {
		return "/", nil
	}
	if !strings.HasPrefix(dir, "/") {
		return "", fmt.Errorf("dir %q must be vault-absolute, starting with /", dir)
	}
	for _, seg := range strings.Split(dir, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("dir %q must not contain hidden or relative segments", dir)
		}
	}
	return path.Clean(dir), nil
}
