package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
)

// vaultSections describe one vault rather than the install, so
// <vault>/.hylo/config.toml holds them and the settings UI writes them there.
// The global file cannot set them: a new vault must not inherit the last one's plugins.
var vaultSections = []string{"plugins", "proactive"}

// IsVaultKey reports whether the dotted key belongs in the vault's config.
func IsVaultKey(key string) bool {
	for _, s := range vaultSections {
		if key == s || strings.HasPrefix(key, s+".") {
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
	// Global files from older installs still carry these sections. Drop them
	// before the vault file is applied, then move any explicit values across.
	if err := resetVaultScoped(cfg); err != nil {
		return err
	}

	raw, err := ReadTomlMap(VaultConfigPath(root))
	if err != nil {
		return fmt.Errorf("reading vault config: %w", err)
	}
	m := pickKeys(raw, true)
	if globalPath != "" {
		if moved := migrateVaultScoped(root, globalPath, legacy); moved != nil {
			m = moved
		}
	}
	return decodeInto(cfg, m)
}

// resetVaultScoped puts vault sections back to built-in defaults so values
// read from the global file do not apply.
func resetVaultScoped(cfg *Config) error {
	def, err := defaultConfig()
	if err != nil {
		return err
	}
	cfg.Plugins = def.Plugins
	cfg.Proactive = def.Proactive
	return nil
}

// WriteVaultConfig merges the vault keys of patch into the vault's config.toml.
func WriteVaultConfig(root string, patch map[string]any) error {
	commit, err := PrepareVaultConfig(root, patch)
	if err != nil {
		return err
	}
	return commit()
}

// writeVaultMerge is WriteVaultConfig that still writes when the vault file was
// already invalid in exactly the same way. Migration must be able to move a
// token onto a file that, for example, enabled git sync without a branch.
func writeVaultMerge(root string, patch map[string]any) error {
	path, out, err := mergedVault(root, patch)
	if err == nil {
		return commitVault(path, out)
	}
	var ve ValidationError
	if !errors.As(err, &ve) || len(out) == 0 || !sameVaultInvalid(root, err) {
		return err
	}
	return commitVault(path, out)
}

// sameVaultInvalid reports whether err is the validation result of the vault
// file with nothing merged, so patch did not add a new error.
func sameVaultInvalid(root string, err error) bool {
	var got ValidationError
	if !errors.As(err, &got) {
		return false
	}
	_, bare := PrepareVaultConfig(root, nil)
	var had ValidationError
	return errors.As(bare, &had) && had.Error() == got.Error()
}

// PrepareVaultConfig validates the merged vault config without writing it, so
// a caller saving several files can check them all before touching any.
func PrepareVaultConfig(root string, patch map[string]any) (commit func() error, err error) {
	path, out, err := mergedVault(root, patch)
	if err != nil {
		return nil, err
	}
	return func() error { return commitVault(path, out) }, nil
}

// mergedVault merges patch into the vault file. On a validation failure the
// encoded bytes are still returned, so a pre-existing error can be written through.
func mergedVault(root string, patch map[string]any) (path string, out []byte, err error) {
	path = VaultConfigPath(root)
	raw, err := ReadTomlMap(path)
	if err != nil {
		return "", nil, err
	}
	merged := pickKeys(raw, true)
	deepMerge(merged, pickKeys(patch, true))
	// A key the vault file never sets keeps its built-in default, which is
	// what Load and the settings screen already show. A zero Config would
	// read that as "" and reject turning rounds on.
	probe, err := defaultConfig()
	if err != nil {
		return "", nil, err
	}
	if err := decodeInto(probe, merged); err != nil {
		return "", nil, ValidationError{Errors: []string{err.Error()}}
	}
	out, err = toml.Marshal(merged)
	if err != nil {
		return "", nil, fmt.Errorf("encode vault config: %w", err)
	}
	if err := Validate(probe); err != nil {
		return path, out, err
	}
	return path, out, nil
}

func commitVault(path string, out []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("mkdir vault config dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("write vault config: %w", err)
	}
	return os.Rename(tmp, path)
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
