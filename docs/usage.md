# Resources and tools

| Type          | What for                                                       | MCP URI / Tool id                |
|---------------|----------------------------------------------------------------|----------------------------------|
| **Resources** | Browse archive stats, chats, and folders read-only             | `telegram-archive://stats`<br>`telegram-archive://chats`<br>`telegram-archive://folders`<br>`telegram-archive://health` |
| **Tools**     | Search and retrieve messages (offset or keyset cursor paging; whole calendar days in a timezone), inspect chat statistics | `search_messages`<br>`get_messages`<br>`get_pinned_messages`<br>`get_messages_by_date`<br>`get_chat_stats`<br>`get_topics`<br>`refresh_stats` |

Everything is exposed over a single JSON-RPC endpoint (`/mcp`).
LLMs / Agents can: `initialize` -> `readResource` -> `listTools` -> `callTool` ... and so on.
