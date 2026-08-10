# vibecodeway

A desktop app and CLI for managing Jira/Slack search contexts locally.

Save named JQL queries as "contexts", fetch matching issues and their comments into a local SQLite database, and feed the data to coding agents via `vibecodeway-cli`.

## Prerequisites

- **Go 1.22+** — [install](https://go.dev/doc/install)
- **Node.js 18+** — required by Wails for frontend builds ([install](https://nodejs.org/))
- **Wails CLI** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Atlassian CLI (acli)** — required for Jira integration ([install](https://docs.atlassian.com/cli/))

### Jira Authentication

Before using the app or CLI, authenticate with Jira:

1. Ensure `acli` is available in your `PATH`:
   ```bash
   acli --version
   ```
2. Log in to your Jira instance:
   ```bash
   acli jira auth login
   ```
3. Verify authentication:
   ```bash
   acli jira auth status
   ```

The desktop app and CLI require a valid `acli` session. If not authenticated, the GUI shows a warning banner and Jira CLI commands return an error.

## What it does

- **Contexts** — named collections of Jira issues, Slack threads, text notes, and files
- **Refresh** — run the saved JQL again, see which issues are new, select what to add
- **Comments** — full comment history per issue, deduplicated on sync
- **CLI** — JSON in/out for coding agents (`describe` emits the full command schema)

## Binaries

| Binary | Purpose |
|--------|---------|
| `vibecodeway` | Wails desktop app (GUI) |
| `vibecodeway-cli` | CLI for agents and terminal use |

Both share the same SQLite database at `~/.vibecodeway/vibecodeway.db`.

## Getting Started

```bash
# Install frontend dependencies
make setup

# Run the desktop app in dev mode (also installs the CLI)
make dev

# Build the desktop app
make build-desktop

# Build / install the CLI
make build-cli
make install-cli
```

## CLI for coding agents

Every command prints JSON to stdout. Load the machine-readable schema first:

```bash
vibecodeway-cli describe
```

Each entry includes `description`, `input`, `output`, `flags`, and `example`. Prefer `describe` over scraping `--help`.

### Typical agent workflow

1. `vibecodeway-cli context-list` → see available contexts
2. `vibecodeway-cli context-get --context-key my-context` → load full context
3. `vibecodeway-cli jira-search --context-key my-context` → see what's new in Jira
4. `vibecodeway-cli jira-add --context-id ... --jira-keys JH-1,JH-5` → add chosen issues

Slash command `/vibecodeway-prime` loads the schema and routes to the right CLI call.

### Slack passphrase

When Slack integration is used, set `CONTEXT_CONSOLIDATOR_PASSPHRASE` (GUI) / the passphrase the app injects into `slack-cli` via `SLACK_MCP_PASSPHRASE`. If you don't use Slack, you can omit it.
