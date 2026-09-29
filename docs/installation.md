# Installation

## Docker Compose

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

## npm (stdio transport)

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

See [Configuration](configuration.md) for the environment variables.
