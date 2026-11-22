package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const cursorInstallScript = "curl -fsSL https://cursor.com/install | bash"

type cursorInstaller struct{}

func (cursorInstaller) AgentID() string    { return "cursor-agent" }
func (cursorInstaller) Supported() bool    { return isMacOS() }
func (cursorInstaller) InstallCmd() string { return cursorInstallScript }

func (cursorInstaller) BinDirs(home string) []string {
	return []string{filepath.Join(home, ".local", "bin")}
}

func (cursorInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "bash", cursorInstallScript, env, out)
}

func (cursorInstaller) Update(ctx context.Context, binPath string, env []string, out io.Writer) error {
	return runBinary(ctx, binPath, []string{"update"}, env, out)
}

func (cursorInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (cursorInstaller) LatestVersion(context.Context, *http.Client) (string, error) {
	return "", nil
}

func (cursorInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "", "", "")
}
