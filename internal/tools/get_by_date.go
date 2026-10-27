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
	TopicID   int64             `json:"topic_id,omitempty"`
	Count     int               `json:"count"`
	Truncated bool              `json:"truncated"`
	Messages  []json.RawMessage `json:"messages"`
}

const byDateOutputSchema = `{"type":"object","properties":{` +
	`"date":{"type":"string","description":"The day asked for, YYYY-MM-DD."},` +
	`"timezone":{"type":"string","description":"IANA timezone the day was taken in."},` +
	`"topic_id":{"type":"integer","description":"The forum topic filter, when one was given."},` +
	`"count":{"type":"integer","description":"Messages returned."},` +
	`"truncated":{"type":"boolean","description":"True when the day held more than limit messages and the newest were dropped."},` +
	`"messages":{"type":"array","description":"The day's messages, oldest first, with the same fields as get_messages.","items":{"type":"object"}}},` +
	`"required":["date","timezone","count","truncated","messages"]}`

func NewGetMessagesByDate(c *client.Client) (mcp.Tool, server.ToolHandlerFunc) {

	tool := mcp.NewTool("get_messages_by_date",
		mcp.WithDescription("Get every message of a Telegram chat sent on one calendar day, oldest first. The day is taken in the given timezone (default UTC); the archive stores dates in UTC. Walks the chat with the keyset cursor, so it is cheap even on chats with hundreds of thousands of messages. Use it to answer \"what was said on <day>\"; use get_messages to page through a chat instead."),
		describe("Read one day of a chat", true),
		chatIDParam(),
		mcp.WithString("date",
			mcp.Required(),
			mcp.Pattern(`^\d{4}-\d{2}-\d{2}$`),
			mcp.Description("The calendar day, YYYY-MM-DD (e.g. 2026-06-10)."),
		),
		mcp.WithString("timezone",
			mcp.Description("IANA timezone the date is expressed in (e.g. Europe/Madrid). Default UTC."),
		),
		mcp.WithInteger("limit",
			mcp.Min(1), mcp.Max(byDateMaxLimit), mcp.DefaultNumber(byDateDefaultLimit),
			mcp.Description("Maximum messages to return, 1 to 5000 (default 1000). When the day has more, the newest are dropped and truncated is true."),
		),
		topicIDParam(),
		mcp.WithRawOutputSchema([]byte(byDateOutputSchema)),
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
		tz := ""
		if raw, present := args["timezone"]; present && raw != nil {
			v, ok := raw.(string)
			if !ok {
				return mcp.NewToolResultError("timezone must be a string"), nil
			}
			tz = v
		}
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

		limit64, err := limitArg(args, byDateDefaultLimit, byDateMaxLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := int(limit64)
		topicID, err := topicIDArg(args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		messages, truncated, err := collectDay(ctx, c, chatID, topicID, dayStart, dayEnd, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		res := byDateResult{
			Date:      date,
			Timezone:  tz,
			TopicID:   topicID,
			Count:     len(messages),
			Truncated: truncated,
			Messages:  messages,
		}
		out, err := json.Marshal(res)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructured(res, string(out)), nil
	}

	return tool, handler
}

// collectDay walks the chat backwards from dayEnd until the day boundary, an
// empty page or a cursor that does not advance, and keeps the messages whose
// date falls in [dayStart, dayEnd). It returns them oldest first. When more
// than limit messages fall in the window, the oldest limit are kept and
// truncated is true; memory stays bounded by 2*limit while walking.
func collectDay(ctx context.Context, c *client.Client, chatID string, topicID int64, dayStart, dayEnd time.Time, limit int) ([]json.RawMessage, bool, error) {
	var collected []json.RawMessage
	cur := client.MessagesCursor{BeforeDate: formatArchiveTime(dayEnd)}
	truncated := false

	for {
		body, err := c.ListMessages(ctx, chatID, client.MessagesQuery{Limit: byDatePageSize, TopicID: topicID, Cursor: cur})
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
			if len(collected) >= 2*limit {
				// walking newest first, the oldest are at the tail: drop the head
				collected = append([]json.RawMessage(nil), collected[len(collected)-limit:]...)
				truncated = true
			}
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

	if len(collected) > limit {
		collected = collected[len(collected)-limit:]
		truncated = true
	}
	// newest first -> oldest first
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	if collected == nil {
		collected = []json.RawMessage{}
	}
	return collected, truncated, nil
}
