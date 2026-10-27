# The published image is built by goreleaser (.goreleaser.yaml, "dockers"),
# which puts the binary it has just built and checksummed for the release into
# the build context. So the image runs exactly the released build, with its
# version stamped in, instead of a second compile. To build an image locally:
#   goreleaser release --snapshot --clean
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
# The base image can lag the CA bundle; take the current one, which drops
# the compromised eMudhra (emSign) intermediates.
RUN apk upgrade --no-cache ca-certificates-bundle
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/telegram-archive-mcp"
COPY telegram-archive-mcp /usr/local/bin/telegram-archive-mcp
# Any non-root uid works; 65532 is the conventional "nonroot".
USER 65532:65532
EXPOSE 8080
# stdio by default, as the MCP registry entry declares, so a client can run
#   docker run -i --rm -e TELEGRAM_ARCHIVE_URL=... drumsergio/telegram-archive-mcp:<version>
# For a long-running HTTP server (Docker Compose) set TRANSPORT=http and
# MCP_AUTH_TOKEN: the server listens on 0.0.0.0:8080 inside the container and
# refuses to start without a token on a non-loopback address.
ENV TRANSPORT=stdio
ENV LISTEN_ADDR=0.0.0.0:8080
ENV TELEGRAM_ARCHIVE_URL=http://telegram-viewer:8000
ENTRYPOINT ["/usr/local/bin/telegram-archive-mcp"]
