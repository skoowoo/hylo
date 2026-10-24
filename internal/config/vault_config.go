package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
)

// vaultKeys describe one vault rather than the install, so
// <vault>/.hylo/config.toml overrides them and the settings UI writes them there.
var vaultKeys = []string{
	"plugins.search.exclude_prefixes",
	"plugins.git_sync.enabled",
	"plugins.git_sync.remote",
	"plugins.git_sync.branch",
}

// IsVaultKey reports whether the dotted key belongs in the vault's config.
func IsVaultKey(key string) bool {
	for _, k := range vaultKeys {
		if k == key {
			return true
		}
	}
	return false
}

// VaultRoot resolves vault.path the way storage.New does.
func VaultRoot(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Abs(path)
}

// VaultConfigPath is where a vault keeps its own settings.
func VaultConfigPath(root string) string {
	return filepath.Join(root, ".hylo", "config.toml")
}

// applyVaultConfig overlays the vault's config.toml onto cfg.
func applyVaultConfig(cfg *Config, globalPath string) error {
	root, err := VaultRoot(cfg.Vault.Path)
	if err != nil {
		return fmt.Errorf("resolve vault path: %w", err)
	}
	legacy := cfg.Plugins.GitSync
	clearRepoKeys(cfg)

	raw, err := ReadTomlMap(VaultConfigPath(root))
	if err != nil {
		return fmt.Errorf("reading vault config: %w", err)
	}
	m := pickKeys(raw, true)
	if globalPath != "" {
		if moved := migrateGitSync(root, globalPath, legacy); moved != nil {
			m = moved
		}
	}
	return decodeInto(cfg, m)
}

// clearRepoKeys drops git sync's repository keys, which only a vault's config may set.
func clearRepoKeys(cfg *Config) {
	cfg.Plugins.GitSync.Enabled = false
	cfg.Plugins.GitSync.Remote = ""
	cfg.Plugins.GitSync.Branch = ""
}

// WriteVaultConfig merges the vault keys of patch into the vault's config.toml.
func WriteVaultConfig(root string, patch map[string]any) error {
	commit, err := PrepareVaultConfig(root, patch)
	if err != nil {
		return err
	}
	return commit()
}

// PrepareVaultConfig validates the merged vault config without writing it, so
// a caller saving several files can check them all before touching any.
func PrepareVaultConfig(root string, patch map[string]any) (commit func() error, err error) {
	path := VaultConfigPath(root)
	raw, err := ReadTomlMap(path)
	if err != nil {
		return nil, err
	}
	merged := pickKeys(raw, true)
	deepMerge(merged, pickKeys(patch, true))
	var probe Config
	if err := decodeInto(&probe, merged); err != nil {
		return nil, ValidationError{Errors: []string{err.Error()}}
	}
	if err := Validate(&probe); err != nil {
		return nil, err
	}
	out, err := toml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode vault config: %w", err)
	}
	return func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("mkdir vault config dir: %w", err)
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, out, 0o600); err != nil {
			return fmt.Errorf("write vault config: %w", err)
		}
		return os.Rename(tmp, path)
	}, nil
}

// SplitVaultPatch separates a nested patch into its vault and global parts.
func SplitVaultPatch(patch map[string]any) (vault, global map[string]any) {
	return pickKeys(patch, true), pickKeys(patch, false)
}

// pickKeys copies the leaves of m that are (vault=true) or are not vault keys.
func pickKeys(m map[string]any, vault bool) map[string]any {
	return pickUnder(m, "", vault)
}

func pickUnder(m map[string]any, prefix string, vault bool) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		key := prefix + k
		if sub, ok := v.(map[string]any); ok && !IsVaultKey(key) {
			if picked := pickUnder(sub, key+".", vault); len(picked) > 0 {
				out[k] = picked
			}
			continue
		}
		if IsVaultKey(key) == vault {
			out[k] = v
		}
	}
	return out
}

func decodeInto(cfg *Config, m map[string]any) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           cfg,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return err
	}
	if err := dec.Decode(m); err != nil {
		return fmt.Errorf("decoding vault config: %w", err)
	}
	return nil
}
