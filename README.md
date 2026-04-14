<div align="center">
  <img src=".github/assets/logo.svg" alt="raindrop-cli Logo" width="200">
  <h1>raindrop-cli</h1>

  <a href="https://github.com/5uck1ess/raindrop-cli/actions/workflows/release.yaml"><img alt="Build Workflow" src="https://github.com/5uck1ess/raindrop-cli/actions/workflows/release.yaml/badge.svg"></a>&nbsp;<a href="https://github.com/5uck1ess/raindrop-cli/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/5uck1ess/raindrop-cli?color=green&v=2"></a>&nbsp;<a href="https://github.com/5uck1ess/raindrop-cli/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/5uck1ess/raindrop-cli?v=2"></a><br><br>
  <a href="#capabilities">Capabilities</a> &bull; <a href="#installation">Installation</a> &bull; <a href="#usage">Usage</a> &bull; <a href="#tips-and-notes">Tips & Notes</a> &bull; <a href="#release">Release</a> &bull; <a href="#credits">Credits</a> &bull; <a href="#license">License</a>
</div>

---

CLI for [Raindrop.io](https://raindrop.io) bookmark management. Thin wrapper over the [REST API](https://developer.raindrop.io/) — built for bulk cleanup, tag refactoring, and deduplication from the terminal or an AI agent.

## Capabilities

| Service | Commands | Description |
|---------|----------|-------------|
| Auth | `RAINDROP_TOKEN` env | Bearer token auth via test-token or OAuth |
| Bookmarks | `bookmarks list` | List raindrops by collection with optional search query |
| Bookmarks | `bookmarks untagged` | List all raindrops with empty tags (client-side filter) |
| Bookmarks | `bookmarks tag` | Add / remove / replace tags on one or many raindrops |
| Collections | `collections list` | Flat, tree (`--tree`), or TSV (`--for-ai`) view of all collections |
| Collections | `collections create` | Create a collection under root or a parent; prints new ID |
| Collections | `collections move` | Reparent a collection (`--parent root` to promote) |
| Collections | `collections rename` | Change a collection's title |
| Collections | `collections delete` | Delete one (`--id`) or all empty (`--empty`); items go to Trash |
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

Download from [releases](https://github.com/5uck1ess/raindrop-cli/releases).

**Linux / macOS:**

```bash
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64; [ "$ARCH" = "aarch64" ] && ARCH=arm64
curl -sL https://github.com/5uck1ess/raindrop-cli/releases/latest/download/raindrop-$(uname -s | tr '[:upper:]' '[:lower:]')-$ARCH -o raindrop
chmod +x raindrop
sudo mv raindrop /usr/local/bin/
```

**Windows (PowerShell):**

```powershell
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
Invoke-WebRequest -Uri "https://github.com/5uck1ess/raindrop-cli/releases/latest/download/raindrop-windows-$arch.exe" -OutFile "$env:USERPROFILE\bin\raindrop.exe"
# Ensure $env:USERPROFILE\bin is on your PATH
```

### Build from Source

```bash
git clone https://github.com/5uck1ess/raindrop-cli
cd raindrop-cli
make build          # current platform
make build-all      # all targets (linux/darwin/windows × amd64/arm64)
```

## Usage

### Setup

1. Go to [app.raindrop.io/settings/integrations](https://app.raindrop.io/settings/integrations) and click **"Create new app"** under *For Developers*. Name it anything (e.g. `raindrop-cli`).
2. Click into your new app, scroll down, and hit **"Create test token"**. Copy the token string — it's scoped to your own account only.
3. Export it:
   ```bash
   export RAINDROP_TOKEN="…"
   ```
4. The client auto-throttles to 600ms between requests to stay under the 120 req/min API limit.

> **Why test tokens, not OAuth?** For a single-user CLI that accesses only the developer's own Raindrop.io account, a personal (test) token is the simplest and most appropriate authentication method. OAuth is necessary when the application needs to authenticate multiple users or is deployed as a hosted or distributed service.

### Bookmarks

```bash
raindrop bookmarks list                           # all raindrops (auto-paginates)
raindrop bookmarks list -c 12345                  # scoped to a collection
raindrop bookmarks list -s "devops kubernetes"    # Raindrop search syntax
raindrop bookmarks list --include-collection      # add COLLECTION column
raindrop bookmarks list --page 0 --per-page 50    # explicit paging (disables auto)

raindrop bookmarks untagged                       # items with empty tags[]
raindrop bookmarks untagged --for-ai              # TSV: id, collection_id, domain, title, link
```

`list` auto-paginates through every page by default. Pass `--page` or `--per-page` to page explicitly.

Collection IDs: `0` = all, `-1` = unsorted, `-99` = trash.

Tag a single bookmark or a batch from a TSV file:

```bash
# Single-id mode
raindrop bookmarks tag --id 12345 --add ai,tools           # append
raindrop bookmarks tag --id 12345 --remove legacy          # remove
raindrop bookmarks tag --id 12345 --set ai,tools           # replace

# Batch mode — pick a --mode, feed a TSV:
raindrop bookmarks tag --from-file plan.tsv --mode add --progress
raindrop bookmarks tag --from-file plan.tsv --mode set
raindrop bookmarks tag --from-file plan.tsv --mode remove
```

TSV formats (`#` comments ignored):

```
<id>\t<tag1,tag2,...>                    # preflights to resolve collection_id
<id>\t<collection_id>\t<tag1,tag2,...>   # direct bulk, no preflight
```

Pipe the output of `bookmarks untagged --for-ai` (which already includes `collection_id`) into your plan file and you get the 3-column form for free.

> **Performance**: `--mode add` and `--mode set` use Raindrop's bulk `PUT /raindrops/{cid}` endpoint — up to 100 ids per call, grouped by `(collection_id, tag_set)`. ~1000 items tag in seconds instead of minutes. `--mode remove` always uses per-item writes (bulk cannot remove specific tags). Pass `--no-bulk` to force per-item on any mode.

### Collections

```bash
raindrop collections list                         # flat table
raindrop collections list --tree                  # indented tree (roots by sort, children by count desc)
raindrop collections list --for-ai                # TSV: id, parent_id, count, title
raindrop collections list --tree --for-ai         # TSV with depth column: depth, id, parent_id, count, title

raindrop collections create --title "🧪 Lab"                  # at root
raindrop collections create --title "sub" --parent 12345      # nested
raindrop collections rename --id 12345 --to "New Title"
raindrop collections move   --id 12345 --parent 67890         # reparent
raindrop collections move   --id 12345 --parent root          # promote to root
raindrop collections delete --id 12345                        # errors if non-empty
raindrop collections delete --id 12345 --force                # deletes; items → Trash
raindrop collections delete --empty --dry-run                 # preview prune
raindrop collections delete --empty                           # prune zero-count (incl. parent buckets!)
raindrop collections delete --empty --leaf-only               # prune only childless zero-count
raindrop collections create --title "X" --parent 123 --quiet  # prints just the new ID
```

> **Deleting a collection moves its items to Trash** (Raindrop API behavior), not Unsorted. Use `raindrop bookmarks list -c -99` to inspect, or restore in the web UI.

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
- `--for-ai` produces pure TSV — header row plus tab-separated data rows. Summary / info lines go to stderr so `wc -l`, `jq`, `awk` on stdout see only payload rows. ~30% fewer tokens than markdown for the same data.
- `--debug` prints structured request/response logs including rate-limit headers (`X-RateLimit-Remaining`, `X-RateLimit-Reset`).
- The client self-throttles to 600ms between calls (120 req/min). Long-running bulk jobs are safe.
- Bulk endpoints handle up to 100 items per call; the CLI paginates automatically.
- For interactive / conversational bookmark queries, pair this with the [official Raindrop MCP server](https://developer.raindrop.io/mcp/mcp) — this CLI is the heavy-lifter for deterministic bulk jobs.

## Release

Releases are cut automatically on every push to `main`. Version bumping follows the commit message:

| Commit message contains | Bump | Example |
|--------------------------|------|---------|
| `[major-release]` | `vX.0.0` | Breaking changes |
| `[minor-release]` | `vx.Y.0` | New commands / features |
| _(nothing)_ | `vx.y.Z` | Patch (default) |

GitHub Actions builds a matrix of binaries for `linux/darwin/windows × amd64/arm64`, uploads them as release assets, and publishes the GitHub Release.

## Credits

Design patterns, repo layout, and CLI ergonomics are borrowed heavily from [tanq16](https://github.com/tanq16)'s Go CLI projects — in particular the flag/output conventions (`--for-ai`, `--dry-run`, markdown-table output for agents) and the thin service/client/cmd separation.

## License

[MIT](LICENSE) © 5uck1ess
