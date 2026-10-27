package tools

import (
	"context"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const topicsOutputSchema = `{"type":"object","properties":{"topics":{"type":"array","description":"One row per forum topic: id (the topic_id other tools take), title, message_count, last_message_date, is_closed, is_pinned.","items":{"type":"object"}}}}`

func NewGetTopics(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_topics",
		mcp.WithDescription("List the forum topics (threads) of a forum supergroup, with message counts. Pass a topic's id as topic_id to get_messages, search_messages or get_messages_by_date to read only that thread. A chat that is not a forum has no topics."),
		describe("List forum topics", true),
		chatIDParam(),
		mcp.WithRawOutputSchema([]byte(topicsOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := c.GetTopics(ctx, chatID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, "topics"), nil
	}

	return tool, handler
}
