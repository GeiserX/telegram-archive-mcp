FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/telegram-archive-mcp ./cmd/server

FROM alpine:3.23
COPY --from=builder /out/telegram-archive-mcp /usr/local/bin/telegram-archive-mcp
EXPOSE 8080
ENV TELEGRAM_ARCHIVE_URL=http://telegram-archive:3000
ENV TELEGRAM_ARCHIVE_USER=""
ENV TELEGRAM_ARCHIVE_PASS=""
ENTRYPOINT ["/usr/local/bin/telegram-archive-mcp"]
