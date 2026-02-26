// Package extdatetime provides extended date/time functions for GoSonata beyond
// the official JSONata spec.
//
// All functions use milliseconds-since-epoch (Unix ms) as the date representation,
// consistent with JSONata's $toMillis / $fromMillis / $millis functions.
package extdatetime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended date/time function definitions.
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		DateAdd(),
		DateDiff(),
		DateComponents(),
		DateStartOf(),
		DateEndOf(),
	}
}

// AllEntries returns all date/time function definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions].
func AllEntries() []functions.FunctionEntry {
	all := All()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// DateAdd returns the definition for $dateAdd(millis, amount, unit).
// Adds (or subtracts if negative) the given amount of the specified unit.
//
// Supported units: "year", "month", "day", "hour", "minute", "second", "millisecond".
func DateAdd() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "dateAdd",
		Signature: "<n-n-s:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			ms, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$dateAdd: %w", err)
			}
			amount, err := toFloat(args[1])
			if err != nil {
				return nil, fmt.Errorf("$dateAdd: %w", err)
			}
			unit, ok := args[2].(string)
			if !ok {
				return nil, fmt.Errorf("$dateAdd: unit must be a string")
			}
			t := msToTime(ms)
			n := int(amount)
			switch strings.ToLower(unit) {
			case "year":
				t = t.AddDate(n, 0, 0)
			case "month":
				t = t.AddDate(0, n, 0)
			case "day":
				t = t.AddDate(0, 0, n)
			case "hour":
				t = t.Add(time.Duration(n) * time.Hour)
			case "minute":
				t = t.Add(time.Duration(n) * time.Minute)
			case "second":
				t = t.Add(time.Duration(n) * time.Second)
			case "millisecond":
				t = t.Add(time.Duration(n) * time.Millisecond)
			default:
				return nil, fmt.Errorf("$dateAdd: unsupported unit %q", unit)
			}
			return timeToMs(t), nil
		},
	}
}

// DateDiff returns the definition for $dateDiff(from, to, unit).
// Returns the difference (to - from) in the specified unit.
//
// Supported units: "year", "month", "day", "hour", "minute", "second", "millisecond".
func DateDiff() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "dateDiff",
		Signature: "<n-n-s:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			from, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$dateDiff: %w", err)
			}
			to, err := toFloat(args[1])
			if err != nil {
				return nil, fmt.Errorf("$dateDiff: %w", err)
			}
			unit, ok := args[2].(string)
			if !ok {
				return nil, fmt.Errorf("$dateDiff: unit must be a string")
			}
			tFrom := msToTime(from)
			tTo := msToTime(to)
			switch strings.ToLower(unit) {
			case "millisecond":
				return float64(tTo.UnixMilli() - tFrom.UnixMilli()), nil
			case "second":
				return math64(tTo.Unix() - tFrom.Unix()), nil
			case "minute":
				return math64((tTo.Unix() - tFrom.Unix()) / 60), nil
			case "hour":
				return math64((tTo.Unix() - tFrom.Unix()) / 3600), nil
			case "day":
				return math64((tTo.Unix() - tFrom.Unix()) / 86400), nil
			case "month":
				years, months, _ := dateDiffYMD(tFrom, tTo)
				return float64(years*12 + months), nil
			case "year":
				years, _, _ := dateDiffYMD(tFrom, tTo)
				return float64(years), nil
			default:
				return nil, fmt.Errorf("$dateDiff: unsupported unit %q", unit)
			}
		},
	}
}

// DateComponents returns the definition for $dateComponents(millis [, timezone]).
// Returns an object with year, month, day, hour, minute, second, millisecond fields.
func DateComponents() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "dateComponents",
		Signature: "<n<s>?:o>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			ms, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$dateComponents: %w", err)
			}
			loc := time.UTC
			if len(args) >= 2 && args[1] != nil {
				tzName, ok := args[1].(string)
				if !ok {
					return nil, fmt.Errorf("$dateComponents: timezone must be a string")
				}
				l, err := time.LoadLocation(tzName)
				if err != nil {
					return nil, fmt.Errorf("$dateComponents: invalid timezone %q: %w", tzName, err)
				}
				loc = l
			}
			t := msToTime(ms).In(loc)
			return map[string]interface{}{
				"year":        float64(t.Year()),
				"month":       float64(t.Month()),
				"day":         float64(t.Day()),
				"hour":        float64(t.Hour()),
				"minute":      float64(t.Minute()),
				"second":      float64(t.Second()),
				"millisecond": float64(t.Nanosecond() / 1e6),
				"weekday":     float64(t.Weekday()), // 0=Sunday
			}, nil
		},
	}
}

// DateStartOf returns the definition for $dateStartOf(millis, unit).
// Truncates the date to the start of the specified unit (UTC).
func DateStartOf() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "dateStartOf",
		Signature: "<n-s:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			ms, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$dateStartOf: %w", err)
			}
			unit, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("$dateStartOf: unit must be a string")
			}
			t := msToTime(ms).UTC()
			var result time.Time
			switch strings.ToLower(unit) {
			case "year":
				result = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			case "month":
				result = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			case "day":
				result = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			case "hour":
				result = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
			case "minute":
				result = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
			case "second":
				result = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
			default:
				return nil, fmt.Errorf("$dateStartOf: unsupported unit %q", unit)
			}
			return timeToMs(result), nil
		},
	}
}

// DateEndOf returns the definition for $dateEndOf(millis, unit).
// Returns the last millisecond of the specified unit (UTC).
func DateEndOf() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "dateEndOf",
		Signature: "<n-s:n>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			ms, err := toFloat(args[0])
			if err != nil {
				return nil, fmt.Errorf("$dateEndOf: %w", err)
			}
			unit, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("$dateEndOf: unit must be a string")
			}
			t := msToTime(ms).UTC()
			var result time.Time
			switch strings.ToLower(unit) {
			case "year":
				result = time.Date(t.Year(), 12, 31, 23, 59, 59, 999999999, time.UTC)
			case "month":
				// Last day of month
				firstOfNext := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
				result = firstOfNext.Add(-time.Millisecond)
			case "day":
				result = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999000000, time.UTC)
			case "hour":
				result = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 59, 999000000, time.UTC)
			case "minute":
				result = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 59, 999000000, time.UTC)
			case "second":
				result = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 999000000, time.UTC)
			default:
				return nil, fmt.Errorf("$dateEndOf: unsupported unit %q", unit)
			}
			return timeToMs(result), nil
		},
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func msToTime(ms float64) time.Time {
	sec := int64(ms) / 1000
	nsec := (int64(ms) % 1000) * int64(time.Millisecond)
	return time.Unix(sec, nsec).UTC()
}

func timeToMs(t time.Time) float64 {
	return float64(t.UnixMilli())
}

func math64(n int64) float64 {
	return float64(n)
}

func toFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

// dateDiffYMD returns the difference in full years, months, days (ignoring days here).
func dateDiffYMD(from, to time.Time) (years, months, days int) {
	y1, m1, d1 := from.Date()
	y2, m2, d2 := to.Date()
	years = y2 - y1
	months = int(m2) - int(m1)
	days = d2 - d1
	if days < 0 {
		months--
		// Don't need to compute exact days here
	}
	if months < 0 {
		years--
		months += 12
	}
	return
}
