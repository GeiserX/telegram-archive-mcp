package tools

import (
	"context"
	"math"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewGetMessages(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_messages",
		mcp.WithDescription("Get messages from a Telegram chat, newest first. Page with offset, or with the keyset cursor: pass the date and id of the last message you received as before_date and before_id to get the older ones (constant time on huge chats). after_id returns the messages right after an id. Dates are naive UTC. Each message carries reply_to_msg_id and, when it has a file, media (pass its id to get_media)."),
		describe("Read a chat's messages", true),
		chatIDParam(),
		mcp.WithInteger("limit",
			mcp.Min(1), mcp.Max(500), mcp.DefaultNumber(50),
			mcp.Description("Maximum messages to return, 1 to 500 (default 50)."),
		),
		mcp.WithInteger("offset",
			mcp.Min(0), mcp.Max(100000), mcp.DefaultNumber(0),
			mcp.Description("Messages to skip, 0 to 100000 (default 0). Slow deep into a long chat; ignored when a cursor is given."),
		),
		mcp.WithString("before_date",
			mcp.Description("Keyset cursor: ISO 8601 date-time, naive values are UTC (e.g. 2026-06-10T18:04:17). Returns messages older than this instant; pair with before_id."),
		),
		mcp.WithInteger("before_id",
			mcp.Min(0),
			mcp.Description("Keyset cursor: message id. Returns messages with a smaller id; pair with before_date."),
		),
		mcp.WithInteger("after_id",
			mcp.Min(0),
			mcp.Description("Returns the messages right after this message id (newer), still newest first."),
		),
		topicIDParam(),
		mcp.WithRawOutputSchema([]byte(messagesOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := req.GetArguments()

		limit, err := limitArg(args, 50, 500)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		topicID, err := topicIDArg(args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var cur client.MessagesCursor
		if raw, present := args["before_date"]; present && raw != nil {
			v, ok := raw.(string)
			if !ok {
				return mcp.NewToolResultError("before_date must be a string"), nil
			}
			if v != "" {
				t, err := parseArchiveTime(v)
				if err != nil {
					return mcp.NewToolResultError("invalid before_date: " + err.Error()), nil
				}
				cur.BeforeDate = formatArchiveTime(t)
			}
		}
		if v, _, err := wholeNumberArg(args, "before_id", math.MaxInt64); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		} else {
			cur.BeforeID = v
		}
		if v, _, err := wholeNumberArg(args, "after_id", math.MaxInt64); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		} else {
			cur.AfterID = v
		}

		q := client.MessagesQuery{Limit: int(limit), TopicID: topicID, Cursor: cur}
		if cur == (client.MessagesCursor{}) {
			offset, _, err := wholeNumberArg(args, "offset", 100000)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			q.Offset = int(offset)
		}

		body, err := c.ListMessages(ctx, chatID, q)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, "messages"), nil
	}

	return tool, handler
}
