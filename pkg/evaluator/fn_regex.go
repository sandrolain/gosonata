package evaluator

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

func fnMatch(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	str, ok := args[0].(string)
	if !ok {
		str = fmt.Sprint(args[0])
	}

	// Get limit if provided (must be extracted before using in custom matcher)
	limit := -1
	if len(args) > 2 && args[2] != nil {
		limitNum, err := e.toNumber(args[2])
		if err != nil {
			return nil, err
		}
		limit = int(limitNum)
	}

	// Get pattern (string or regex)
	var regexPattern *regexp.Regexp
	var err error

	switch pattern := args[1].(type) {
	case string:
		regexPattern, err = getOrCompileRegex(regexp.QuoteMeta(pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	case *regexp.Regexp:
		regexPattern = pattern
	case *Lambda, *FunctionDef:
		// Custom matcher function protocol.
		// The matcher is called as matcher(str, offset?) and returns a match object:
		//   { match: string, start: int, end: int, groups: array, next: function }
		// or null/undefined if no match.
		// To iterate: call result.next() to get the next match object.
		result := make([]interface{}, 0)
		cnt := 0
		// Initial call with just the string (no offset)
		currentMatch, err := e.callHOFFn(ctx, evalCtx, args[1], []interface{}{str})
		if err != nil {
			return nil, err
		}
		for currentMatch != nil {
			if limit >= 0 && cnt >= limit {
				break
			}
			// Extract match fields
			var matchStr string
			var matchIndex float64
			var groups []interface{}

			switch m := currentMatch.(type) {
			case map[string]interface{}:
				if v, ok := m["match"].(string); ok {
					matchStr = v
				}
				if v, ok := m["start"].(float64); ok {
					matchIndex = v
				}
				if v, ok := m["groups"].([]interface{}); ok {
					groups = v
				}
			case *OrderedObject:
				if v, ok := m.Values["match"].(string); ok {
					matchStr = v
				}
				if v, ok := m.Values["start"].(float64); ok {
					matchIndex = v
				}
				if v, ok := m.Values["groups"].([]interface{}); ok {
					groups = v
				}
			default:
				break
			}
			if groups == nil {
				groups = []interface{}{}
			}
			matchObj := &OrderedObject{
				Keys: []string{"match", "index", "groups"},
				Values: map[string]interface{}{
					"match":  matchStr,
					"index":  matchIndex,
					"groups": groups,
				},
			}
			result = append(result, matchObj)
			cnt++

			// Get next match by calling the next() function from the match object
			var nextFn interface{}
			switch m := currentMatch.(type) {
			case map[string]interface{}:
				nextFn = m["next"]
			case *OrderedObject:
				nextFn = m.Values["next"]
			}
			if nextFn == nil {
				break
			}
			nextMatch, err := e.callHOFFn(ctx, evalCtx, nextFn, []interface{}{})
			if err != nil {
				return nil, err
			}
			currentMatch = nextMatch
		}
		return result, nil
	default:
		return nil, fmt.Errorf("pattern must be string or regex")
	}

	// Find all matches (for string/regex patterns; custom matcher handled above)
	matches := regexPattern.FindAllStringSubmatchIndex(str, limit)
	if matches == nil {
		return []interface{}{}, nil
	}

	result := make([]interface{}, len(matches))
	for i, match := range matches {
		// match[0:2] is the full match start:end
		// match[2:] are capture groups
		matchStr := str[match[0]:match[1]]
		groups := make([]interface{}, 0)

		// Add capture groups
		for j := 1; j < len(match)/2; j++ {
			start := match[2*j]
			end := match[2*j+1]
			if start >= 0 && end >= 0 {
				groups = append(groups, str[start:end])
			} else {
				groups = append(groups, nil)
			}
		}

		matchObj := &OrderedObject{
			Keys: []string{"match", "index", "groups"},
			Values: map[string]interface{}{
				"match":  matchStr,
				"index":  float64(match[0]),
				"groups": groups,
			},
		}
		result[i] = matchObj
	}

	return result, nil
}

// jsonataExpandTemplate expands a JSONata replacement template string.
// $0 = full match, $1..$N = capture groups (1-indexed).
// Unknown named references like $w are kept as literals.
// Multi-digit group refs use greedy backtracking: try longest first,
// falling back until single digit; if single digit has no group, it expands to "".

func jsonataExpandTemplate(template string, numGroups int, groups []string, fullMatch string) string {
	buf := acquireBuf()
	defer releaseBuf(buf)
	i := 0
	for i < len(template) {
		if template[i] != '$' {
			buf.WriteByte(template[i])
			i++
			continue
		}
		i++ // skip '$'
		if i >= len(template) {
			buf.WriteByte('$')
			break
		}

		c := template[i]

		// $$ = literal '$'
		if c == '$' {
			buf.WriteByte('$')
			i++
			continue
		}

		// $0 = whole match
		if c == '0' {
			buf.WriteString(fullMatch)
			i++
			continue
		}

		// Numeric reference ($1..$N)
		if c >= '1' && c <= '9' {
			j := i
			for j < len(template) && template[j] >= '0' && template[j] <= '9' {
				j++
			}
			digits := template[i:j]
			i = j

			// Greedy backtracking: try longest numeric prefix that matches an existing group.
			written := false
			for end := len(digits); end >= 1; end-- {
				n, _ := strconv.Atoi(digits[:end])
				if n >= 1 && n <= numGroups {
					buf.WriteString(groups[n-1])
					buf.WriteString(digits[end:]) // remaining digits are literal
					written = true
					break
				}
				if end == 1 {
					// Single digit group doesn't exist → "" + remaining digits as literal
					buf.WriteString(digits[1:])
					written = true
					break
				}
			}
			if !written {
				buf.WriteString(digits)
			}
			continue
		}

		// Named reference (letters/underscore) → keep as literal $name
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			j := i
			for j < len(template) && (template[j] >= 'a' && template[j] <= 'z' ||
				template[j] >= 'A' && template[j] <= 'Z' ||
				template[j] >= '0' && template[j] <= '9' ||
				template[j] == '_') {
				j++
			}
			buf.WriteByte('$')
			buf.WriteString(template[i:j])
			i = j
			continue
		}

		// '$' followed by non-alphanumeric → literal '$'; leave current char for next iteration
		buf.WriteByte('$')
	}
	return buf.String()
}

// buildMatchObject creates the match object passed to lambda replacements in $replace.

func buildMatchObject(fullMatch string, index int, groups []string) *OrderedObject {
	groupArr := make([]interface{}, len(groups))
	for i, g := range groups {
		groupArr[i] = g
	}
	return &OrderedObject{
		Keys: []string{"match", "index", "groups"},
		Values: map[string]interface{}{
			"match":  fullMatch,
			"index":  float64(index),
			"groups": groupArr,
		},
	}
}

// fnReplace finds and replaces using regex or string pattern.
// Signature: $replace(str, pattern, replacement [, limit])

func fnReplace(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	// undefined inputs return undefined
	if args[0] == nil {
		return nil, nil
	}

	str, ok := args[0].(string)
	if !ok {
		str = fmt.Sprint(args[0])
	}

	// Get limit if provided
	limit := -1 // -1 means unlimited
	if len(args) > 3 && args[3] != nil {
		limitNum, err := e.toNumber(args[3])
		if err != nil {
			return nil, err
		}
		limit = int(limitNum)
		if limit < 0 {
			return nil, fmt.Errorf("D3011: limit must be non-negative")
		}
	}

	switch pattern := args[1].(type) {
	case string:
		// Validate pattern is not empty
		if pattern == "" {
			return nil, fmt.Errorf("D3010: pattern cannot be empty")
		}
		replacement := fmt.Sprint(args[2])
		if limit < 0 {
			return strings.ReplaceAll(str, pattern, replacement), nil
		}
		return strings.Replace(str, pattern, replacement, limit), nil

	case *regexp.Regexp:
		// Validate pattern is not empty
		if pattern.String() == "" {
			return nil, fmt.Errorf("D3010: pattern cannot be empty")
		}

		// Find all submatch indices (respects limit)
		maxMatches := -1
		if limit >= 0 {
			maxMatches = limit
		}
		allMatches := pattern.FindAllStringSubmatchIndex(str, maxMatches)

		buf := acquireBuf()
		defer releaseBuf(buf)
		lastEnd := 0
		for _, match := range allMatches {
			matchStart := match[0]
			matchEnd := match[1]

			// D1004: a zero-length match would cause an infinite replacement loop
			if matchStart == matchEnd {
				return nil, types.NewError(types.ErrZeroLengthMatch, "regular expression match did not advance position", -1)
			}

			buf.WriteString(str[lastEnd:matchStart])

			fullMatch := str[matchStart:matchEnd]

			// Extract capture groups
			numGroups := (len(match) - 2) / 2
			groups := make([]string, numGroups)
			for j := 0; j < numGroups; j++ {
				gStart := match[2+2*j]
				gEnd := match[3+2*j]
				if gStart >= 0 && gEnd >= 0 {
					groups[j] = str[gStart:gEnd]
				}
				// non-participating group stays as ""
			}

			switch args[2].(type) {
			case *Lambda, *FunctionDef:
				matchObj := buildMatchObject(fullMatch, matchStart, groups)
				result, err := e.callHOFFn(ctx, evalCtx, args[2], []interface{}{matchObj})
				if err != nil {
					return nil, err
				}
				if result == nil {
					// nil = undefined → keep as empty string
					break
				}
				resultStr, ok := result.(string)
				if !ok {
					return nil, types.NewError(types.ErrReplacementNotString, "replacement function must return a string", -1)
				}
				buf.WriteString(resultStr)
			default:
				replacement := fmt.Sprint(args[2])
				expanded := jsonataExpandTemplate(replacement, numGroups, groups, fullMatch)
				buf.WriteString(expanded)
			}

			lastEnd = matchEnd
		}

		buf.WriteString(str[lastEnd:])
		return buf.String(), nil

	default:
		return nil, fmt.Errorf("pattern must be string or regex")
	}
}

// --- Date/Time Functions ---
