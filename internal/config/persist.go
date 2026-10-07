package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
)

// ResolveConfigWritePath picks the filesystem path used when persisting configuration.
// loadedPath is the path viper successfully read during Load (maybe empty).
func ResolveConfigWritePath(loadedPath string) (string, error) {
	if loadedPath != "" {
		return filepath.Abs(loadedPath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve default config path: %w", err)
	}
	dir := filepath.Join(home, ".hylo")
	return filepath.Join(dir, "config.toml"), nil
}

// ReadTomlMap reads path into nested maps suitable for merging. Missing file yields an empty root.
func ReadTomlMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var root map[string]any
	if err := toml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse toml %s: %w", path, err)
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

// MergeJSONIntoTomlRoot deep-merges a JSON-decoded nested map into the TOML root (dst mutated).
func MergeJSONIntoTomlRoot(dst, src map[string]any) {
	if len(src) == 0 {
		return
	}
	deepMerge(dst, src)
}

// ConfigToMap encodes cfg as the same nested JSON-shaped map returned by GET /api/config (snake_case keys).
func ConfigToMap(cfg *Config) (map[string]any, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// MapToConfig decodes nested mapstructure-shaped data into Config.
func MapToConfig(m map[string]any) (*Config, error) {
	var cfg Config
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &cfg,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(m); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// WriteConfigToml writes merged configuration to path (creates parent directory).
func WriteConfigToml(path string, merged map[string]any) error {
	cfg, err := MapToConfig(merged)
	if err != nil {
		return fmt.Errorf("map to config: %w", err)
	}
	if err := Validate(cfg); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	out, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	return os.Rename(tmp, path)
}

// NormalizeJSONDecodedMap converts json.Number leaf values after json.Unmarshal for cleaner TOML output.
func NormalizeJSONDecodedMap(root map[string]any) {
	for k, v := range root {
		switch x := v.(type) {
		case json.Number:
			i, ierr := x.Int64()
			if ierr == nil {
				root[k] = i
				continue
			}
			f, ferr := x.Float64()
			if ferr == nil {
				root[k] = f
			}
		case map[string]any:
			NormalizeJSONDecodedMap(x)
		case []any:
			for i, el := range x {
				if m, ok := el.(map[string]any); ok {
					NormalizeJSONDecodedMap(m)
				}
				if n, ok := el.(json.Number); ok {
					if iv, err := n.Int64(); err == nil {
						x[i] = iv
						continue
					}
					if fv, err := n.Float64(); err == nil {
						x[i] = fv
					}
				}
			}
		}
	}
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if v == nil {
			continue
		}
		dv, ok := dst[k]
		if !ok {
			dst[k] = mergeCopyValue(v)
			continue
		}
		srcMap, srcIsMap := v.(map[string]any)
		dstMap, dstIsMap := dv.(map[string]any)
		if srcIsMap && dstIsMap {
			deepMerge(dstMap, srcMap)
			dst[k] = dstMap
			continue
		}
		dst[k] = mergeCopyValue(v)
	}
}

func mergeCopyValue(v any) any {
	m, ok := v.(map[string]any)
	if ok {
		out := map[string]any{}
		deepMerge(out, m)
		return out
	}
	if sl, ok := v.([]any); ok {
		cp := make([]any, len(sl))
		copy(cp, sl)
		return cp
	}
	return v
}
