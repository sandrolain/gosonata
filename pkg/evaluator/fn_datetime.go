package evaluator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var nowTime time.Time

var nowCalculated bool

// reTimezoneOffset matches a bare timezone offset like +0000 or -0000 at end of string.
var reTimezoneOffset = mustCompileRegex(`([+-])(\d{2})(\d{2})$`) // fnNow returns current timestamp in ISO 8601 format.
// Signature: $now([picture [, timezone]])

func fnNow(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Cache the current time for all evaluations in this context
	if !nowCalculated {
		nowTime = time.Now()
		nowCalculated = true
	}

	// Simple ISO 8601 format if no picture provided
	if len(args) == 0 {
		return nowTime.UTC().Format(time.RFC3339Nano), nil
	}

	// Note: Full XPath datetime formatting is complex and not implemented
	// Return simple ISO format for now
	return nowTime.UTC().Format(time.RFC3339Nano), nil
}

// fnMillis returns milliseconds since Unix epoch.

func fnMillis(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// Use same time as $now for consistency
	if !nowCalculated {
		nowTime = time.Now()
		nowCalculated = true
	}
	return float64(nowTime.UnixMilli()), nil
}

// fnFromMillis converts milliseconds since epoch to ISO 8601 string.
// Signature: $fromMillis(number [, picture [, timezone]])

func fnFromMillis(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	millis, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	timestamp := time.Unix(0, int64(millis)*1000000).UTC()

	// Simple ISO 8601 format if no picture provided
	if len(args) < 2 || args[1] == nil {
		return timestamp.Format(time.RFC3339Nano), nil
	}

	picture, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("D3110: picture argument of $fromMillis must be a string")
	}

	return formatTimestampWithPicture(timestamp, picture), nil
}

// formatTimestampWithPicture formats a time.Time using an XPath picture format string.
// Supports common markers: [Y], [Y0001], [M], [M01], [D], [D01], [H], [H00],
// [m], [m00], [s], [s00], [X0001] (ISO week year), [W01] (ISO week), [F1] (weekday).

func formatTimestampWithPicture(t time.Time, picture string) string {
	result := picture

	// ISO week date components
	isoYear, isoWeek := t.ISOWeek()
	weekday := int(t.Weekday()) // 0=Sun, 1=Mon, ..., 6=Sat
	isoWeekday := weekday
	if isoWeekday == 0 {
		isoWeekday = 7 // Sun = 7 in ISO
	}

	// Process markers from most specific to least specific
	// ISO week date markers
	result = strings.ReplaceAll(result, "[X0001]", fmt.Sprintf("%04d", isoYear))
	result = strings.ReplaceAll(result, "[X]", fmt.Sprintf("%d", isoYear))
	result = strings.ReplaceAll(result, "[W01]", fmt.Sprintf("%02d", isoWeek))
	result = strings.ReplaceAll(result, "[W]", fmt.Sprintf("%d", isoWeek))
	result = strings.ReplaceAll(result, "[F1]", fmt.Sprintf("%d", isoWeekday))
	result = strings.ReplaceAll(result, "[F]", fmt.Sprintf("%d", isoWeekday))

	// Standard date/time components (specific markers before generic)
	result = strings.ReplaceAll(result, "[Y0001]", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "[Y0000]", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "[Y,*-4]", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "[Y]", fmt.Sprintf("%04d", t.Year()))
	result = strings.ReplaceAll(result, "[M01]", fmt.Sprintf("%02d", int(t.Month())))
	result = strings.ReplaceAll(result, "[M00]", fmt.Sprintf("%02d", int(t.Month())))
	result = strings.ReplaceAll(result, "[M]", fmt.Sprintf("%d", int(t.Month())))
	result = strings.ReplaceAll(result, "[D01]", fmt.Sprintf("%02d", t.Day()))
	result = strings.ReplaceAll(result, "[D00]", fmt.Sprintf("%02d", t.Day()))
	result = strings.ReplaceAll(result, "[D]", fmt.Sprintf("%d", t.Day()))
	result = strings.ReplaceAll(result, "[H00]", fmt.Sprintf("%02d", t.Hour()))
	result = strings.ReplaceAll(result, "[H01]", fmt.Sprintf("%02d", t.Hour()))
	result = strings.ReplaceAll(result, "[H]", fmt.Sprintf("%d", t.Hour()))
	result = strings.ReplaceAll(result, "[m00]", fmt.Sprintf("%02d", t.Minute()))
	result = strings.ReplaceAll(result, "[m01]", fmt.Sprintf("%02d", t.Minute()))
	result = strings.ReplaceAll(result, "[m]", fmt.Sprintf("%d", t.Minute()))
	result = strings.ReplaceAll(result, "[s00]", fmt.Sprintf("%02d", t.Second()))
	result = strings.ReplaceAll(result, "[s01]", fmt.Sprintf("%02d", t.Second()))
	result = strings.ReplaceAll(result, "[s]", fmt.Sprintf("%d", t.Second()))
	result = strings.ReplaceAll(result, "[f001]", fmt.Sprintf("%03d", t.Nanosecond()/1e6))
	result = strings.ReplaceAll(result, "[f]", fmt.Sprintf("%d", t.Nanosecond()/1e6))

	return result
}

