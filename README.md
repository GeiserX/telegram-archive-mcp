<p align="center">
  <img src="https://raw.githubusercontent.com/GeiserX/telegram-archive-mcp/main/docs/images/banner.svg" alt="Telegram Archive MCP banner" width="900"/>
</p>

<h1 align="center">Telegram-Archive-MCP</h1>

<p align="center">
  <a href="https://www.npmjs.com/package/telegram-archive-mcp"><img src="https://img.shields.io/npm/v/telegram-archive-mcp?style=flat-square&logo=npm" alt="npm"/></a>
  <a href="https://github.com/GeiserX/telegram-archive-mcp/actions/workflows/ci.yml"><img src="https://github.com/GeiserX/telegram-archive-mcp/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://codecov.io/gh/GeiserX/telegram-archive-mcp"><img src="https://codecov.io/gh/GeiserX/telegram-archive-mcp/graph/badge.svg" alt="codecov"/></a>
  <a href="https://hub.docker.com/r/drumsergio/telegram-archive-mcp"><img src="https://img.shields.io/docker/pulls/drumsergio/telegram-archive-mcp?style=flat-square&logo=docker" alt="Docker Pulls"/></a>
  <a href="https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/telegram-archive-mcp?style=flat-square" alt="License"/></a>
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

Set `TELEGRAM_ARCHIVE_URL`, `TELEGRAM_ARCHIVE_USER` and `TELEGRAM_ARCHIVE_PASS` first. Docker Compose and local builds are in [Installation](https://github.com/GeiserX/telegram-archive-mcp/blob/main/docs/installation.md).

## Documentation

- [Installation](https://github.com/GeiserX/telegram-archive-mcp/blob/main/docs/installation.md): Docker Compose, npm, local build
- [Configuration](https://github.com/GeiserX/telegram-archive-mcp/blob/main/docs/configuration.md): environment variables and an example client config
- [Resources and tools](https://github.com/GeiserX/telegram-archive-mcp/blob/main/docs/usage.md)
- [Development](https://github.com/GeiserX/telegram-archive-mcp/blob/main/docs/development.md): testing, contributing, credits
- [Related projects and listings](https://github.com/GeiserX/telegram-archive-mcp/blob/main/docs/related.md)

## License

[GPL-3.0](https://github.com/GeiserX/telegram-archive-mcp/blob/main/LICENSE)
