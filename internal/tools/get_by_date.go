package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	_ "time/tzdata" // the release image has no zoneinfo; embed it so IANA names resolve

	"github.com/geiserx/telegram-archive-mcp/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	byDatePageSize     = 500
	byDateDefaultLimit = 1000
	byDateMaxLimit     = 5000
	byDateMaxPages     = 100
)

// archiveMessage is the slice of a message the day walker needs.
type archiveMessage struct {
	ID   int64  `json:"id"`
	Date string `json:"date"`
}

// byDateResult is what get_messages_by_date returns.
type byDateResult struct {
	Date      string            `json:"date"`
	Timezone  string            `json:"timezone"`
	Count     int               `json:"count"`
	Truncated bool              `json:"truncated"`
	Messages  []json.RawMessage `json:"messages"`
}

func NewGetMessagesByDate(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_messages_by_date",
		mcp.WithDescription("Get every message of a Telegram chat sent on one calendar day, oldest first. The day is taken in the given timezone (default UTC); the archive stores dates in UTC. Walks the chat with the keyset cursor, so it is cheap even on chats with hundreds of thousands of messages."),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("Chat ID to retrieve messages from"),
		),
		mcp.WithString("date",
			mcp.Required(),
			mcp.Description("Date in YYYY-MM-DD format"),
		),
		mcp.WithString("timezone",
			mcp.Description("IANA timezone the date is expressed in (e.g. Europe/Madrid). Default UTC."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum messages to return (default 1000, max 5000). When the day has more, the newest are dropped and truncated is true."),
		),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			ReadOnlyHint: boolPtr(true),
		}),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatID, err := req.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		date, err := req.RequireString("date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := req.GetArguments()

		loc := time.UTC
		tz, _ := args["timezone"].(string)
		if tz != "" {
			l, err := time.LoadLocation(tz)
			if err != nil {
				return mcp.NewToolResultError("invalid timezone: expected an IANA name such as Europe/Madrid"), nil
			}
			loc = l
		} else {
			tz = "UTC"
		}

		day, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			return mcp.NewToolResultError("invalid date format: expected YYYY-MM-DD"), nil
		}
		dayStart := day.UTC()
		dayEnd := day.AddDate(0, 0, 1).UTC()

		limit := byDateDefaultLimit
		if v, ok := args["limit"].(float64); ok && v > 0 {
			limit = int(v)
		}
		if limit > byDateMaxLimit {
			limit = byDateMaxLimit
		}

		messages, truncated, err := collectDay(ctx, c, chatID, dayStart, dayEnd, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		out, err := json.Marshal(byDateResult{
			Date:      date,
			Timezone:  tz,
			Count:     len(messages),
			Truncated: truncated,
			Messages:  messages,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	}

	return tool, handler
}

// collectDay walks the chat backwards from dayEnd and keeps the messages whose
// date falls in [dayStart, dayEnd). It returns them oldest first. When more
// than limit messages fall in the window, the oldest limit are kept and
// truncated is true.
func collectDay(ctx context.Context, c *client.Client, chatID string, dayStart, dayEnd time.Time, limit int) ([]json.RawMessage, bool, error) {
	var collected []json.RawMessage
	cur := client.MessagesCursor{BeforeDate: formatArchiveTime(dayEnd)}
	truncated := false

	for page := 0; page < byDateMaxPages; page++ {
		body, err := c.GetMessagesCursor(ctx, chatID, byDatePageSize, cur)
		if err != nil {
			return nil, false, err
		}
		var rows []json.RawMessage
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, false, fmt.Errorf("unexpected response from the archive: %w", err)
		}
		if len(rows) == 0 {
			break
		}

		stop := false
		var last archiveMessage
		for _, raw := range rows {
			var m archiveMessage
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, false, fmt.Errorf("unexpected message shape: %w", err)
			}
			ts, err := parseArchiveTime(m.Date)
			if err != nil {
				return nil, false, err
			}
			last = m
			if ts.Before(dayStart) {
				stop = true
				break
			}
			if !ts.Before(dayEnd) {
				continue
			}
			if cur.BeforeID > 0 && m.ID >= cur.BeforeID {
				// already seen: the API did not honour the cursor
				continue
			}
			collected = append(collected, raw)
		}
		if stop {
			break
		}
		if last.ID == 0 || (cur.BeforeID > 0 && last.ID >= cur.BeforeID) {
			// the cursor did not advance; do not loop forever
			break
		}
		cur = client.MessagesCursor{BeforeDate: last.Date, BeforeID: last.ID}
	}

	// newest first -> oldest first
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	if len(collected) > limit {
		collected = collected[:limit]
		truncated = true
	}
	if collected == nil {
		collected = []json.RawMessage{}
	}
	return collected, truncated, nil
}
