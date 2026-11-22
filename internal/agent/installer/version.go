package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var semverRe = regexp.MustCompile(`(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)`)

// ExtractSemver pulls the first semver token (X.Y.Z[-pre]) out of raw CLI output.
func ExtractSemver(raw string) string {
	s := strings.TrimSpace(raw)
	if m := semverRe.FindStringSubmatch(s); len(m) == 2 {
		return m[1]
	}
	return strings.TrimPrefix(s, "v")
}

// IsNewer reports whether latest is strictly newer than installed.
func IsNewer(installed, latest string) bool {
	a, okA := parseSemver(installed)
	b, okB := parseSemver(latest)
	if !okA || !okB {
		return false
	}
	for i := 0; i < 3; i++ {
		if b.nums[i] != a.nums[i] {
			return b.nums[i] > a.nums[i]
		}
	}
	if a.pre == b.pre {
		return false
	}
	// Release version is newer than pre-release of the same core version.
	if a.pre != "" && b.pre == "" {
		return true
	}
	if a.pre == "" && b.pre != "" {
		return false
	}
	return b.pre > a.pre
}

type semver struct {
	nums [3]int
	pre  string
}

func parseSemver(s string) (semver, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "v"))
	m := semverRe.FindStringSubmatch(s)
	if len(m) != 2 {
		return semver{}, false
	}
	core := m[1]
	var pre string
	if i := strings.IndexByte(core, '-'); i >= 0 {
		pre = core[i+1:]
		core = core[:i]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var out semver
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return semver{}, false
		}
		out.nums[i] = n
	}
	out.pre = pre
	return out, true
}

// FetchGitHubLatestTag resolves github.com/<owner>/<repo>/releases/latest via 302 Location,
// avoiding the unauthenticated GitHub REST API rate limit.
func FetchGitHubLatestTag(ctx context.Context, client *http.Client, owner, repo string) (string, error) {
	c := cloneNoRedirect(client)
	endpoint := fmt.Sprintf("https://github.com/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("missing redirect location")
	}
	u, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	tag := path.Base(strings.TrimRight(u.Path, "/"))
	if tag == "" || tag == "." || tag == "releases" || tag == "latest" {
		return "", fmt.Errorf("no release tag in %q", loc)
	}
	return ExtractSemver(tag), nil
}

// FetchPlainTextVersion fetches a plain-text version endpoint (e.g. Claude Code GCS stable).
func FetchPlainTextVersion(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	v := ExtractSemver(string(body))
	if v == "" {
		return "", fmt.Errorf("empty version response")
	}
	return v, nil
}

func cloneNoRedirect(base *http.Client) *http.Client {
	c := http.Client{}
	if base != nil {
		c = *base
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &c
}

func runShell(ctx context.Context, shell, script string, env []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, shell, "-c", script)
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = out
	prepareCommand(cmd)
	return cmd.Run()
}

func runBinary(ctx context.Context, bin string, args []string, env []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = out
	prepareCommand(cmd)
	return cmd.Run()
}
