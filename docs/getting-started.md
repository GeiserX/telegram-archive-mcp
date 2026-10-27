# Getting started

## Docker (stdio)

The image runs the server over stdio by default, so any MCP client can start it like a local binary. Use a release tag from the [releases page](https://github.com/GeiserX/telegram-archive-mcp/releases), never `latest`:

```sh
docker run -i --rm \
  -e TELEGRAM_ARCHIVE_URL=http://host.docker.internal:8000 \
  -e TELEGRAM_ARCHIVE_USER=your-username -e TELEGRAM_ARCHIVE_PASS=your-password \
  drumsergio/telegram-archive-mcp:vX.Y.Z
```

Inside the container `localhost` is the container itself; reach a viewer on the host through `host.docker.internal` (Docker Desktop) or the host's address.

## Docker Compose (HTTP)

To run it next to the viewer as a long-running HTTP server, switch the transport to HTTP and set a token. Inside the container it listens on `0.0.0.0:8080`, and it refuses to start on that address without `MCP_AUTH_TOKEN`:

```yaml
services:
  telegram-archive-mcp:
    image: drumsergio/telegram-archive-mcp:vX.Y.Z
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - TRANSPORT=http
      - MCP_AUTH_TOKEN=a-long-random-token   # clients send it as "Authorization: Bearer ..."
      - TELEGRAM_ARCHIVE_URL=http://telegram-viewer:8000   # the Telegram-Archive viewer, on the same Docker network
      - TELEGRAM_ARCHIVE_USER=your-username
      - TELEGRAM_ARCHIVE_PASS=your-password
      - READ_ONLY=1   # optional: hide refresh_stats
```

The container runs as an unprivileged user (uid 65532). Images before the 2026-07-28 upgrade started HTTP on the container's loopback, which no port mapping can reach; with those, set `LISTEN_ADDR=0.0.0.0:8080` as well.

## npm (stdio transport)

```sh
npx telegram-archive-mcp
```

Or install globally:

```sh
npm install -g telegram-archive-mcp
telegram-archive-mcp
```

This downloads the pre-built Go binary from GitHub Releases for your platform, checks its SHA-256 against the `checksums.txt` published inside the npm package, and runs it with stdio transport. The download happens on install, or on first start when the package manager skipped install scripts (npm 12, pnpm, Bun do by default).

## Local build

```sh
git clone https://github.com/GeiserX/telegram-archive-mcp
cd telegram-archive-mcp

# (optional) create .env from the sample
cp .env.example .env && $EDITOR .env

go run ./cmd/server
```

See [Configuration](configuration.md) for the environment variables.
