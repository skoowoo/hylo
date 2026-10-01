# Developer Handbook

A living reference for architecture decisions, coding standards, and operational
runbooks. Keep it up-to-date as the system evolves.

Related notes: [[Architecture Overview]] · [[Deployment Runbook]] · [[Team Norms]]

---

## Table of Contents

- [Project Structure](#project-structure)
- [Development Setup](#development-setup)
- [Architecture](#architecture)
- [API Reference](#api-reference)
- [Database](#database)
- [Testing](#testing)
- [Deployment](#deployment)
- [Troubleshooting](#troubleshooting)

---

## Project Structure

The repository follows a standard Go project layout. Each package has a single,
well-defined responsibility.

```
.
├── cmd/
│   └── hylo/         # main entry point
├── internal/
│   ├── cli/             # cobra commands
│   ├── client/          # HTTP client for the server API
│   ├── config/          # viper-based config loading
│   ├── server/
│   │   ├── handler/     # HTTP request handlers
│   │   └── router.go    # route registration
│   ├── storage/         # vault + SQLite layer
│   └── util/            # shared helpers
└── testdata/            # fixtures used by tests
```

Key conventions:

- All vault paths are **absolute** and start with `/` (e.g. `/journal/today.md`).
- The root directory is represented as `"/"`, never as `""` or `"."`.
- Public API surfaces live in `internal/`; nothing is exported from `cmd/`.

See also: [[Project Layout#Packages]] and [[Coding Conventions]].

---

## Development Setup

### Prerequisites

| Tool    | Minimum version | Notes                          |
|---------|-----------------|--------------------------------|
| Go      | 1.22            | modules required               |
| SQLite  | 3.40            | bundled via `modernc.org/sqlite` |
| Git     | 2.39            | for the git-sync plugin        |
| Make    | 3.81            | optional, for convenience targets |

### Installation

Clone and build in one step:

```bash
git clone https://github.com/hardhacker/hylo
cd hylo
go build -o bin/hylo ./cmd/hylo
```

Run the server pointing at your Obsidian vault:

```bash
./bin/hylo serve --vault ~/Notes --port 7070
```

On first run, Hylo creates `.hylo/meta.db` inside the vault root. This
file is **safe to delete** — it will be rebuilt from the filesystem on the next
start, though re-indexing may take a moment for large vaults.

### Editor Integration

Add the binary to your `PATH` and configure your editor's shell environment:

```bash
# ~/.zshrc or ~/.bashrc
export PATH="$HOME/hardhacker/hylo/bin:$PATH"
export HYLO_API_KEY="your-key-here"
```

For VS Code, install the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client)
extension and point it at `http://localhost:7070`.

---

## Architecture

### Overview

Hylo is a thin HTTP server that wraps an Obsidian vault directory. All state
lives in two places:

1. **The filesystem** — source of truth for note content.
2. **SQLite** (`meta.db`) — index of metadata (path, size, timestamps, flags).

The server is single-process, single-writer. SQLite runs in WAL mode with
`max_open_conns = 1` to avoid `SQLITE_BUSY` under concurrent requests.

### Request Lifecycle

```
CLI / MCP client
      │
      ▼
  HTTP POST /api/vault/write
      │
      ▼
  VaultHandler.Write()          ← handler/vault_put.go
      │
      ├─► storage.Vault.WriteNote()    writes file to disk
      │
      └─► dbUpsert()                   updates SQLite metadata
```

### Plugin System

Plugins are optional modules that hook into vault events. Each plugin is
**disabled by default** and must be enabled explicitly in `hylo.yaml`.

Currently available plugins:

- **search** — full-text search via Bleve; supports Jieba tokenisation for CJK.
- **git_sync** — auto-commit and push on a configurable interval.
- **summarize** — LLM-generated summaries stored alongside notes.

```yaml
plugins:
  search:
    enabled: true
    use_jieba: true
  git_sync:
    enabled: true
    remote: origin
    branch: main
    auto_commit: true
    sync_interval: 5m
  summarize:
    enabled: false
    provider: anthropic
    model: claude-haiku-4-5-20251001
    output_dir: /summaries
    language: zh
```

Related: [[Plugin Architecture]] · [[Search Plugin Deep Dive]]

---

## API Reference

All endpoints accept and return JSON. Authentication is via the
`X-Hylo-API-Key` header when `api_key` is set in config.

### Read a note

```http
POST /api/vault/read
Content-Type: application/json

{
  "path": "/journal/2026/april.md"
}
```

Response: raw file bytes with `Content-Type: text/plain; charset=utf-8`.

Use `"name"` instead of `"path"` for a vault-wide name lookup:

```json
{ "name": "april.md" }
```

### Write a note

```http
POST /api/vault/write
Content-Type: application/json

{
  "path": "/journal/2026/april.md",
  "content": "# April\n\nContent here.",
  "append": false
}
```

Set `"append": true` to append with smart markdown spacing instead of
overwriting.

### List notes

```http
POST /api/vault/list
Content-Type: application/json

{
  "path": "/journal",
  "sort": "time",
  "limit": 20
}
```

Pass `"all": true` to list the entire vault regardless of directory.

### Search

Requires the search plugin to be enabled and the index to be built.

```http
POST /api/search
Content-Type: application/json

{
  "q": "architecture sqlite wal",
  "type": "content",
  "limit": 10
}
```

`type` accepts `"name"`, `"content"`, or omit for both.

---

## Database

### Schema

```sql
CREATE TABLE IF NOT EXISTS notes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    dir         TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    size        INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    indexed     INTEGER NOT NULL DEFAULT 0,
    summarized  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(dir, name)
);
```

Timestamps are stored as **Unix nanoseconds** (int64) for sub-second precision
without timezone ambiguity.

### Path Encoding

| Vault path            | `dir`        | `name`       |
|-----------------------|--------------|--------------|
| `/note.md`            | `/`          | `note.md`    |
| `/journal/today.md`   | `/journal`   | `today.md`   |
| `/a/b/c/deep.md`      | `/a/b/c`     | `deep.md`    |

The `(dir, name)` pair is the **composite unique key**. Never construct paths by
string concatenation — use `storage.JoinPath()` and `storage.PathParts()`.

### Common Queries

Find notes modified in the last 7 days:

```sql
SELECT dir, name, updated_at
FROM notes
WHERE updated_at >= ?
ORDER BY updated_at DESC
LIMIT 50;
```

Find all unindexed notes for the search backfill:

```sql
SELECT dir, name FROM notes WHERE indexed = 0;
```

Reset the search index (forces full re-index on next startup):

```sql
UPDATE notes SET indexed = 0;
```

---

## Testing

### Running Tests

```bash
# all packages
go test ./...

# single package, verbose
go test ./internal/cli/ -v

# specific test function
go test ./internal/cli/ -run TestMdParseSection -v

# with race detector
go test -race ./...
```

### Test Conventions

1. **Table-driven tests** — every test function uses a `tests []struct{...}` slice
   and a `for _, tc := range tests` loop. Subtests are named via `t.Run(tc.name, ...)`.
2. **No mocks for storage** — integration tests hit a real temp vault. We were
   [burned by mock/prod divergence](https://github.com/hardhacker/hylo/issues/42)
   during a migration last quarter.
3. **Testdata files** live in `testdata/` at the repo root. Prefer real markdown
   fixtures over inline strings for complex parsing tests.

### Coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

Target: **≥ 80 %** line coverage for `internal/storage` and `internal/cli`.
Parser helpers (`mdParse*`) should be close to **100 %**.

---

## Deployment

### Production Config

```yaml
# /etc/hylo/hylo.yaml
vault: /data/vault
server:
  host: 127.0.0.1
  port: 7070
  api_key: "${HYLO_API_KEY}"
  tls:
    cert: /etc/hylo/tls/cert.pem
    key:  /etc/hylo/tls/key.pem
log:
  level: info
  format: json
```

### Systemd Unit

```ini
[Unit]
Description=Hylo note server
After=network.target

[Service]
Type=simple
User=hylo
ExecStart=/usr/local/bin/hylo serve --config /etc/hylo/hylo.yaml
Restart=on-failure
RestartSec=5s
Environment=HYLO_API_KEY=changeme

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now hylo
sudo journalctl -u hylo -f
```

### Health Check

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:7070/api/info
# expected: 200
```

### Backup

The vault directory is the only thing that needs backing up. The SQLite database
is derived state and can be rebuilt.

```bash
# simple rsync snapshot
rsync -av --delete ~/Notes/ /backup/notes-$(date +%Y%m%d)/
```

---

## Troubleshooting

### Server won't start

1. Check the config file path: `hylo serve --config /path/to/hylo.yaml`
2. Verify the vault directory exists and is readable.
3. Check for port conflicts: `lsof -i :7070`

### SQLITE_BUSY errors

This should not happen in normal operation (single writer, WAL mode). If you
see them:

- Ensure only **one** Hylo process is running against the vault.
- Check for orphaned lock files: `ls -la /path/to/vault/.hylo/`
- Delete `meta.db` and restart — it will be rebuilt automatically.

### Search returns no results

- [ ] Is the search plugin enabled in config?
- [ ] Has the index been built? Check `indexed` counts:

```bash
./bin/hylo info | jq .vault
sqlite3 ~/.notes/.hylo/meta.db "SELECT COUNT(*) FROM notes WHERE indexed=0;"
```

- [ ] Re-trigger indexing by restarting the server (it runs a backfill on startup).

### Notes not appearing after manual file edits

Hylo's SQLite index is updated **only through the API**. If you edit files
directly on disk, you need to either:

1. Restart the server (it reconciles the DB against the filesystem on startup), or
2. Use `hylo sync` to trigger a manual reconciliation.

> **Note:** Direct filesystem edits bypass the git-sync plugin's change
> detection. Commit manually if needed: `cd ~/Notes && git add -A && git commit -m "manual edit"`

---

## Appendix

### Useful One-liners

List the 10 most recently modified raw notes:

```bash
./bin/hylo raw list --limit 10
```

Extract all Go code blocks from a note:

```bash
./bin/hylo extract code /docs/architecture.md
```

Resolve a bare filename to all vault locations:

```bash
./bin/hylo resolve today.md
```

Pull the outline of a long note:

```bash
./bin/hylo extract outline /docs/developer-handbook.md
```

### Glossary

**vault** — the root directory that Hylo manages; typically your Obsidian
notes folder.

**vault-absolute path** — a `/`-prefixed path relative to the vault root, e.g.
`/journal/2026/april.md`. Never a filesystem absolute path.

**dir** — the directory component of a vault-absolute path. The root is `"/"`.

**name** — the filename component, always including the `.md` or `.markdown`
extension.

**upsert** — insert-or-update; Hylo uses SQLite's `ON CONFLICT DO UPDATE`
to keep metadata in sync with file writes.

### External References

- Go documentation: <https://pkg.go.dev/github.com/hardhacker/hylo>
- Goldmark AST reference: [yuin/goldmark](https://github.com/yuin/goldmark)
- Bleve search docs: [blevesearch.com](https://blevesearch.com/docs/Home/)
- SQLite WAL mode: [sqlite.org/wal.html](https://www.sqlite.org/wal.html)
- CommonMark spec: <https://spec.commonmark.org>

### Changelog

#### v0.4 — 2026-03-15

- Added `extract section`, `extract code`, `extract list`, `extract link` subcommands.
- Wiki link (`[[Page]]`) support in `extract link`.
- `create` now checks for duplicate filenames vault-wide before writing.

#### v0.3 — 2026-01-20

- Introduced the summarize plugin with Anthropic / OpenAI / Ollama backends.
- `resolve` command for vault-wide name lookup.
- TLS support for the HTTP server.

#### v0.2 — 2025-11-05

- Git-sync plugin: auto-commit and push on a configurable interval.
- Full-text search via Bleve; optional Jieba tokeniser for CJK text.
- `list` command with `--sort time` and `--limit` flags.

#### v0.1 — 2025-09-01

- Initial release: read, write, delete, list over HTTP.
- SQLite metadata index with WAL mode.
- Basic CLI wrapping the HTTP API.
