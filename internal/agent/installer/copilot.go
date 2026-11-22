package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const copilotInstallScript = `curl -fsSL https://gh.io/copilot-install | PREFIX="$HOME/.local" bash`

type copilotInstaller struct{}

func (copilotInstaller) AgentID() string    { return "copilot" }
func (copilotInstaller) Supported() bool    { return isMacOS() }
func (copilotInstaller) InstallCmd() string { return copilotInstallScript }

func (copilotInstaller) BinDirs(home string) []string {
	return []string{filepath.Join(home, ".local", "bin")}
}

func (copilotInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "bash", copilotInstallScript, env, out)
}

func (copilotInstaller) Update(ctx context.Context, _ string, env []string, out io.Writer) error {
	// Copilot CLI has no non-interactive update subcommand (only TUI /update).
	return runShell(ctx, "bash", copilotInstallScript, env, out)
}

func (copilotInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (copilotInstaller) LatestVersion(ctx context.Context, client *http.Client) (string, error) {
	return FetchGitHubLatestTag(ctx, client, "github", "copilot-cli")
}

func (copilotInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "brew upgrade --cask copilot-cli", "npm i -g @github/copilot", "")
}