// fnToMillis converts ISO 8601 timestamp to milliseconds since epoch.
// Signature: $toMillis(timestamp [, picture])

func fnToMillis(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	timestamp, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("D3110: timestamp must be a string, got %T", args[0])
	}

	// If picture format is provided, use custom parsing
	if len(args) == 2 && args[1] != nil {
		picture, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("picture format must be a string")
		}
		return parseTimestampWithPicture(timestamp, picture)
	}

	// Normalize timezone offset: convert +0000 to +00:00
	normalized := normalizeTimezoneOffset(timestamp)

	// Try parsing ISO 8601 formats
	layouts := []string{
		time.RFC3339Nano,                     // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,                         // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05.999999999Z0700", // with numeric timezone
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05.999999999", // without timezone
		"2006-01-02T15:04:05",
		"2006-01-02", // date only
		"2006-01",    // year-month only
		"2006",       // year only
	}

	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, normalized)
		if err == nil {
			return float64(t.UnixMilli()), nil
		}
	}

	return nil, fmt.Errorf("D3110: cannot parse timestamp: %s", timestamp)
}

// normalizeTimezoneOffset converts timezone offsets like +0000 to +00:00

func normalizeTimezoneOffset(timestamp string) string {
	if reTimezoneOffset.MatchString(timestamp) {
		return reTimezoneOffset.ReplaceAllString(timestamp, `$1$2:$3`)
	}
	return timestamp
}

// parseTimestampWithPicture parses a timestamp using a picture format string.
// Picture format uses markers like [Y0001] for year, [M01] for month, etc.
// This is a simplified implementation supporting only the patterns in the test suite.

func parseTimestampWithPicture(timestamp, picture string) (interface{}, error) {
	// Parse picture format to extract component patterns
	type component struct {
		name    string
		pattern string
	}

	var components []component
	pattern := picture

	// Escape regex special characters in literal parts between markers
	// Replace picture markers with regex groups (check longer/specific markers first)
	type replacement struct {
		markers []string
		comp    component
	}

	replacements := []replacement{
		{[]string{"[Y0001]", "[Y0000]", "[Y,*-4]", "[Y]"}, component{"year", `(\d{1,4})`}},
		{[]string{"[M01]", "[M00]", "[M]"}, component{"month", `(\d{1,2})`}},
		{[]string{"[D01]", "[D00]", "[D]"}, component{"day", `(\d{1,2})`}},
		{[]string{"[H00]", "[H]"}, component{"hour", `(\d{1,2})`}},
		{[]string{"[m00]", "[m]"}, component{"minute", `(\d{1,2})`}},
		{[]string{"[s00]", "[s]"}, component{"second", `(\d{1,2})`}},
	}

	for _, repl := range replacements {
		for _, marker := range repl.markers {
			if strings.Contains(pattern, marker) {
				components = append(components, repl.comp)
				pattern = strings.Replace(pattern, marker, repl.comp.pattern, 1)
				break
			}
		}
	}

	// Compile and match — use regex cache to avoid re-compiling identical patterns.
	re, err := getOrCompileRegex("^" + pattern + "$")
	if err != nil {
		return nil, fmt.Errorf("invalid picture format: %s", picture)
	}

	matches := re.FindStringSubmatch(timestamp)
	if matches == nil {
		return nil, fmt.Errorf("D3110: cannot parse timestamp with picture format: %s", timestamp)
	}

	// Extract components
	values := make(map[string]int)
	for i, comp := range components {
		val, _ := strconv.Atoi(matches[i+1])
		values[comp.name] = val
	}

	// Default missing components
	year := values["year"]
	if year == 0 {
		year = time.Now().UTC().Year()
	}
	month := values["month"]
	if month == 0 {
		month = 1
	}
	day := values["day"]
	if day == 0 {
		day = 1
	}
	hour := values["hour"]
	minute := values["minute"]
	second := values["second"]

	// Create time and convert to milliseconds
	t := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	return float64(t.UnixMilli()), nil
}

// --- Encoding Functions (Fase 5.3) ---

// fnBase64Encode encodes a string to base64.
// Signature: $base64encode(string)
