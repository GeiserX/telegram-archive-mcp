package tools

import (
	"fmt"
	"strings"
	"time"
)

// archiveTimeLayouts are the shapes the viewer uses for message dates. Values
// without a zone are naive UTC (that is how the archive stores them).
var archiveTimeLayouts = []string{
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
}

// parseArchiveTime parses a message date as the viewer emits it and returns it
// in UTC. Naive values are read as UTC; zoned values are converted.
func parseArchiveTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	for _, layout := range archiveTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q: expected ISO 8601, e.g. 2026-06-10T18:04:17", s)
}

// formatArchiveTime renders a UTC instant the way the viewer expects cursors:
// naive ISO 8601 seconds, which the API reads as UTC.
func formatArchiveTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05")
}
