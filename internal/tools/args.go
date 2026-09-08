package tools

import (
	"fmt"
	"math"
)

// wholeNumberArg reads an optional numeric tool argument that must be a whole
// number in [0, max]. JSON numbers arrive as float64, so 42.5 or 1e30 would
// otherwise be truncated or converted to an undefined int64. Absent arguments
// return (0, false, nil).
func wholeNumberArg(args map[string]any, key string, max int64) (int64, bool, error) {
	raw, present := args[key]
	if !present || raw == nil {
		return 0, false, nil
	}
	v, ok := raw.(float64)
	if !ok {
		return 0, true, fmt.Errorf("%s must be a number", key)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
		return 0, true, fmt.Errorf("%s must be a whole number", key)
	}
	if v < 0 || v > float64(max) {
		return 0, true, fmt.Errorf("%s must be between 0 and %d", key, max)
	}
	// Past 2^53 a float64 no longer holds every integer, so the value the caller
	// meant is not recoverable; with max = MaxInt64 the range check above cannot
	// catch it either, because float64(MaxInt64) rounds up.
	if v >= 1<<53 {
		return 0, true, fmt.Errorf("%s is too large to be represented exactly", key)
	}
	return int64(v), true, nil
}
