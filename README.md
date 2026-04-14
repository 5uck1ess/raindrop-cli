<div align="center">
  <img src=".github/assets/logo.svg" alt="raindrop-cli Logo" width="200">
  <h1>raindrop-cli</h1>

  <a href="https://github.com/5uck1ess/raindrop-cli/actions/workflows/release.yaml"><img alt="Build Workflow" src="https://github.com/5uck1ess/raindrop-cli/actions/workflows/release.yaml/badge.svg"></a>&nbsp;<a href="https://github.com/5uck1ess/raindrop-cli/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/5uck1ess/raindrop-cli"></a>&nbsp;<a href="https://github.com/5uck1ess/raindrop-cli/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/5uck1ess/raindrop-cli"></a><br><br>
  <a href="#capabilities">Capabilities</a> &bull; <a href="#installation">Installation</a> &bull; <a href="#usage">Usage</a> &bull; <a href="#tips-and-notes">Tips & Notes</a>
</div>

---

CLI for [Raindrop.io](https://raindrop.io) bookmark management. Thin wrapper over the [REST API](https://developer.raindrop.io/) — built for bulk cleanup, tag refactoring, and deduplication from the terminal or an AI agent.

Modeled after [Tanq16/gcli](https://github.com/Tanq16/gcli): single static Go binary, auto-released via tag-on-commit, `--for-ai` mode for token-efficient output.

## Capabilities

| Service | Commands | Description |
|---------|----------|-------------|
| Auth | `RAINDROP_TOKEN` env | Bearer token auth via test-token or OAuth |
| Bookmarks | `bookmarks list` | List raindrops by collection with optional search query |
| Tags | `tags list` | List tags in a collection with usage counts |
| Tags | `tags merge`, `tags rename` | Combine or rename tags across a collection (bulk) |
| Tags | `tags delete` | Remove a tag from all raindrops that use it |
| Tools | `tools dedup` | Find and remove duplicate raindrops by URL |
| Tools | `tools broken` | Identify raindrops with broken/unreachable links |
| Global | `--dry-run` | Preview every mutation before it runs |
| Global | `--for-ai` | Plain-text / markdown-table output for agent consumption |
| Global | `--debug` | Verbose request/response logging |

## Installation

### Binary

Download from [releases](https://github.com/5uck1ess/raindrop-cli/releases):

```bash
# Linux/macOS
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64; [ "$ARCH" = "aarch64" ] && ARCH=arm64
curl -sL https://github.com/5uck1ess/raindrop-cli/releases/latest/download/raindrop-$(uname -s | tr '[:upper:]' '[:lower:]')-$ARCH -o raindrop
chmod +x raindrop
sudo mv raindrop /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/5uck1ess/raindrop-cli
cd raindrop-cli
make build          # current platform
make build-all      # all supported targets (linux/darwin × amd64/arm64)
```

## Usage

### Setup

1. Create a test token at [app.raindrop.io/settings/integrations](https://app.raindrop.io/settings/integrations)
2. Export it:
   ```bash
   export RAINDROP_TOKEN="…"
   ```
3. The client enforces a 600ms throttle to stay under the 120 req/min API limit.

### Bookmarks

```bash
raindrop bookmarks list                           # all raindrops
raindrop bookmarks list -c 12345                  # scoped to a collection
raindrop bookmarks list -s "devops kubernetes"    # Raindrop search syntax
raindrop bookmarks list -c 12345 -s "#untagged" --for-ai
```

Collection IDs: `0` = all, `-1` = unsorted, `-99` = trash.

### Tags

```bash
raindrop tags list                                # all tags with counts
raindrop tags list -c 12345

raindrop tags merge --from old1,old2,legacy --to archive --dry-run
raindrop tags merge --from old1,old2,legacy --to archive

raindrop tags rename --from old --to new
raindrop tags delete --tag unwanted --dry-run
```

### Tools

```bash
raindrop tools dedup                              # preview dupes across library
raindrop tools dedup -c 12345 --dry-run
raindrop tools broken -c 12345                    # list broken links
```

### AI agent mode

Pipe-friendly plain output — no spinners, no ANSI colors, table-formatted for token efficiency:

```bash
raindrop bookmarks list -s "#inbox" --for-ai
raindrop tags list --for-ai | head -20
```

## Tips and Notes

- All mutating commands accept `--dry-run` to preview changes before committing — use it on `merge`, `rename`, `delete`, `dedup`.
- `--for-ai` produces plain text / markdown tables — ideal for piping into tools or feeding directly to an agent.
- `--debug` prints structured request/response logs including rate-limit headers (`X-RateLimit-Remaining`, `X-RateLimit-Reset`).
- The client self-throttles to 600ms between calls (120 req/min). Long-running bulk jobs are safe.
- Bulk endpoints handle up to 100 items per call; the CLI paginates automatically.
- Commit to `main` with `[minor-release]` or `[major-release]` in the message to bump; otherwise patch bumps automatically. GitHub Actions builds binaries for linux/darwin × amd64/arm64 and publishes the release.
- For interactive / conversational bookmark queries, pair this with the [official Raindrop MCP server](https://developer.raindrop.io/mcp/mcp) — this CLI is the heavy-lifter for deterministic bulk jobs.
