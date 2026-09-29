# Configuration

| Variable                | Default                    | Description                                      |
|-------------------------|----------------------------|--------------------------------------------------|
| `TELEGRAM_ARCHIVE_URL`  | `http://localhost:8000`    | Telegram-Archive instance URL (without trailing /)|
| `TELEGRAM_ARCHIVE_USER` | _(empty)_                  | Login username for session auth via `/api/login` |
| `TELEGRAM_ARCHIVE_PASS` | _(empty)_                  | Login password for session auth via `/api/login` |
| `LISTEN_ADDR`           | `127.0.0.1:8080`           | HTTP listen address (Docker sets `0.0.0.0:8080`) |
| `MCP_AUTH_TOKEN`        | _(empty)_                  | Bearer token for HTTP auth (required if not loopback) |
| `TRANSPORT`             | _(empty = HTTP)_           | Set to `stdio` for stdio transport               |

Put them in a `.env` file (from `.env.example`) or set them in the environment.

## Client configuration

With the npm package, the client starts the server itself and talks to it over stdio. For Claude Desktop, Cursor or Claude Code:

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

With the Docker image, the server runs on its own and listens for HTTP on `/mcp`. Point a client that supports HTTP servers at it, and send `MCP_AUTH_TOKEN` as a bearer token when you set one:

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
