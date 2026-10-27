# Configuration

| Variable                | Default                    | Description                                      |
|-------------------------|----------------------------|--------------------------------------------------|
| `TELEGRAM_ARCHIVE_URL`  | `http://localhost:8000`    | Telegram-Archive viewer URL (without trailing /). The server warns at startup when it is plain `http` to a public host, because the password would cross the network in cleartext. |
| `TELEGRAM_ARCHIVE_USER` | _(empty)_                  | Login username for session auth via `/api/login` |
| `TELEGRAM_ARCHIVE_PASS` | _(empty)_                  | Login password for session auth via `/api/login` |
| `READ_ONLY`             | `false`                    | A true value hides `refresh_stats`, the one tool that asks the viewer to do work. `1`, `t`, `true` (or `T`, `TRUE`, `True`) hide it; `0`, `f`, `false` (or `F`, `FALSE`, `False`) or unset keep it. Anything else stops the server, so a typo never leaves it on. |
| `TRANSPORT`             | `http` (`stdio` in Docker and npm) | `stdio` or `http` |
| `LISTEN_ADDR`           | `127.0.0.1:8080` (`0.0.0.0:8080` in Docker) | HTTP listen address |
| `MCP_AUTH_TOKEN`        | _(empty)_                  | Bearer token for HTTP. Required when `LISTEN_ADDR` is not loopback (`:8080` binds every interface, so it needs one too); clients send `Authorization: Bearer <token>`. |
| `MCP_ALLOWED_ORIGINS`   | _(empty)_                  | Comma-separated browser origins (e.g. `https://inspector.example`) allowed to call `/mcp` cross-origin. Requests without an `Origin` header, which is every non-browser client, are not affected. |

Put them in a `.env` file (from `.env.example`) or set them in the environment.

## Client configuration

With the npm package, the client starts the server itself and talks to it over stdio. For Claude Desktop or Cursor (the [README](https://github.com/GeiserX/telegram-archive-mcp#install) has Claude Code, VS Code, Codex and Docker too):

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

Run as an HTTP server (`TRANSPORT=http`, see [Getting started](getting-started.md#docker-compose-http)), it listens on `/mcp`. Point a client that supports HTTP servers at it and send `MCP_AUTH_TOKEN` as a bearer token:

```json
{
  "mcpServers": {
    "telegram-archive": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": { "Authorization": "Bearer your-mcp-auth-token" }
    }
  }
}
```

## Protocol versions

The server speaks MCP 2026-07-28 and every earlier revision down to 2024-11-05, on stdio and on HTTP. A client that opens with `initialize` gets the 2025-11-25 behaviour; a 2026-07-28 client calls `server/discover` and sends its version with each request. Nothing needs configuring for either.

## HTTP hardening

- `Origin` is checked on every request: a browser page from another site gets 403 unless its origin is in `MCP_ALLOWED_ORIGINS`.
- Request bodies over 4 MiB are refused, and slow or idle connections are closed (10 s for headers, 60 s for the request, 120 s idle).
- A request that arrives over the loopback interface must name the loopback in `Host` (`localhost`, `127.0.0.1` or `[::1]`), which stops a DNS-rebinding page from reaching the server. A reverse proxy on the same machine should set `Host` to `localhost` when it forwards.
- The endpoint is stateless: it never issues an `Mcp-Session-Id`.
- `GET /mcp` answers 405. The server pushes no notifications, so there is no event stream to open.
