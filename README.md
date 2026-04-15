# context-consolidator

A desktop app and MCP server for managing Jira search contexts locally.

Save named JQL queries as "contexts", fetch matching issues and their comments into a local SQLite database, and feed the data to AI tools via MCP.

## Prerequisites

- **Go 1.22+** — [install](https://go.dev/doc/install)
- **Node.js 18+** — required by Wails for frontend builds ([install](https://nodejs.org/))
- **Wails CLI** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Atlassian CLI (acli)** — required for Jira integration ([install](https://docs.atlassian.com/cli/))

### Jira Authentication

Before using the app or MCP server, authenticate with Jira:

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

Both the desktop app and the MCP server require a valid `acli` session. If not authenticated, the GUI shows a warning banner and the MCP server returns an error on tool calls that hit Jira.

## What it does

- **Contexts** — named collections of Jira issues, each tied to a JQL query
- **Refresh** — run the saved JQL again, see which issues are new, select what to add
- **Comments** — full comment history per issue, deduplicated on sync
- **MCP server** — lets AI tools (Cursor, etc.) query contexts and trigger syncs via stdio

## Binaries

| Binary | Purpose |
|--------|---------|
| `context-consolidator` | Wails desktop app (GUI) |
| `context-consolidator-mcp` | stdio MCP server for AI clients |

Both share the same SQLite database at `~/.context-consolidator/context_consolidator.db`.

## Getting Started

```bash
# Install frontend dependencies
make setup

# Run the desktop app in dev mode
make dev

# Build the desktop app
make build-desktop

# Build the MCP server
make build-mcp
```

## MCP Setup (Cursor)

Add to your Cursor `mcp.json`:

```json
{
  "mcpServers": {
    "context-consolidator": {
      "type": "stdio",
      "command": "/path/to/bin/context-consolidator-mcp"
    }
  }
}
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `context_list` | List all saved contexts |
| `context_search` | Run JQL and get diff of new vs existing issues |
| `context_add` | Add selected issues + sync comments |
| `context_get` | Get full context with all issues and comments |

### Typical AI workflow

1. `context_list` → see available contexts
2. `context_search(context_key="my-context")` → see what's new in Jira
3. `context_add(context_id="...", jira_keys="JH-1,JH-5")` → add chosen issues
4. `context_get(context_key="my-context")` → read all data for analysis
