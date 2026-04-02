<h1 align="center">Telegram-Archive-MCP</h1>

<p align="center">
  <a href="https://www.npmjs.com/package/telegram-archive-mcp"><img src="https://img.shields.io/npm/v/telegram-archive-mcp?style=flat-square&logo=npm" alt="npm"/></a>
  <img src="https://img.shields.io/badge/Go-1.24-blue?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <a href="https://hub.docker.com/r/drumsergio/telegram-archive-mcp"><img src="https://img.shields.io/docker/pulls/drumsergio/telegram-archive-mcp?style=flat-square&logo=docker" alt="Docker Pulls"/></a>
  <a href="https://github.com/GeiserX/telegram-archive-mcp/stargazers"><img src="https://img.shields.io/github/stars/GeiserX/telegram-archive-mcp?style=flat-square&logo=github" alt="GitHub Stars"/></a>
  <a href="https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/telegram-archive-mcp?style=flat-square" alt="License"/></a>
</p>
<p align="center">
  <a href="https://glama.ai/mcp/servers/GeiserX/telegram-archive-mcp"><img src="https://glama.ai/mcp/servers/GeiserX/telegram-archive-mcp/badges/score.svg" alt="Glama MCP Server" /></a>
</p>

<p align="center"><strong>A tiny bridge that exposes any Telegram-Archive instance as an MCP server, enabling LLMs to search messages, browse chats, and access archived Telegram history.</strong></p>

---

## What you get

| Type          | What for                                                       | MCP URI / Tool id                |
|---------------|----------------------------------------------------------------|----------------------------------|
| **Resources** | Browse archive stats, chats, and folders read-only             | `telegram-archive://stats`<br>`telegram-archive://chats`<br>`telegram-archive://folders`<br>`telegram-archive://health` |
| **Tools**     | Search and retrieve messages, inspect chat statistics           | `search_messages`<br>`get_messages`<br>`get_pinned_messages`<br>`get_messages_by_date`<br>`get_chat_stats`<br>`get_topics`<br>`refresh_stats` |

Everything is exposed over a single JSON-RPC endpoint (`/mcp`).
LLMs / Agents can: `initialize` -> `readResource` -> `listTools` -> `callTool` ... and so on.

---

## Quick-start (Docker Compose)

```yaml
services:
  telegram-archive-mcp:
    image: drumsergio/telegram-archive-mcp:latest
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - TELEGRAM_ARCHIVE_URL=http://telegram-archive:3000
      - TELEGRAM_ARCHIVE_USER=your-username
      - TELEGRAM_ARCHIVE_PASS=your-password
```

> **Security note:** The HTTP transport listens on `127.0.0.1:8080` by default. If you need to expose it on a network, place it behind a reverse proxy with authentication.

## Install via npm (stdio transport)

```sh
npx telegram-archive-mcp
```

Or install globally:

```sh
npm install -g telegram-archive-mcp
telegram-archive-mcp
```

This downloads the pre-built Go binary from GitHub Releases for your platform and runs it with stdio transport. Requires at least one [published release](https://github.com/GeiserX/telegram-archive-mcp/releases).

## Local build

```sh
git clone https://github.com/GeiserX/telegram-archive-mcp
cd telegram-archive-mcp

# (optional) create .env from the sample
cp .env.example .env && $EDITOR .env

go run ./cmd/server
```

## Configuration

| Variable                | Default                    | Description                                      |
|-------------------------|----------------------------|--------------------------------------------------|
| `TELEGRAM_ARCHIVE_URL`  | `http://localhost:3000`    | Telegram-Archive instance URL (without trailing /)|
| `TELEGRAM_ARCHIVE_USER` | _(empty)_                  | Login username for session auth via `/api/login` |
| `TELEGRAM_ARCHIVE_PASS` | _(empty)_                  | Login password for session auth via `/api/login` |
| `LISTEN_ADDR`           | `127.0.0.1:8080`           | HTTP listen address (Docker sets `0.0.0.0:8080`) |
| `TRANSPORT`             | _(empty = HTTP)_           | Set to `stdio` for stdio transport               |

Put them in a `.env` file (from `.env.example`) or set them in the environment.

## Testing

Tested with [Inspector](https://modelcontextprotocol.io/docs/tools/inspector) and it is currently fully working. Before making a PR, make sure this MCP server behaves well via this medium.

## Example configuration for client LLMs

```json
{
  "schema_version": "v1",
  "name_for_human": "Telegram-Archive-MCP",
  "name_for_model": "telegram_archive_mcp",
  "description_for_human": "Search messages, browse chats, and access archived Telegram history.",
  "description_for_model": "Interact with a Telegram-Archive instance that stores archived Telegram messages. First call initialize, then reuse the returned session id in header \"Mcp-Session-Id\" for every other call. Use readResource to fetch URIs that begin with telegram-archive://. Use listTools to discover available actions and callTool to execute them.",
  "auth": { "type": "none" },
  "api": {
    "type": "jsonrpc-mcp",
    "url":  "http://localhost:8080/mcp",
    "init_method": "initialize",
    "session_header": "Mcp-Session-Id"
  },
  "contact_email": "acsdesk@protonmail.com",
  "legal_info_url": "https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE"
}
```

## Credits

[Telegram-Archive](https://github.com/nicmart-dev/telegram-archive) -- Telegram message archival and search

[MCP-GO](https://github.com/mark3labs/mcp-go) -- modern MCP implementation

[GoReleaser](https://goreleaser.com/) -- painless multi-arch releases

## Maintainers

[@GeiserX](https://github.com/GeiserX).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/GeiserX/telegram-archive-mcp/issues/new) or submit PRs.

Telegram-Archive-MCP follows the [Contributor Covenant](http://contributor-covenant.org/version/2/1/) Code of Conduct.

## Other MCP Servers by GeiserX

- [cashpilot-mcp](https://github.com/GeiserX/cashpilot-mcp) — Passive income monitoring
- [duplicacy-mcp](https://github.com/GeiserX/duplicacy-mcp) — Backup health monitoring
- [genieacs-mcp](https://github.com/GeiserX/genieacs-mcp) — TR-069 device management
- [lynxprompt-mcp](https://github.com/GeiserX/lynxprompt-mcp) — AI configuration blueprints
- [pumperly-mcp](https://github.com/GeiserX/pumperly-mcp) — Fuel and EV charging prices
