# Configuration

| Variable                | Default                    | Description                                      |
|-------------------------|----------------------------|--------------------------------------------------|
| `TELEGRAM_ARCHIVE_URL`  | `http://localhost:3000`    | Telegram-Archive instance URL (without trailing /)|
| `TELEGRAM_ARCHIVE_USER` | _(empty)_                  | Login username for session auth via `/api/login` |
| `TELEGRAM_ARCHIVE_PASS` | _(empty)_                  | Login password for session auth via `/api/login` |
| `LISTEN_ADDR`           | `127.0.0.1:8080`           | HTTP listen address (Docker sets `0.0.0.0:8080`) |
| `MCP_AUTH_TOKEN`        | _(empty)_                  | Bearer token for HTTP auth (required if not loopback) |
| `TRANSPORT`             | _(empty = HTTP)_           | Set to `stdio` for stdio transport               |

Put them in a `.env` file (from `.env.example`) or set them in the environment.

## Example configuration for client LLMs

```json
{
  "schema_version": "v1",
  "name_for_human": "Telegram-Archive-MCP",
  "name_for_model": "telegram_archive_mcp",
  "description_for_human": "Search messages, browse chats, and access archived Telegram history.",
  "description_for_model": "Interact with a Telegram-Archive instance that stores archived Telegram messages. First call initialize, then reuse the returned session id in header \"Mcp-Session-Id\" for every other call. Use readResource to fetch URIs that begin with telegram-archive://. Use listTools to discover available actions and callTool to execute them.",
  "auth": {
    "type": "bearer",
    "token": "<your MCP_AUTH_TOKEN value>"
  },
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
