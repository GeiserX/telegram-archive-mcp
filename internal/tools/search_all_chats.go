package tools

import (
	"context"
	"errors"
	"net/http"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var errEmptyQuery = errors.New("query must not be empty")

const searchAllOutputSchema = `{"type":"object","properties":{` +
	`"query":{"type":"string"},"limit":{"type":"integer"},"offset":{"type":"integer"},` +
	`"has_more":{"type":"boolean","description":"True when more hits follow; call again with offset + limit."},` +
	`"indexed":{"type":"boolean","description":"False while the viewer is still building its search index, so hits may be missing."},` +
	`"results":{"type":"array","description":"Hits, newest first: id, date (naive UTC), text, sender_name, topic_title, matched_in, and chat {ref, title, type, ...}. chat.ref is the chat_id other tools take.","items":{"type":"object"}}}}`

func NewSearchAllChats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("search_all_chats",
		mcp.WithDescription("Search message text across every chat this archive login may see, newest first. Matching is by word prefix on the viewer's full-text index. Each hit names its chat, and chat.ref is the chat_id for get_message_context or get_messages. Use search_messages instead to search one known chat. Needs a Telegram Archive viewer with cross-chat search; an older viewer answers 404."),
		describe("Search every chat", true),
		mcp.WithString("query",
			mcp.Required(), mcp.MinLength(1), mcp.MaxLength(500),
			mcp.Description("Words to find (e.g. \"climbing gym address\")."),
		),
		mcp.WithInteger("limit",
			mcp.Min(1), mcp.Max(100), mcp.DefaultNumber(20),
			mcp.Description("Maximum hits to return, 1 to 100 (default 20)."),
		),
		mcp.WithInteger("offset",
			mcp.Min(0), mcp.Max(5000), mcp.DefaultNumber(0),
			mcp.Description("Hits to skip, 0 to 5000 (default 0)."),
		),
		mcp.WithRawOutputSchema([]byte(searchAllOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := queryArg(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := req.GetArguments()
		limit, err := limitArg(args, 20, 100)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		offset, _, err := wholeNumberArg(args, "offset", 5000)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := c.SearchAllMessages(ctx, query, int(limit), int(offset))
		var se *client.StatusError
		if errors.As(err, &se) && se.Status == http.StatusNotFound {
			return mcp.NewToolResultError("this Telegram Archive viewer has no cross-chat search; update it, or use list_chats and search_messages on one chat"), nil
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, ""), nil
	}

	return tool, handler
}
