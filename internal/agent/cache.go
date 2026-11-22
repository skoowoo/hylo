package agent

import (
	"context"
	"sync"
	"time"
)

const defaultAgentCacheTTL = time.Hour

// AgentCache holds a stale-while-revalidate in-memory cache of detected agents.
// On first access with no cache it blocks until detection completes.
// On subsequent accesses it returns the cached result immediately and, if stale,
// triggers a single background refresh so the next caller sees fresh data.
type AgentCache struct {
	mu            sync.Mutex
	agents        []AgentInfo
	fetchedAt     time.Time
	refreshing    bool
	ttl           time.Duration
	configuredEnv map[string]map[string]string // agentID -> env overrides from config
}

// NewAgentCache creates a cache with the given TTL and per-agent env overrides.
// Pass nil for configuredEnv to use defaults.
func NewAgentCache(ttl time.Duration, configuredEnv map[string]map[string]string) *AgentCache {
	if ttl <= 0 {
		ttl = defaultAgentCacheTTL
	}
	if configuredEnv == nil {
		configuredEnv = map[string]map[string]string{}
	}
	return &AgentCache{ttl: ttl, configuredEnv: configuredEnv}
}

// AgentListResult is returned by Get.
type AgentListResult struct {
	Agents    []AgentInfo
	FromCache bool
	Stale     bool
	FetchedAt time.Time
}

// WarmUp starts a one-shot background goroutine to pre-populate the cache.
// Safe to call multiple times; only the first call that finds an empty cache
// will actually trigger detection.
func (c *AgentCache) WarmUp() {
	c.mu.Lock()
	if !c.fetchedAt.IsZero() || c.refreshing {
		c.mu.Unlock()
		return
	}
	c.refreshing = true
	c.mu.Unlock()

	go func() {
		agents := DetectAgents(context.Background(), c.configuredEnv)
		c.mu.Lock()
		c.agents = agents
		c.fetchedAt = time.Now()
		c.refreshing = false
		c.mu.Unlock()
	}()
}

// Get returns the agent list.
//   - force=false: stale-while-revalidate — returns any cached value immediately,
//     triggers a background refresh when the cache is stale (local probe only).
//     Blocks only when there is no cache at all (first call ever).
//   - force=true: re-detects synchronously, checks upstream latest versions, and updates the cache.
func (c *AgentCache) Get(ctx context.Context, force bool) AgentListResult {
	if force {
		agents := DetectAgentsWithOptions(ctx, c.configuredEnv, true)
		c.mu.Lock()
		c.agents = agents
		c.fetchedAt = time.Now()
		c.refreshing = false
		c.mu.Unlock()
		return AgentListResult{Agents: agents, FromCache: false, FetchedAt: c.fetchedAt}
	}

	c.mu.Lock()
	hasCache := !c.fetchedAt.IsZero()
	isFresh := hasCache && time.Since(c.fetchedAt) < c.ttl
	c.mu.Unlock()

	if !hasCache {
		// No cache yet — block until first detection.
		agents := DetectAgents(ctx, c.configuredEnv)
		c.mu.Lock()
		c.agents = agents
		c.fetchedAt = time.Now()
		c.mu.Unlock()
		return AgentListResult{Agents: agents, FromCache: false, FetchedAt: c.fetchedAt}
	}

	// Return cached data immediately.
	c.mu.Lock()
	result := AgentListResult{
		Agents:    c.agents,
		FromCache: true,
		Stale:     !isFresh,
		FetchedAt: c.fetchedAt,
	}
	shouldRefresh := !isFresh && !c.refreshing
	if shouldRefresh {
		c.refreshing = true
	}
	c.mu.Unlock()

	// Kick off a single background refresh when stale.
	if shouldRefresh {
		go func() {
			agents := DetectAgents(context.Background(), c.configuredEnv)
			c.mu.Lock()
			c.agents = agents
			c.fetchedAt = time.Now()
			c.refreshing = false
			c.mu.Unlock()
		}()
	}

	return result
}

// RefreshOne re-probes a single agent (e.g. right after install/update) and
// updates its entry in the cache so other tabs immediately see the new state.
func (c *AgentCache) RefreshOne(ctx context.Context, agentID string) (AgentInfo, bool) {
	def := GetAgentDef(agentID)
	if def == nil {
		return AgentInfo{}, false
	}
	cfg := c.configuredEnv[agentID]
	if cfg == nil {
		cfg = map[string]string{}
	}
	info := ProbeAgent(ctx, def, cfg)
	c.mu.Lock()
	defer c.mu.Unlock()
	replaced := false
	for i := range c.agents {
		if c.agents[i].ID == agentID {
			c.agents[i] = info
			replaced = true
			break
		}
	}
	if !replaced && len(c.agents) > 0 {
		c.agents = append(c.agents, info)
	}
	return info, true
}
