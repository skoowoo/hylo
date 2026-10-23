package skills

import (
	"net/http"

	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/hardhacker/hylo/internal/config"
)

// InstallGitHTTP points go-git at server.proxy for every HTTP remote.
// go-git takes its HTTP client from the process-wide protocol table, so skill
// clones and git sync share one registration made at startup.
func InstallGitHTTP(p config.Proxy) {
	c := &http.Client{Transport: p.Transport()}
	t := githttp.NewClient(c)
	client.InstallProtocol("http", t)
	client.InstallProtocol("https", t)
}
