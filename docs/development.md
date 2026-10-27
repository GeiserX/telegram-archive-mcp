# Development

## Testing

```bash
go build -o telegram-archive-mcp ./cmd/server
go test ./...
node --test
```

- `cmd/server/protocol_test.go` drives the real server over stdio, once as a 2026-07-28 client (`server/discover`, no handshake) and once as a legacy client (`initialize`, then `tools/list`).
- `cmd/server/http_test.go` covers the HTTP endpoint: both eras, the bearer token, the Origin check and the body cap.
- `cmd/server/tools_contract_test.go` calls every tool against a fake viewer and checks its structured output against its `outputSchema`.
- `postinstall.test.js` covers the npm installer's URL and checksum checks.

In a checkout, run `npm install --ignore-scripts` if you need the Node dependencies. The postinstall step downloads a release binary and checks it against `checksums.txt`, which is only bundled when the package is published, so without the flag it stops with "checksums.txt is not bundled".

For a manual check, [MCP Inspector](https://modelcontextprotocol.io/docs/tools/inspector) connects over stdio or HTTP and shows each call.

## Contributing

Feel free to dive in! [Open an issue](https://github.com/GeiserX/telegram-archive-mcp/issues/new) or submit PRs.

Telegram-Archive-MCP follows the [Contributor Covenant](http://contributor-covenant.org/version/2/1/) Code of Conduct.

## Maintainers

[@GeiserX](https://github.com/GeiserX).

## Credits

[Telegram-Archive](https://github.com/GeiserX/Telegram-Archive) -- Telegram message archival and search

[MCP-GO](https://github.com/mark3labs/mcp-go) -- modern MCP implementation

[GoReleaser](https://goreleaser.com/) -- painless multi-arch releases
