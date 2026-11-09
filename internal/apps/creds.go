package apps

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// credential is one connected app: how it signed in and what its last check found.
type credential struct {
	Method string      `json:"method"`
	Secret string      `json:"secret,omitempty"` // key or API token
	OAuth  *oauthGrant `json:"oauth,omitempty"`

	Account     string    `json:"account,omitempty"`
	Problem     string    `json:"problem,omitempty"`
	Reconnect   bool      `json:"reconnect,omitempty"`
	ConnectedAt time.Time `json:"connectedAt"` // which connection this is: findings land only on their own
}

// usable is a credential the platform hasn't refused: bots get the app.
func (c *credential) usable() bool { return c != nil && !c.Reconnect }

// oauthGrant is a browser sign-in: enough to refresh and to revoke it.
type oauthGrant struct {
	ClientID  string        `json:"clientId"`
	TokenURL  string        `json:"tokenUrl"`
	RevokeURL string        `json:"revokeUrl,omitempty"`
	Resource  string        `json:"resource"`
	Token     *oauth2.Token `json:"token"`
}

// credStore keeps one file per connected app in dir, readable by the user only:
// secrets stay out of the vault, which may sync or sit in git.
type credStore struct {
	dir string
	mu  sync.Mutex
	m   map[string]*credential
}

func openCreds(dir string) (*credStore, error) {
	if dir == "" {
		return nil, errors.New("apps: no credential dir")
	}
	s := &credStore{dir: dir, m: map[string]*credential{}}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var c credential
		if err := json.Unmarshal(b, &c); err != nil {
			continue
		}
		s.m[id] = &c
	}
	return s, nil
}

// get returns a copy, so callers never race a refresh writing the original.
func (s *credStore) get(id string) *credential {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[id]
	if !ok {
		return nil
	}
	cp := *c
	if c.OAuth != nil {
		g := *c.OAuth
		cp.OAuth = &g
	}
	return &cp
}

// put keeps c as id's credential, replacing whatever was there: a new connection.
func (s *credStore) put(id string, c *credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(id, c)
}

// update applies fn to id's credential, when it is still the one connected at
// since. Checks and calls run on a copy across a network round trip; writing
// that copy back would revive a disconnected app, undo a reconnect, or put
// back a token a refresh just rotated, so they only patch what they found.
func (s *credStore) update(id string, since time.Time, fn func(*credential)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.m[id]
	if !ok || !cur.ConnectedAt.Equal(since) {
		return nil
	}
	c := *cur
	if cur.OAuth != nil {
		g := *cur.OAuth
		c.OAuth = &g
	}
	fn(&c)
	return s.write(id, &c)
}

// write is called with s.mu held.
func (s *credStore) write(id string, c *credential) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "."+id+".json.tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(s.dir, id+".json")); err != nil {
		return err
	}
	cp := *c
	s.m[id] = &cp
	return nil
}

// setToken keeps a refreshed token for the sign-in connected at since.
func (s *credStore) setToken(id string, since time.Time, t *oauth2.Token) error {
	return s.update(id, since, func(c *credential) {
		if c.OAuth != nil {
			c.OAuth.Token = t
		}
	})
}

func (s *credStore) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	err := os.Remove(filepath.Join(s.dir, id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
