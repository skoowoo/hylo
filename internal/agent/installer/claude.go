package installer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
)

const (
	claudeInstallScript = "curl -fsSL https://claude.ai/install.sh | bash"
	claudeStableURL     = "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases/stable"
)

type claudeInstaller struct{}

func (claudeInstaller) AgentID() string  { return "claude" }
func (claudeInstaller) Supported() bool  { return isMacOS() }
func (claudeInstaller) InstallCmd() string { return claudeInstallScript }

func (claudeInstaller) BinDirs(home string) []string {
	return []string{filepath.Join(home, ".local", "bin")}
}

func (claudeInstaller) Install(ctx context.Context, env []string, out io.Writer) error {
	return runShell(ctx, "bash", claudeInstallScript, env, out)
}

func (claudeInstaller) Update(ctx context.Context, binPath string, env []string, out io.Writer) error {
	return runBinary(ctx, binPath, []string{"update"}, env, out)
}

func (claudeInstaller) NormalizeVersion(raw string) string { return ExtractSemver(raw) }

func (claudeInstaller) LatestVersion(ctx context.Context, client *http.Client) (string, error) {
	return FetchPlainTextVersion(ctx, client, claudeStableURL)
}

func (claudeInstaller) ExternalUpdateHint(source InstallSource, binPath string) string {
	return defaultExternalHint(source, binPath, "brew upgrade --cask claude-code", "npm i -g @anthropic-ai/claude-code", "")
}
