package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const openCodeInstallScript = "curl -fsSL https://opencode.ai/v2/install | bash"

type openCodeInstaller struct{}

func (openCodeInstaller) AgentID() string    { return "opencode" }
func (openCodeInstaller) Supported() bool    { return isMacOS() }
func (openCodeInstaller) InstallCmd() string { return openCodeInstallScript }

func (openCodeInstaller) BinDirs(home string) []string {
	return []string{
		filepath.Join(home, ".opencode", "bin"),
		filepath.Join(home, ".local", "bin"),
	}
}

func (openCodeInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "bash", openCodeInstallScript, env, out)
}

func (openCodeInstaller) Update(ctx context.Context, binPath string, env []string, out io.Writer) error {
	return runBinary(ctx, binPath, []string{"upgrade"}, env, out)
}

func (openCodeInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (openCodeInstaller) LatestVersion(ctx context.Context, client *http.Client) (string, error) {
	return FetchGitHubLatestTag(ctx, client, "anomalyco", "opencode")
}

func (openCodeInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "brew upgrade anomalyco/tap/opencode-v2", "", "")
}
