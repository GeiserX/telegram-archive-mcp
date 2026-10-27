package tools

import (
	"context"
	"strings"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// queryArg reads the required query argument and refuses a blank one: the
// viewer treats an empty search as no search and returns the newest messages.
func queryArg(req mcp.CallToolRequest) (string, error) {
	q, err := req.RequireString("query")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(q) == "" {
		return "", errEmptyQuery
	}
	return q, nil
}

func NewSearchMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("search_messages",
		mcp.WithDescription("Search the messages of one chat for a keyword, newest first, and return the matching messages with the same fields as get_messages. Use search_all_chats when you do not know the chat. To read the conversation around a hit, pass its id to get_message_context."),
		describe("Search one chat", true),
		chatIDParam(),
		mcp.WithString("query",
			mcp.Required(), mcp.MinLength(1), mcp.MaxLength(500),
			mcp.Description("Text to find in the message text (e.g. \"invoice\")."),
		),
		mcp.WithInteger("limit",
			mcp.Min(1), mcp.Max(200), mcp.DefaultNumber(20),
			mcp.Description("Maximum messages to return, 1 to 200 (default 20)."),
		),
		topicIDParam(),
		mcp.WithRawOutputSchema([]byte(messagesOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		query, err := queryArg(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := req.GetArguments()
		limit, err := limitArg(args, 20, 200)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		topicID, err := topicIDArg(args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := c.ListMessages(ctx, chatID, client.MessagesQuery{Search: query, Limit: int(limit), TopicID: topicID})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, "messages"), nil
	}

	return tool, handler
}
