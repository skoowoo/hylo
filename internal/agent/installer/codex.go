package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const codexInstallScript = "curl -fsSL https://chatgpt.com/codex/install.sh | sh"

type codexInstaller struct{}

func (codexInstaller) AgentID() string    { return "codex" }
func (codexInstaller) Supported() bool    { return isMacOS() }
func (codexInstaller) InstallCmd() string { return codexInstallScript }

func (codexInstaller) BinDirs(home string) []string {
	return []string{filepath.Join(home, ".local", "bin")}
}

func (codexInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "sh", codexInstallScript, env, out)
}

func (codexInstaller) Update(ctx context.Context, binPath string, env []string, out io.Writer) error {
	return runBinary(ctx, binPath, []string{"update"}, env, out)
}

func (codexInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (codexInstaller) LatestVersion(ctx context.Context, client *http.Client) (string, error) {
	return FetchGitHubLatestTag(ctx, client, "openai", "codex")
}

func (codexInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "brew upgrade --cask codex", "npm i -g @openai/codex", "")
}
