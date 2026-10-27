package tools

import (
	"context"
	"math"

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const listChatsOutputSchema = `{"type":"object","properties":{` +
	`"chats":{"type":"array","description":"One row per chat: ref (the chat_id other tools take), id (numeric Telegram chat id), type, accounts, and its title or name.","items":{"type":"object"}},` +
	`"total":{"type":"integer","description":"Chats matching the filters, across all pages."},` +
	`"limit":{"type":"integer"},"offset":{"type":"integer"},` +
	`"has_more":{"type":"boolean","description":"True when another page follows; call again with offset + limit."}}}`

func NewListChats(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("list_chats",
		mcp.WithDescription("List archived Telegram chats, most recent first. Use this first: each chat's ref is the chat_id the other tools take. Each row also carries id (the numeric Telegram chat id), type, accounts (the archive account ids holding the chat), and its title or name. Narrow it with search (by name), archived or folder_id (from list_folders). The answer is {chats, total, limit, offset, has_more}; page with offset while has_more is true."),
		describe("List archived chats", true),
		mcp.WithInteger("limit",
			mcp.Min(1), mcp.Max(500), mcp.DefaultNumber(100),
			mcp.Description("Maximum chats to return, 1 to 500 (default 100)."),
		),
		mcp.WithInteger("offset",
			mcp.Min(0), mcp.DefaultNumber(0),
			mcp.Description("Number of chats to skip (default 0)."),
		),
		mcp.WithString("search",
			mcp.MaxLength(200),
			mcp.Description("Keep chats whose title, first or last name, or username contains this text, case-insensitive (e.g. \"climbing\")."),
		),
		mcp.WithBoolean("archived",
			mcp.Description("true keeps only chats archived in Telegram, false only the others. Omit for both."),
		),
		mcp.WithInteger("folder_id",
			mcp.Min(0),
			mcp.Description("Keep only the chats of one Telegram folder; the id comes from list_folders."),
		),
		mcp.WithRawOutputSchema([]byte(listChatsOutputSchema)),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		limit, err := limitArg(args, 100, 500)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		offset, _, err := wholeNumberArg(args, "offset", math.MaxInt32)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		q := client.ChatsQuery{Limit: int(limit), Offset: int(offset)}
		if raw, present := args["search"]; present && raw != nil {
			v, ok := raw.(string)
			if !ok {
				return mcp.NewToolResultError("search must be a string"), nil
			}
			q.Search = v
		}
		if raw, present := args["archived"]; present && raw != nil {
			v, ok := raw.(bool)
			if !ok {
				return mcp.NewToolResultError("archived must be true or false"), nil
			}
			q.Archived = &v
		}
		if v, present, err := wholeNumberArg(args, "folder_id", 1<<53-1); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		} else if present {
			q.FolderID = &v
		}

		body, err := c.ListChats(ctx, q)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(body, "chats"), nil
	}

	return tool, handler
}
