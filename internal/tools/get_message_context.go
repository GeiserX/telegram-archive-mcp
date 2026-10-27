package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const contextMaxSide = 50

// contextResult is what get_message_context returns.
type contextResult struct {
	MessageID int64             `json:"message_id"`
	Before    int               `json:"before"`
	After     int               `json:"after"`
	Messages  []json.RawMessage `json:"messages"`
}

const contextOutputSchema = `{"type":"object","properties":{` +
	`"message_id":{"type":"integer","description":"The message asked for; it is in messages."},` +
	`"before":{"type":"integer","description":"How many older messages came back (fewer than asked at the start of the chat)."},` +
	`"after":{"type":"integer","description":"How many newer messages came back (fewer than asked at the end of the chat)."},` +
	`"messages":{"type":"array","description":"Oldest first: the older messages, the message itself, then the newer ones, with the same fields as get_messages.","items":{"type":"object"}}},` +
	`"required":["message_id","before","after","messages"]}`

func NewGetMessageContext(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_message_context",
		mcp.WithDescription("Read the conversation around one message: the message itself with up to `before` older and `after` newer messages of the same chat, oldest first. Use it after search_messages or search_all_chats finds a hit, or to see what a reply_to_msg_id points at."),
		describe("Read around a message", true),
		chatIDParam(),
		mcp.WithInteger("message_id",
			mcp.Required(), mcp.Min(1),
			mcp.Description("Id of the message to centre on (a message's id field)."),
		),
		mcp.WithInteger("before",
			mcp.Min(0), mcp.Max(contextMaxSide), mcp.DefaultNumber(5),
			mcp.Description("Older messages to include, 0 to 50 (default 5)."),
		),
		mcp.WithInteger("after",
			mcp.Min(0), mcp.Max(contextMaxSide), mcp.DefaultNumber(5),
			mcp.Description("Newer messages to include, 0 to 50 (default 5)."),
		),
		mcp.WithRawOutputSchema([]byte(contextOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := req.GetArguments()
		// The older half asks for before_id = message_id + 1, which must stay below 2^53.
		messageID, present, err := wholeNumberArg(args, "message_id", 1<<53-2)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if !present || messageID < 1 {
			return mcp.NewToolResultError("message_id is required and must be at least 1"), nil
		}
		before, err := sideArg(args, "before")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		after, err := sideArg(args, "after")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// A lone before_id is ordered by id, newest first, so before_id = N+1
		// returns message N and then the older ones.
		older, err := fetchRows(ctx, c, chatID, client.MessagesQuery{Limit: before + 1, Cursor: client.MessagesCursor{BeforeID: messageID + 1}})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(older) == 0 || rowID(older[0]) != messageID {
			return mcp.NewToolResultError(fmt.Sprintf("message %d not found in this chat", messageID)), nil
		}
		var newer []json.RawMessage
		if after > 0 {
			// after_id returns the messages right after N, newest first.
			newer, err = fetchRows(ctx, c, chatID, client.MessagesQuery{Limit: after, Cursor: client.MessagesCursor{AfterID: messageID}})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		slices.Reverse(older)
		slices.Reverse(newer)
		res := contextResult{
			MessageID: messageID,
			Before:    len(older) - 1,
			After:     len(newer),
			Messages:  append(older, newer...),
		}
		out, err := json.Marshal(res)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructured(res, string(out)), nil
	}

	return tool, handler
}

func sideArg(args map[string]any, key string) (int, error) {
	v, present, err := wholeNumberArg(args, key, contextMaxSide)
	if err != nil {
		return 0, err
	}
	if !present {
		return 5, nil
	}
	return int(v), nil
}

// fetchRows lists messages and decodes the viewer's JSON array.
func fetchRows(ctx context.Context, c *client.Client, chatID string, q client.MessagesQuery) ([]json.RawMessage, error) {
	body, err := c.ListMessages(ctx, chatID, q)
	if err != nil {
		return nil, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("unexpected answer from the archive: %w", err)
	}
	return rows, nil
}

// rowID reads the id of one message row, or 0.
func rowID(raw json.RawMessage) int64 {
	var m archiveMessage
	if json.Unmarshal(raw, &m) != nil {
		return 0
	}
	return m.ID
}
