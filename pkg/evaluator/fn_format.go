package evaluator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sandrolain/gosonata/pkg/types"
)

func fnFormatNumber(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	// Default formatting
	if len(args) == 1 {
		return e.formatNumberForString(num), nil
	}

	// Picture string formatting
	picture := e.toString(args[1])

	// Create decimal format with default or custom options
	format := NewDecimalFormat()

	// Parse options if provided
	if len(args) > 2 && args[2] != nil {
		var opts map[string]interface{}

		// Handle OrderedObject or regular map
		switch v := args[2].(type) {
		case *OrderedObject:
			opts = v.Values
		case map[string]interface{}:
			opts = v
		}

		if opts != nil {
			if ds, ok := opts["decimal-separator"].(string); ok && len(ds) > 0 {
				for _, r := range ds {
					format.DecimalSeparator = r
					break
				}
			}
			if gs, ok := opts["grouping-separator"].(string); ok && len(gs) > 0 {
				for _, r := range gs {
					format.GroupSeparator = r
					break
				}
			}
			if es, ok := opts["exponent-separator"].(string); ok && len(es) > 0 {
				for _, r := range es {
					format.ExponentSeparator = r
					break
				}
			}
			if ms, ok := opts["minus-sign"].(string); ok && len(ms) > 0 {
				for _, r := range ms {
					format.MinusSign = r
					break
				}
			}
			if inf, ok := opts["infinity"].(string); ok {
				format.Infinity = inf
			}
			if nan, ok := opts["NaN"].(string); ok {
				format.NaN = nan
			}
			if pct, ok := opts["percent"].(string); ok {
				format.Percent = pct
			}
			if pm, ok := opts["per-mille"].(string); ok {
				format.PerMille = pm
			}
			if zd, ok := opts["zero-digit"].(string); ok && len(zd) > 0 {
				for _, r := range zd {
					format.ZeroDigit = r
					break
				}
			}
			if od, ok := opts["digit"].(string); ok && len(od) > 0 {
				for _, r := range od {
					format.OptionalDigit = r
					break
				}
			}
			if ps, ok := opts["pattern-separator"].(string); ok && len(ps) > 0 {
				for _, r := range ps {
					format.PatternSeparator = r
					break
				}
			}
		}
	}

	// Use the complete XPath-compliant formatting
	formatted, err := FormatNumberWithPicture(num, picture, format)
	if err != nil {
		return nil, types.NewError(types.ErrorCode(err.Error()[:5]), err.Error()[7:], -1)
	}

	return formatted, nil
}

func fnFormatBase(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	// Check for non-finite values
	if math.IsInf(num, 0) || math.IsNaN(num) {
		return nil, fmt.Errorf("D3061: cannot format non-finite number")
	}

	// Default radix is 10
	radix := 10
	if len(args) > 1 && args[1] != nil {
		radixNum, err := e.toNumber(args[1])
		if err != nil {
			return nil, err
		}
		radix = int(radixNum)
		if radix < 2 || radix > 36 {
			return nil, fmt.Errorf("D3100: radix must be between 2 and 36")
		}
	}

	// Round to nearest integer using banker's rounding
	intNum := int64(roundBankers(num, 0))
	return strconv.FormatInt(intNum, radix), nil
}

// fnFormatInteger formats an integer with optional picture string.
// Signature: $formatInteger(number [, picture])
// Simplified implementation supporting basic Roman numerals and ordinals.

func fnFormatInteger(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	// Check for non-finite values
	if math.IsInf(num, 0) || math.IsNaN(num) {
		return nil, fmt.Errorf("D3061: cannot format non-finite number")
	}

	intNum := int(num)

	// Default formatting
	if len(args) == 1 {
		return fmt.Sprintf("%d", intNum), nil
	}

	// Picture string formatting
	picture := e.toString(args[1])

	switch picture {
	case "i": // Roman numerals lowercase
		return strings.ToLower(toRomanNumeral(intNum)), nil
	case "I": // Roman numerals uppercase
		return toRomanNumeral(intNum), nil
	case "w": // Words lowercase
		return strings.ToLower(numberToWords(intNum)), nil
	case "W": // Words uppercase
		return numberToWords(intNum), nil
	case "Ww": // Words title case
		return strings.Title(strings.ToLower(numberToWords(intNum))), nil
	default:
		// Default to decimal
		return fmt.Sprintf("%d", intNum), nil
	}
}

// toRomanNumeral converts an integer to Roman numeral representation.

func toRomanNumeral(num int) string {
	if num <= 0 || num >= 4000 {
		return fmt.Sprintf("%d", num) // Outside roman numeral range
	}

	val := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	sym := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}

	var result strings.Builder
	for i := 0; i < len(val); i++ {
		for num >= val[i] {
			result.WriteString(sym[i])
			num -= val[i]
		}
	}

	return result.String()
}

// numberToWords converts an integer to English words (simplified).

func numberToWords(num int) string {
	// Simplified implementation for common numbers
	if num == 0 {
		return "zero"
	}

	if num < 0 {
		return "minus " + numberToWords(-num)
	}

	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teens := []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}

	if num < 10 {
		return ones[num]
	}

	if num < 20 {
		return teens[num-10]
	}

	if num < 100 {
		return tens[num/10] + hyphenIfNeeded(num%10) + ones[num%10]
	}

	if num < 1000 {
		result := ones[num/100] + " hundred"
		if num%100 != 0 {
			result += " " + numberToWords(num%100)
		}
		return result
	}

	if num < 1000000 {
		result := numberToWords(num/1000) + " thousand"
		if num%1000 != 0 {
			result += " " + numberToWords(num%1000)
		}
		return result
	}

	// For larger numbers, just return the decimal representation
	return fmt.Sprintf("%d", num)
}

func hyphenIfNeeded(n int) string {
	if n > 0 {
		return "-"
	}
	return ""
}

// fnParseInteger parses a string to an integer with optional radix.
// Signature: $parseInteger(string [, radix])

func fnParseInteger(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := strings.TrimSpace(e.toString(args[0]))

	// Default radix is 10
	radix := 10
	if len(args) > 1 && args[1] != nil {
		radixNum, err := e.toNumber(args[1])
		if err != nil {
			return nil, err
		}
		radix = int(radixNum)
		if radix < 2 || radix > 36 {
			return nil, fmt.Errorf("D3100: radix must be between 2 and 36")
		}
	}

	// Parse integer
	num, err := strconv.ParseInt(str, radix, 64)
	if err != nil {
		return nil, fmt.Errorf("D3137: cannot parse '%s' as integer", str)
	}

	return float64(num), nil
}

// --- Enhanced Array Functions (Fase 5.2) ---

// fnDistinct removes duplicate values from an array.
// Signature: $distinct(array)
