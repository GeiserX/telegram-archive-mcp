<p align="center">
  <img src="https://raw.githubusercontent.com/GeiserX/telegram-archive-mcp/main/docs/images/banner.svg" alt="telegram-archive-mcp" width="900"/>
</p>

<h1 align="center">telegram-archive-mcp</h1>

<p align="center">
  <a href="https://www.npmjs.com/package/telegram-archive-mcp"><img src="https://img.shields.io/npm/v/telegram-archive-mcp?style=flat-square&logo=npm" alt="npm"/></a>
  <a href="https://github.com/GeiserX/telegram-archive-mcp/actions/workflows/ci.yml"><img src="https://github.com/GeiserX/telegram-archive-mcp/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/telegram-archive-mcp?style=flat-square" alt="License"/></a>
  <a href="https://hub.docker.com/r/drumsergio/telegram-archive-mcp"><img src="https://img.shields.io/docker/pulls/drumsergio/telegram-archive-mcp?style=flat-square&logo=docker" alt="Docker Pulls"/></a>
  <a href="https://codecov.io/gh/GeiserX/telegram-archive-mcp"><img src="https://codecov.io/gh/GeiserX/telegram-archive-mcp/graph/badge.svg" alt="codecov"/></a>
</p>

<p align="center"><strong>An MCP server for any <a href="https://github.com/GeiserX/Telegram-Archive">Telegram-Archive</a> instance. LLMs use it to search messages, browse chats and read archived Telegram history.</strong></p>

## Features

- Speaks MCP 2026-07-28 and every earlier revision down to 2024-11-05 on stdio and Streamable HTTP. Clients that still use the `initialize` handshake work unchanged.
- Search one chat or every chat at once, read a chat page by page, one calendar day in your timezone, or the conversation around one message.
- Forum topics, pinned messages, folders and per-chat statistics.
- The photo attached to a message, returned as an image the model can look at and scaled to 1568 pixels when larger (`get_media`).
- Every tool is read-only except `refresh_stats`; `READ_ONLY=1` hides that one too.
- Signs in to Telegram-Archive through `/api/login` with `TELEGRAM_ARCHIVE_USER` and `TELEGRAM_ARCHIVE_PASS`.
- HTTP listens on loopback by default, checks `Origin`, caps request bodies, and requires `MCP_AUTH_TOKEN` on any other address.
- Ships as an npm package, a Docker image and multi-arch Go binaries.

## Install

Every client below runs the server over stdio. Set the URL, user and password of your Telegram-Archive viewer.

**Claude Code**

```sh
claude mcp add --env TELEGRAM_ARCHIVE_URL=http://localhost:8000 \
  --env TELEGRAM_ARCHIVE_USER=admin --env TELEGRAM_ARCHIVE_PASS=your-viewer-password \
  --transport stdio telegram-archive -- npx -y telegram-archive-mcp
```

**Claude Desktop** (`claude_desktop_config.json`) and **Cursor** (`~/.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "telegram-archive": {
      "command": "npx",
      "args": ["-y", "telegram-archive-mcp"],
      "env": {
        "TELEGRAM_ARCHIVE_URL": "http://localhost:8000",
        "TELEGRAM_ARCHIVE_USER": "admin",
        "TELEGRAM_ARCHIVE_PASS": "your-viewer-password"
      }
    }
  }
}
```

**VS Code** (`.vscode/mcp.json`, asks for the password once)

```json
{
  "inputs": [
    { "type": "promptString", "id": "tg-pass", "description": "Telegram-Archive viewer password", "password": true }
  ],
  "servers": {
    "telegram-archive": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "telegram-archive-mcp"],
      "env": {
        "TELEGRAM_ARCHIVE_URL": "http://localhost:8000",
        "TELEGRAM_ARCHIVE_USER": "admin",
        "TELEGRAM_ARCHIVE_PASS": "${input:tg-pass}"
      }
    }
  }
}
```

**Codex** (`~/.codex/config.toml`)

```toml
[mcp_servers.telegram-archive]
command = "npx"
args = ["-y", "telegram-archive-mcp"]
env = { TELEGRAM_ARCHIVE_URL = "http://localhost:8000", TELEGRAM_ARCHIVE_USER = "admin", TELEGRAM_ARCHIVE_PASS = "your-viewer-password" }
```

**Docker**, for any client above: use `docker` as the command with these arguments, and a release tag from the [releases page](https://github.com/GeiserX/telegram-archive-mcp/releases), never `latest`:

```sh
docker run -i --rm -e TELEGRAM_ARCHIVE_URL -e TELEGRAM_ARCHIVE_USER -e TELEGRAM_ARCHIVE_PASS drumsergio/telegram-archive-mcp:vX.Y.Z
```

The npm package downloads the release binary on first start and checks it against the checksums published with the package, so it also works with npm 12, which skips install scripts. A long-running HTTP server (Docker Compose) and local builds are in [Getting started](https://geiserx.github.io/telegram-archive-mcp/getting-started/).

## Tools

| Tool | What it does |
|---|---|
| `list_chats` | Lists chats, most recent first; filter by `search` (name), `archived` or `folder_id`. Each chat's `ref` is the `chat_id` the other tools take. |
| `list_folders` | Lists your Telegram folders and their ids. |
| `search_all_chats` | Searches message text in every chat at once. |
| `search_messages` | Searches one chat, optionally one forum topic. |
| `get_messages` | Reads a chat newest first, by offset or keyset cursor, optionally one forum topic. |
| `get_message_context` | Returns one message with the messages before and after it. |
| `get_messages_by_date` | Returns every message of one calendar day in a timezone. |
| `get_pinned_messages` | Returns a chat's pinned messages. |
| `get_topics` | Lists a forum group's topics; their ids feed `topic_id`. |
| `get_chat_stats` | Message and media counts and size of one chat. |
| `get_media` | The photo attached to a message, as an image, or the file's details. |
| `refresh_stats` | Makes the viewer recalculate archive-wide statistics. Hidden with `READ_ONLY=1`. |

Every tool returns structured content with an output schema, alongside the same JSON as text. Arguments and defaults are on [Usage](https://geiserx.github.io/telegram-archive-mcp/usage/).

## Documentation

The full documentation is at [geiserx.github.io/telegram-archive-mcp](https://geiserx.github.io/telegram-archive-mcp/).

- [Getting started](https://geiserx.github.io/telegram-archive-mcp/getting-started/): npm, Docker Compose, local build
- [Configuration](https://geiserx.github.io/telegram-archive-mcp/configuration/): environment variables and client configuration
- [Usage](https://geiserx.github.io/telegram-archive-mcp/usage/): the 4 resources and 12 tools, paging, reading one day at a time
- [Development](https://geiserx.github.io/telegram-archive-mcp/development/): testing, contributing, credits
- [Related projects](https://geiserx.github.io/telegram-archive-mcp/related/): Telegram Archive, other MCP servers, where this one is listed

## License

[GPL-3.0-or-later](https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE)
