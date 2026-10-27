# Usage

Every resource and tool below reads your Telegram Archive viewer through its HTTP API, signed in once: the server keeps the session cookie and logs in again only when the viewer answers 401, so many calls cost one login against the viewer's limit of 15 logins per client IP in 5 minutes. Only `refresh_stats` asks the viewer to do something: it recalculates the archive's statistics. None of them changes a message, and none of them talks to Telegram.

## Resources

| URI | What it returns |
|---|---|
| `telegram-archive://stats` | Chat count, message count, media count and total size of the archive |
| `telegram-archive://chats` | The 500 most recent chats with basic info (`list_chats` pages further) |
| `telegram-archive://folders` | Your Telegram chat folders |
| `telegram-archive://health` | Whether the viewer is up |

## Tools

Most tools take a `chat_id`. Call `list_chats` first: the `ref` of each chat is its `chat_id`.

| Tool | What it does | Arguments |
|---|---|---|
| `list_chats` | Lists archived chats, most recent first. Each row has `ref` (the `chat_id` the other tools take), `id` (the numeric Telegram chat id), `type`, `accounts` (the archive accounts holding it) and its title or name | `limit` (1 to 500, default 100), `offset`, `search` (part of a chat's name or username), `archived` (true or false), `folder_id` (from `list_folders`) |
| `list_folders` | Lists your Telegram chat folders and their ids | none |
| `search_all_chats` | Finds messages that contain words in every chat the login may see, newest first; each hit names its chat by `ref`. Needs a viewer with cross-chat search | `query`, `limit` (1 to 100, default 20), `offset` (0 to 5000) |
| `search_messages` | Finds messages in one chat that contain a keyword | `chat_id`, `query`, `limit` (1 to 200, default 20), `topic_id` |
| `get_messages` | Returns a chat's messages, newest first | `chat_id`, `limit` (1 to 500, default 50), then either `offset` (up to 100000), or the keyset cursor `before_date` plus `before_id`, or `after_id`; `topic_id` |
| `get_message_context` | Returns one message with the messages around it, oldest first | `chat_id`, `message_id`, `before` and `after` (0 to 50 each, default 5) |
| `get_messages_by_date` | Returns every message of one calendar day, oldest first | `chat_id`, `date` (`YYYY-MM-DD`), `timezone` (IANA name, default UTC), `limit` (1 to 5000, default 1000), `topic_id` |
| `get_pinned_messages` | Returns a chat's pinned messages | `chat_id` |
| `get_topics` | Lists the forum topics of a chat, with the ids `topic_id` takes | `chat_id` |
| `get_chat_stats` | Returns message and media counts, size and first and last message dates for one chat | `chat_id` |
| `get_media` | Returns the photo or file attached to one message: an image to look at, or the file's name, type and size | `chat_id`, `message_id` |
| `refresh_stats` | Makes the viewer recalculate the archive's statistics. Needs the viewer's master login; hidden with `READ_ONLY=1` | none |

Arguments are checked against each tool's schema before it runs: an unknown argument, a fraction where a whole number is due, or a value out of range comes back as an error the model can correct. Every tool answers with `structuredContent` matching its `outputSchema`, and the same data as JSON text for clients that read only text. A list the viewer returns as a bare array (messages, pinned messages) is wrapped as `{"messages": [...]}` in `structuredContent`.

Errors from the viewer come back as one fixed sentence per HTTP status (for example `telegram-archive error 404: not found; check chat_id ...`), never the viewer's own error text.

### Forum topics

In a forum supergroup, `get_topics` lists the threads. Pass a topic's `id` as `topic_id` to `get_messages`, `search_messages` or `get_messages_by_date` to read only that thread; `1` is the General topic.

### Around one message

After a search hit, or to see what a `reply_to_msg_id` points at, `get_message_context` returns that message with up to `before` older and `after` newer messages, oldest first, plus how many of each it found.

### Paging through a long chat

`get_messages` pages two ways. `offset` skips that many messages, which gets slower the deeper you go. The keyset cursor does not: pass the `date` and `id` of the last message you received as `before_date` and `before_id`, and the next call returns the messages older than it. `after_id` returns the messages newer than an id. Dates are ISO 8601; a date without a timezone is UTC, which is how the archive stores them.

### Looking at a photo

Every message row from `get_messages`, `get_messages_by_date` and `search_messages` carries `reply_to_msg_id`, the id of the message it answers, and `media` when it has a file or other media: `media.type` is the kind (`photo`, `document`, `video`, `geo`...) and `media.url` is set when the archive holds a file `get_media` can fetch. So from a reply such as "did you see my picture?" an agent reads `reply_to_msg_id` and passes it to `get_media`.

`get_media` takes the `chat_id` and the `id` of a message. Its first content block is always a JSON text block:

```json
{"message_id": 42, "date": "2026-06-10T18:04:17", "sender": "Ada Lovelace", "text": "our new kitchen", "reply_to_msg_id": null, "media_key": "42_photo", "kind": "photo", "file_name": "1234567890_photo.jpg", "mime_type": "image/jpeg", "size_bytes": 412345, "image_returned": true, "width": 1568, "height": 1176, "scaled": true}
```

`date`, `sender`, `text` (the caption) and `reply_to_msg_id` describe the message. `file_name`, `mime_type` and `size_bytes` describe the original file; `width`, `height` and `scaled` describe the image sent.

- A JPEG, PNG, WebP or GIF follows as an MCP `image` block, so the model sees it. When it is at most 1568 pixels on its longest side and at most 2,000,000 bytes it is sent as is. Otherwise it is sent as a JPEG scaled to 1568 pixels on its longest side, with `scaled: true`; the Claude API scales larger images down to that size anyway. Transparent areas turn white, and a JPEG stored sideways with an EXIF orientation tag is turned upright (and so always re-encoded). The type comes from the file's bytes, not from the archive's metadata.
- Any other file gets the text block only, with `image_returned: false` and a `note` saying why. So does an image whose file is over 20 MB or whose header claims more than 25 million pixels. The file itself is never sent, and an image is never cut short.
- The location map picture the archive keeps for a `geo` or `venue` message comes back as an image too.
- The call fails (`isError`) with a sentence saying why when the message does not exist in that chat, has no media, has a kind with no file (a contact, a poll, a location without a map), has a file the archive has not downloaded, or when the viewer login has downloads turned off.

### One day at a time

`get_messages_by_date` takes the day in the timezone you name, so `2026-06-10` in `Europe/Madrid` starts at 22:00 UTC the evening before. When the day holds more messages than `limit`, the newest are dropped and the result says `truncated: true`.

## On the wire

Over HTTP the server has one JSON-RPC endpoint, `/mcp`; over stdio the same messages go through the process's standard input and output. It speaks MCP 2026-07-28 and every earlier revision down to 2024-11-05:

- A client on 2025-11-25 or earlier calls `initialize`, then `resources/list` and `resources/read`, `tools/list` and `tools/call`.
- A 2026-07-28 client skips the handshake: it calls `server/discover` and sends its protocol version in each request's `_meta`. Tool and resource lists then carry cache hints (`ttlMs` 300000, `cacheScope` public); resource reads are `private` with no caching.

Any MCP client does this for you; [MCP Inspector](https://modelcontextprotocol.io/docs/tools/inspector) shows each call if you want to watch.
