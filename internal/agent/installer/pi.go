package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const piInstallScript = "curl -fsSL https://pi.dev/install.sh | sh"

type piInstaller struct{}

func (piInstaller) AgentID() string    { return "pi" }
func (piInstaller) Supported() bool    { return isMacOS() }
func (piInstaller) InstallCmd() string { return piInstallScript }

func (piInstaller) BinDirs(home string) []string {
	return []string{filepath.Join(home, ".local", "bin")}
}

func (piInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "sh", piInstallScript, env, out)
}

func (piInstaller) Update(ctx context.Context, binPath string, env []string, out io.Writer) error {
	return runBinary(ctx, binPath, []string{"update"}, env, out)
}

func (piInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (piInstaller) LatestVersion(ctx context.Context, client *http.Client) (string, error) {
	return FetchGitHubLatestTag(ctx, client, "earendil-works", "pi")
}

func (piInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "", "npm i -g @earendil-works/pi-coding-agent", "nix profile upgrade pi")
}
