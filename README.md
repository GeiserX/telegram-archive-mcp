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

- Read-only resources for archive stats, chats, folders and health (`telegram-archive://stats`, `telegram-archive://chats`).
- Message search, message paging by offset or keyset cursor, and whole calendar days in a timezone (`get_messages_by_date`).
- Pinned messages, forum topics and per-chat statistics.
- Signs in to Telegram-Archive through `/api/login` with `TELEGRAM_ARCHIVE_USER` and `TELEGRAM_ARCHIVE_PASS`.
- One JSON-RPC endpoint (`/mcp`) over HTTP, or stdio with `TRANSPORT=stdio`.
- Listens on loopback by default; `MCP_AUTH_TOKEN` adds bearer auth when you expose it.
- Ships as a Docker image, an npm package (`npx telegram-archive-mcp`) and multi-arch Go binaries.

## Quick start

```sh
npx telegram-archive-mcp
```

`npx` fetches the server and runs it on stdio, so your MCP client starts it for you. For Claude Desktop, Cursor or Claude Code, add:

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

Set the URL, user and password of your Telegram-Archive viewer. Docker Compose (HTTP on `127.0.0.1:8080`) and local builds are in [Getting started](https://geiserx.github.io/telegram-archive-mcp/getting-started/).

## Documentation

The full documentation is at [geiserx.github.io/telegram-archive-mcp](https://geiserx.github.io/telegram-archive-mcp/).

- [Getting started](https://geiserx.github.io/telegram-archive-mcp/getting-started/): npm, Docker Compose, local build
- [Configuration](https://geiserx.github.io/telegram-archive-mcp/configuration/): environment variables and client configuration
- [Usage](https://geiserx.github.io/telegram-archive-mcp/usage/): the 4 resources and 9 tools, paging, reading one day at a time
- [Development](https://geiserx.github.io/telegram-archive-mcp/development/): testing, contributing, credits
- [Related projects](https://geiserx.github.io/telegram-archive-mcp/related/): Telegram Archive, other MCP servers, where this one is listed

## License

[GPL-3.0-or-later](https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE)
