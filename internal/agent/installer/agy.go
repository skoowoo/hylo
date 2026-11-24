package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const agyInstallScript = "curl -fsSL https://antigravity.google/cli/install.sh | bash"

type agyInstaller struct{}

func (agyInstaller) AgentID() string    { return "agy" }
func (agyInstaller) Supported() bool    { return isMacOS() }
func (agyInstaller) InstallCmd() string { return agyInstallScript }

func (agyInstaller) BinDirs(home string) []string {
	return []string{filepath.Join(home, ".local", "bin")}
}

func (agyInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "bash", agyInstallScript, env, out)
}

func (agyInstaller) Update(ctx context.Context, binPath string, env []string, out io.Writer) error {
	return runBinary(ctx, binPath, []string{"update"}, env, out)
}

func (agyInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (agyInstaller) LatestVersion(context.Context, *http.Client) (string, error) {
	return "", nil
}

func (agyInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "", "", "")
}
