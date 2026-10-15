# Usage

Every resource and tool below reads your Telegram Archive viewer through its HTTP API, signed in once: the server keeps the session cookie and logs in again only when the viewer answers 401, so many calls cost one login against the viewer's limit of 15 logins per client IP in 5 minutes. Only `refresh_stats` asks the viewer to do something: it recalculates the archive's statistics. None of them changes a message, and none of them talks to Telegram.

## Resources

| URI | What it returns |
|---|---|
| `telegram-archive://stats` | Chat count, message count, media count and total size of the archive |
| `telegram-archive://chats` | Every archived chat with basic info |
| `telegram-archive://folders` | Your Telegram chat folders |
| `telegram-archive://health` | Whether the viewer is up |

## Tools

Most tools take a `chat_id`. Call `list_chats` first: the `ref` of each chat is its `chat_id`.

| Tool | What it does | Arguments |
|---|---|---|
| `list_chats` | Lists archived chats, most recent first. Each row has `ref` (the `chat_id` the other tools take), `id` (the numeric Telegram chat id), `type`, `accounts` (the archive accounts holding it) and its title or name | `limit` (default 100, max 500), `offset` |
| `list_folders` | Lists your Telegram chat folders | none |
| `search_messages` | Finds messages in one chat that contain a keyword | `chat_id`, `query`, `limit` (default 20) |
| `get_messages` | Returns a chat's messages, newest first | `chat_id`, `limit` (default 50, max 500), then either `offset`, or the keyset cursor `before_date` plus `before_id`, or `after_id` |
| `get_messages_by_date` | Returns every message of one calendar day, oldest first | `chat_id`, `date` (`YYYY-MM-DD`), `timezone` (IANA name, default UTC), `limit` (default 1000, max 5000) |
| `get_pinned_messages` | Returns a chat's pinned messages | `chat_id` |
| `get_topics` | Lists the forum topics of a chat | `chat_id` |
| `get_chat_stats` | Returns statistics for one chat | `chat_id` |
| `get_media` | Returns the photo or file attached to one message: an image to look at, or the file's name, type and size | `chat_id`, `message_id` |
| `refresh_stats` | Makes the viewer recalculate the archive's statistics | none |

### Paging through a long chat

`get_messages` pages two ways. `offset` skips that many messages, which gets slower the deeper you go. The keyset cursor does not: pass the `date` and `id` of the last message you received as `before_date` and `before_id`, and the next call returns the messages older than it. `after_id` returns the messages newer than an id. Dates are ISO 8601; a date without a timezone is UTC, which is how the archive stores them.

### Looking at a photo

`get_media` takes the `chat_id` and the `id` of a message from any of the message tools. Its first content block is always a JSON text block:

```json
{"message_id": 42, "media_key": "42_photo", "kind": "photo", "file_name": "1234567890_photo.jpg", "mime_type": "image/jpeg", "size_bytes": 36129, "image_returned": true}
```

- A JPEG, PNG, WebP or GIF of up to 3,932,160 bytes (3.75 MB) follows as an MCP `image` block, so the model sees it. At that size the base64 is 5 MB, the Claude API's limit for one image. The type comes from the file's bytes, not from the archive's metadata.
- Any other file, and a larger image, gets the text block only, with `image_returned: false` and a `note` saying why. The file itself is never sent, and an image is never cut short.
- The location map picture the archive keeps for a `geo` or `venue` message comes back as an image too.
- The call fails (`isError`) with a sentence saying why when the message does not exist in that chat, has no media, has a kind with no file (a contact, a poll, a location without a map), has a file the archive has not downloaded, or when the viewer login has downloads turned off.

### One day at a time

`get_messages_by_date` takes the day in the timezone you name, so `2026-06-10` in `Europe/Madrid` starts at 22:00 UTC the evening before. When the day holds more messages than `limit`, the newest are dropped and the result says `truncated: true`.

## On the wire

Over HTTP the server has one JSON-RPC endpoint, `/mcp`; over stdio the same messages go through the process's standard input and output. A client calls `initialize`, then `resources/list` and `resources/read`, `tools/list` and `tools/call`. Any MCP client does this for you; [MCP Inspector](https://modelcontextprotocol.io/docs/tools/inspector) shows each call if you want to watch.
