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

// fnFormatInteger formats an integer using an XPath-like picture string.
// Signature: $formatInteger(number [, picture])
// Supports: decimal patterns, roman numerals, words, alphabetic, ordinals.

func fnFormatInteger(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	num, err := e.toNumber(args[0])
	if err != nil {
		return nil, err
	}

	if math.IsInf(num, 0) || math.IsNaN(num) {
		return nil, fmt.Errorf("D3061: cannot format non-finite number")
	}

	if len(args) == 1 {
		return fmt.Sprintf("%d", int64(num)), nil
	}

	picture := e.toString(args[1])
	return applyIntegerPicture(num, picture)
}

// applyIntegerPicture applies an XPath-style picture string to a number.
// Accepts float64 to handle values that overflow int64 (e.g., 1e46).
func applyIntegerPicture(num float64, picture string) (string, error) {
	n := int64(num) // safe for values within int64 range

	// Split on ';' to separate format token from optional modifier.
	parts := strings.SplitN(picture, ";", 2)
	token := parts[0]
	modifier := ""
	if len(parts) == 2 {
		modifier = parts[1]
	}
	ordinal := modifier == "o"

	switch token {
	case "i":
		s := strings.ToLower(toRomanNumeral(int(n)))
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	case "I":
		s := toRomanNumeral(int(n))
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	case "w":
		s := numberToWordsF64(num)
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	case "W":
		s := strings.ToUpper(numberToWordsF64(num))
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	case "Ww":
		s := toTitleCase(numberToWordsF64(num))
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	case "a":
		s := alphabeticNumber(int(n), false)
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	case "A":
		s := alphabeticNumber(int(n), true)
		if ordinal {
			s = makeOrdinalWord(s)
		}
		return s, nil
	default:
		// Decimal pattern (possibly with grouping/padding/unicode digits).
		sign := ""
		absN := n
		absF := num
		if n < 0 {
			sign = "-"
			absN = -n
			absF = -num
		}
		_ = absF
		formatted, err := formatIntegerWithPattern(absN, token)
		if err != nil {
			return "", err
		}
		result := sign + formatted
		if ordinal {
			result = result + ordinalSuffix(int(absN))
		}
		return result, nil
	}
}

// formatIntegerWithPattern applies a decimal pattern (e.g., '0000', '#,##0', '###١') to an integer.
// Returns an error for invalid patterns such as mixed digit families (D3131).
func formatIntegerWithPattern(n int64, pattern string) (string, error) {
	runes := []rune(pattern)
	if len(runes) == 0 {
		return fmt.Sprintf("%d", n), nil
	}

	// Identify digit families present in the pattern.
	// 'zeroDigit' is the zero of the first digit family encountered.
	var zeroDigit rune
	for _, r := range runes {
		if r >= '0' && r <= '9' {
			if zeroDigit == 0 {
				zeroDigit = '0'
			} else if zeroDigit != '0' {
				return "", fmt.Errorf("D3131: picture string contains characters from different decimal digit families")
			}
		} else if r != '#' {
			if family := getZeroDigitOfFamily(r); family != 0 {
				if zeroDigit == 0 {
					zeroDigit = family
				} else if zeroDigit != family {
					return "", fmt.Errorf("D3131: picture string contains characters from different decimal digit families")
				}
			}
		}
	}
	if zeroDigit == 0 {
		zeroDigit = '0'
	}

	var pattern2 []patternChar

	for i, r := range runes {
		if (zeroDigit == '0' && r >= '0' && r <= '9') ||
			(zeroDigit != '0' && r >= zeroDigit && r < zeroDigit+10) {
			// All decimal digit chars (0-9 of the family) are mandatory.
			pattern2 = append(pattern2, patternChar{isMandatory: true})
		} else if r == '#' {
			pattern2 = append(pattern2, patternChar{isMandatory: false})
		} else if i > 0 {
			// Treat as grouping separator.
			pattern2 = append(pattern2, patternChar{isGrouping: true, groupingChar: r})
		}
	}

	// Count mandatory digit positions.
	minDigits := 0
	for _, pc := range pattern2 {
		if pc.isMandatory {
			minDigits++
		}
	}

	// If no mandatory decimal digits found and no optional digits either,
	// this is a numbering sequence pattern (D3130: not supported).
	if minDigits == 0 {
		hasOptional := false
		for _, pc := range pattern2 {
			if !pc.isGrouping {
				hasOptional = true
				break
			}
		}
		if !hasOptional {
			return "", fmt.Errorf("D3130: $formatInteger() does not support this numbering sequence: %s", pattern)
		}
	}

	// Build the raw digit string for abs(n).
	raw := fmt.Sprintf("%d", n)
	for len(raw) < minDigits {
		raw = "0" + raw
	}

	seps := getGroupingSeparators(pattern2)
	result := insertGroupingSeparators(raw, seps, zeroDigit)
	return result, nil
}

// patternChar represents a character in a decimal integer format pattern.
type patternChar struct {
	isMandatory  bool // true = mandatory digit (e.g., '0'), false = optional ('#')
	isGrouping   bool // true = grouping separator
	groupingChar rune // the separator character (only valid when isGrouping is true)
}

// groupingSep holds a grouping separator position and its character.
type groupingSep struct {
	pos  int  // number of digit positions from the right edge before which to insert
	char rune // the separator character
}

// getGroupingSeparators extracts per-position separator information from the pattern.
// Returns separators ordered from rightmost to leftmost, with CUMULATIVE positions.
func getGroupingSeparators(pattern []patternChar) []groupingSep {
	var result []groupingSep
	digitCount := 0
	cumulativePos := 0
	for i := len(pattern) - 1; i >= 0; i-- {
		pc := pattern[i]
		if pc.isGrouping {
			if digitCount > 0 {
				cumulativePos += digitCount
				result = append(result, groupingSep{pos: cumulativePos, char: pc.groupingChar})
				digitCount = 0
			}
		} else {
			digitCount++
		}
	}
	return result
}

// insertGroupingSeparators inserts separators at positions specified by seps.
// Handles regular intervals (repeating single char) and irregular patterns.
func insertGroupingSeparators(raw string, seps []groupingSep, zeroDigit rune) string {
	if len(seps) == 0 {
		if zeroDigit == '0' {
			return raw
		}
		// Only digit substitution needed.
		var buf strings.Builder
		for _, d := range raw {
			buf.WriteRune(zeroDigit + (d - '0'))
		}
		return buf.String()
	}

	n := len(raw)

	// Detect regular pattern: all seps have the same char AND equal spacing.
	primaryInterval := seps[0].pos
	primaryChar := seps[0].char
	isRegular := true
	for i, sep := range seps {
		if sep.char != primaryChar {
			isRegular = false
			break
		}
		if i > 0 && (sep.pos-seps[i-1].pos) != primaryInterval {
			isRegular = false
			break
		}
	}

	// Build the position→char map.
	posToChar := make(map[int]rune)
	for _, sep := range seps {
		posToChar[sep.pos] = sep.char
	}
	if isRegular {
		// Add repeating positions beyond what the pattern explicitly defines.
		for pos := primaryInterval; pos < n; pos += primaryInterval {
			posToChar[pos] = primaryChar
		}
	}

	// Build output, inserting seps at marked positions and converting digit family.
	digits := []rune(raw)
	var buf strings.Builder
	buf.Grow(n + len(posToChar) + 4)
	for i, digit := range digits {
		posFromRight := n - i // digits remaining including current (= right edge distance + 1)
		if i > 0 {
			if sep, ok := posToChar[posFromRight]; ok {
				buf.WriteRune(sep)
			}
		}
		if zeroDigit != '0' {
			buf.WriteRune(zeroDigit + (digit - '0'))
		} else {
			buf.WriteRune(digit)
		}
	}
	return buf.String()
}

// getZeroDigitOfFamily returns the zero-digit rune of the Unicode digit family
// containing r, or 0 if r is not a Unicode decimal digit.
func getZeroDigitOfFamily(r rune) rune {
	// Check known Unicode digit families by testing if r is in a known range.
	families := []rune{
		0x0660, // Arabic-Indic
		0x06F0, // Extended Arabic-Indic
		0x07C0, // NKo
		0x0966, // Devanagari
		0x09E6, // Bengali
		0x0A66, // Gurmukhi
		0x0AE6, // Gujarati
		0x0B66, // Oriya
		0x0BE6, // Tamil
		0x0C66, // Telugu
		0x0CE6, // Kannada
		0x0D66, // Malayalam
		0x0E50, // Thai
		0x0ED0, // Lao
		0x0F20, // Tibetan
		0x1040, // Myanmar
		0x1090, // Myanmar Shan
		0x17E0, // Khmer
		0x1810, // Mongolian
		0x1946, // Limbu
		0x19D0, // New Tai Lue
		0x1A80, // Tai Tham Hora
		0x1A90, // Tai Tham Tham
		0x1B50, // Balinese
		0x1BB0, // Sundanese
		0x1C40, // Lepcha
		0x1C50, // Ol Chiki
		0xA620, // Vai
		0xA8D0, // Saurashtra
		0xA900, // Kayah Li
		0xA9D0, // Javanese
		0xAA50, // Cham
		0xABF0, // Meetei Mayek
		0xFF10, // Fullwidth
	}
	for _, zero := range families {
		if r >= zero && r < zero+10 {
			return zero
		}
	}
	return 0
}

// toRomanNumeral converts an integer to Roman numeral representation.
func toRomanNumeral(num int) string {
	if num == 0 {
		return ""
	}
	if num < 0 || num >= 4000 {
		return fmt.Sprintf("%d", num)
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

// parseRomanNumeral converts a Roman numeral string to integer.
func parseRomanNumeral(s string) int {
	if s == "" {
		return 0
	}
	s = strings.ToUpper(s)
	vals := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	result := 0
	prev := 0
	for _, r := range s {
		cur := vals[r]
		if cur > prev && prev != 0 {
			result += cur - 2*prev
		} else {
			result += cur
		}
		prev = cur
	}
	return result
}

// alphabeticNumber converts n (1-based) to a base-26 alphabetic string (a=1, b=2, ..., z=26, aa=27, ...).
func alphabeticNumber(n int, upper bool) string {
	if n <= 0 {
		return ""
	}
	var result []rune
	for n > 0 {
		n-- // shift to 0-based
		result = append([]rune{rune('a' + n%26)}, result...)
		n /= 26
	}
	s := string(result)
	if upper {
		s = strings.ToUpper(s)
	}
	return s
}

// parseAlphabeticNumber converts an alphabetic string back to an integer (1-based).
func parseAlphabeticNumber(s string) int {
	s = strings.ToLower(s)
	result := 0
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return 0
		}
		result = result*26 + int(r-'a') + 1
	}
	return result
}

// ordinalSuffix returns "st", "nd", "rd", or "th" for integer n.
func ordinalSuffix(n int) string {
	if n < 0 {
		n = -n
	}
	mod100 := n % 100
	if mod100 >= 11 && mod100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// toTitleCase converts a word-form string to title case.
// Conjunctions like "and" are NOT capitalised; hyphenated compounds capitalise each part.
func toTitleCase(s string) string {
	// Words that should stay lowercase (unless first word).
	exceptions := map[string]bool{
		"and": true, "or": true, "of": true, "in": true, "the": true,
		"a": true, "an": true, "at": true, "by": true, "for": true,
		"to": true, "up": true, "but": true, "nor": true,
	}
	words := strings.Fields(s)
	for i, word := range words {
		// Preserve trailing punctuation (e.g., comma in "thousand,").
		clean := strings.TrimRight(word, ",.;:")
		suffix := word[len(clean):]
		if strings.Contains(clean, "-") {
			// Capitalise each part of a hyphenated compound.
			parts := strings.Split(clean, "-")
			for j, p := range parts {
				p = strings.ToLower(p)
				if len(p) > 0 {
					parts[j] = strings.ToUpper(p[:1]) + p[1:]
				}
			}
			words[i] = strings.Join(parts, "-") + suffix
		} else {
			lower := strings.ToLower(clean)
			if i == 0 || !exceptions[lower] {
				if len(lower) > 0 {
					words[i] = strings.ToUpper(lower[:1]) + lower[1:] + suffix
				}
			} else {
				words[i] = lower + suffix
			}
		}
	}
	return strings.Join(words, " ")
}

// numberToWordsF64 converts a float64 value (which may be very large, beyond int64)
// to English words using the same algorithm as the JSONata JS reference implementation.
// Uses log10-based magnitude detection, capped at "trillion" (10^12).
// For values <= ~9.2e18, delegates to numberToWords(int64(n)).
func numberToWordsF64(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "infinity"
	}
	if n < 0 {
		return "minus " + numberToWordsF64(-n)
	}
	if n == 0 {
		return "zero"
	}
	// If representable as int64, use the exact integer path.
	const maxInt64F = 9.22e18
	if n <= maxInt64F {
		return numberToWords(int64(math.Round(n)))
	}

	// Determine magnitude using log10, mirroring the JS reference implementation.
	// Cap at 4 (trillion), matching magnitudeNames length.
	magnitudeNames := []string{"thousand", "million", "billion", "trillion"}
	// Use exact float64 literals to avoid math.Pow precision issues.
	magnitudeFactors := []float64{1e3, 1e6, 1e9, 1e12}
	log10n := math.Log10(n)
	mag := int(math.Floor(log10n / 3))
	if mag > len(magnitudeNames) {
		mag = len(magnitudeNames)
	}
	factor := magnitudeFactors[mag-1]

	// Force float64 rounding of the mantissa multiplication to prevent FMA
	// from producing a higher-precision "remainder" that differs from JS behavior.
	mant := math.Floor(n / factor)
	product := mant * factor // explicit float64 multiply → rounded result
	// If the product rounds to the same float64 as n, the remainder is 0.
	// (FMA could give a non-zero "phantom" result in this case.)
	var remainder float64
	if product == n {
		remainder = 0
	} else {
		remainder = n - product
	}

	// Due to float64 rounding, mant may be rounded up giving negative remainder.
	if remainder < 0 {
		remainder = 0
	}

	mantWords := numberToWordsF64(mant)
	result := mantWords + " " + magnitudeNames[mag-1]
	if remainder > 0 {
		remainderWords := numberToWordsF64(remainder)
		if remainder < 100 {
			result += " and " + remainderWords
		} else {
			result += ", " + remainderWords
		}
	}
	return result
}

// numberToWords converts an integer (possibly very large) to English words.
// Uses British English style: "five hundred and fifty-five", commas for large groups.
// For numbers > 10^12 (trillion), uses "X thousand trillion", "X million trillion" etc.
func numberToWords(n int64) string {
	if n == 0 {
		return "zero"
	}
	if n < 0 {
		return "minus " + numberToWords(-n)
	}

	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven",
		"eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen",
		"fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tensWords := []string{"", "", "twenty", "thirty", "forty", "fifty",
		"sixty", "seventy", "eighty", "ninety"}

	// below1000 formats n < 1000.
	var below1000 func(int64) string
	below1000 = func(n int64) string {
		if n == 0 {
			return ""
		}
		if n < 20 {
			return ones[n]
		}
		if n < 100 {
			t := tensWords[n/10]
			if n%10 != 0 {
				t += "-" + ones[n%10]
			}
			return t
		}
		h := ones[n/100] + " hundred"
		rest := n % 100
		if rest == 0 {
			return h
		}
		return h + " and " + below1000(rest)
	}

	// Scale groups: trillion is max named unit. Beyond that, multiply trillion.
	// JSONata uses: "one thousand trillion", "one million trillion trillion", etc.
	// Groups up to trillion (10^12).
	type scaleGroup struct {
		divisor int64
		name    string
	}
	groups := []scaleGroup{
		{1000000000000, "trillion"},
		{1000000000, "billion"},
		{1000000, "million"},
		{1000, "thousand"},
	}

	var parts []string
	remaining := n
	var lastIsSmall bool // whether the final appended part represents a value < 100

	// Handle very large numbers (> trillion) using "X trillion" as unit.
	// JSONata does not use quadrillion/quintillion; instead: "one thousand trillion", etc.
	if remaining >= 1000000000000 {
		trillions := remaining / 1000000000000
		remaining %= 1000000000000
		// Recursive call formats the trillion multiplier.
		trillionWords := numberToWords(trillions) + " trillion"
		parts = append(parts, trillionWords)
		lastIsSmall = false
	}

	// Handle billion, million, thousand below the trillion part.
	for _, g := range groups[1:] { // skip trillion, already handled above
		if remaining >= g.divisor {
			count := remaining / g.divisor
			remaining %= g.divisor
			parts = append(parts, below1000(count)+" "+g.name)
			lastIsSmall = false
		}
	}

	// Handle the final sub-thousand remainder.
	if remaining > 0 {
		parts = append(parts, below1000(remaining))
		lastIsSmall = remaining < 100
	}

	if len(parts) == 0 {
		return "zero"
	}
	if len(parts) == 1 {
		return parts[0]
	}

	// Join: use " and " before the last part when it is a bare sub-100 value.
	// Otherwise use ", ".
	last := parts[len(parts)-1]
	rest := strings.Join(parts[:len(parts)-1], ", ")

	if lastIsSmall {
		return rest + " and " + last
	}
	return rest + ", " + last
}

// makeOrdinalWord converts the last word of a word-form string to its ordinal form.
// e.g., "twelve" → "twelfth", "twenty" → "twentieth", "one hundred" → "one hundredth"
func makeOrdinalWord(s string) string {
	if s == "" {
		return s
	}
	// Split on the last space or hyphen to find the last word.
	lastSep := strings.LastIndexAny(s, " -")
	prefix := ""
	lastWord := s
	sep := ""
	if lastSep >= 0 {
		prefix = s[:lastSep]
		sep = string(s[lastSep])
		lastWord = s[lastSep+1:]
	}

	ordinalMap := map[string]string{
		"one":       "first",
		"two":       "second",
		"three":     "third",
		"four":      "fourth",
		"five":      "fifth",
		"six":       "sixth",
		"seven":     "seventh",
		"eight":     "eighth",
		"nine":      "ninth",
		"ten":       "tenth",
		"eleven":    "eleventh",
		"twelve":    "twelfth",
		"thirteen":  "thirteenth",
		"fourteen":  "fourteenth",
		"fifteen":   "fifteenth",
		"sixteen":   "sixteenth",
		"seventeen": "seventeenth",
		"eighteen":  "eighteenth",
		"nineteen":  "nineteenth",
		"twenty":    "twentieth",
		"thirty":    "thirtieth",
		"forty":     "fortieth",
		"fifty":     "fiftieth",
		"sixty":     "sixtieth",
		"seventy":   "seventieth",
		"eighty":    "eightieth",
		"ninety":    "ninetieth",
		"hundred":   "hundredth",
		"thousand":  "thousandth",
		"million":   "millionth",
		"billion":   "billionth",
		"trillion":  "trillionth",
		"zero":      "zeroth",
	}

	// Check lowercase version.
	lw := strings.ToLower(lastWord)

	// Check the ordinal map.
	if ord, ok := ordinalMap[lw]; ok {
		// Preserve case: if lastWord was ALL CAPS, make ordinal all caps.
		if lastWord == strings.ToUpper(lastWord) && len(lastWord) > 0 {
			ord = strings.ToUpper(ord)
		} else if lastWord[0] >= 'A' && lastWord[0] <= 'Z' {
			ord = strings.ToUpper(ord[:1]) + ord[1:]
		}
		if prefix == "" {
			return ord
		}
		return prefix + sep + ord
	}

	// For Roman numerals or alphabetic, just append "th" etc.
	// (These are less common; treat as opaque.)
	return s
}

// parseWordsToNumber converts English word form to a number.
// Uses the same algorithm as the JSONata JS reference wordsToNumber().
// Returns a float64 to support very large values like 1e46.
func parseWordsToNumber(s string) (float64, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	// Build word → value map including ordinals.
	// Mirrors the JS dateTime module's wordValues construction.
	wordVals := map[string]float64{
		// Cardinal small numbers (ordinally: first=1, second=2, etc.)
		"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11,
		"twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
		"seventeen": 17, "eighteen": 18, "nineteen": 19,
		// Ordinal forms of 1..19
		"zeroth": 0, "first": 1, "second": 2, "third": 3, "fourth": 4, "fifth": 5,
		"sixth": 6, "seventh": 7, "eighth": 8, "ninth": 9, "tenth": 10,
		"eleventh": 11, "twelfth": 12, "thirteenth": 13, "fourteenth": 14,
		"fifteenth": 15, "sixteenth": 16, "seventeenth": 17, "eighteenth": 18,
		"nineteenth": 19,
		// Decades and their ordinals
		"twenty": 20, "twenty-ieth": 20, "twentieth": 20,
		"thirty": 30, "thirty-ieth": 30, "thirtieth": 30,
		"forty": 40, "forty-ieth": 40, "fortieth": 40,
		"fifty": 50, "fifty-ieth": 50, "fiftieth": 50,
		"sixty": 60, "sixty-ieth": 60, "sixtieth": 60,
		"seventy": 70, "seventy-ieth": 70, "seventieth": 70,
		"eighty": 80, "eighty-ieth": 80, "eightieth": 80,
		"ninety": 90, "ninety-ieth": 90, "ninetieth": 90,
		// Scale words and their ordinals
		"hundred": 100, "hundredth": 100,
		"thousand": 1e3, "thousandth": 1e3,
		"million": 1e6, "millionth": 1e6,
		"billion": 1e9, "billionth": 1e9,
		"trillion": 1e12, "trillionth": 1e12,
		// Quadrillion/quintillion for legacy parseWordsToNumber compatibility.
		"quadrillion": 1e15, "quadrillionth": 1e15,
		"quintillion": 1e18, "quintillionth": 1e18,
	}

	// Split on ", " or " and " or whitespace or hyphen (same as JS regex: /,\s|\sand\s|[\s\-]/).
	parts := splitWordString(s)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty word string")
	}

	// Look up each part. Return error for unknown words.
	values := make([]float64, len(parts))
	for i, p := range parts {
		v, ok := wordVals[p]
		if !ok {
			return 0, fmt.Errorf("unknown word: %s", p)
		}
		values[i] = v
	}

	// Process values using the JS segment accumulator algorithm.
	segs := []float64{0}
	for _, val := range values {
		if val < 100 {
			top := segs[len(segs)-1]
			segs = segs[:len(segs)-1]
			if top >= 1000 {
				segs = append(segs, top)
				top = 0
			}
			segs = append(segs, top+val)
		} else {
			top := segs[len(segs)-1]
			segs = segs[:len(segs)-1]
			segs = append(segs, top*val)
		}
	}

	// Sum all segments.
	result := 0.0
	for _, seg := range segs {
		result += seg
	}
	return result, nil
}

// splitWordString splits a word number string the same way as the JS reference:
// splits on ", ", " and ", whitespace, and hyphen.
func splitWordString(s string) []string {
	// Normalize: replace ", " → " ", " and " → " ", "-" → " ", then split on spaces.
	s = strings.ReplaceAll(s, ", ", " ")
	s = strings.ReplaceAll(s, " and ", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.Join(strings.Fields(s), " ")
	parts := strings.Fields(s)
	// Filter empty parts.
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// deordinalizeWords converts ordinal word forms back to cardinal.
func deordinalizeWords(s string) string {
	ordToCard := map[string]string{
		"first": "one", "second": "two", "third": "three", "fourth": "four",
		"fifth": "five", "sixth": "six", "seventh": "seven", "eighth": "eight",
		"ninth": "nine", "tenth": "ten", "eleventh": "eleven", "twelfth": "twelve",
		"thirteenth": "thirteen", "fourteenth": "fourteen", "fifteenth": "fifteen",
		"sixteenth": "sixteen", "seventeenth": "seventeen", "eighteenth": "eighteen",
		"nineteenth": "nineteen", "twentieth": "twenty", "thirtieth": "thirty",
		"fortieth": "forty", "fiftieth": "fifty", "sixtieth": "sixty",
		"seventieth": "seventy", "eightieth": "eighty", "ninetieth": "ninety",
		"hundredth": "hundred", "thousandth": "thousand",
		"millionth": "million", "billionth": "billion", "trillionth": "trillion",
		"zeroth": "zero",
	}
	words := strings.Fields(s)
	for i, w := range words {
		if card, ok := ordToCard[w]; ok {
			words[i] = card
		}
	}
	return strings.Join(words, " ")
}

// parseIntegerFromPattern parses a string using the given decimal format pattern.
// Supports grouping separators, Unicode digits, and ordinal suffix stripping.
func parseIntegerFromPattern(str, pattern string) (float64, error) {
	parts := strings.SplitN(pattern, ";", 2)
	token := parts[0]
	modifier := ""
	if len(parts) == 2 {
		modifier = parts[1]
	}
	ordinal := modifier == "o"

	switch token {
	case "i", "I":
		if str == "" {
			return 0, nil
		}
		return float64(parseRomanNumeral(str)), nil
	case "w", "W", "Ww":
		v, err := parseWordsToNumber(str)
		return v, err
	case "a", "A":
		if ordinal {
			// Strip ordinal suffix from alphabetic (not standard, but handle gracefully).
			str = strings.TrimRight(str, "stndrh")
		}
		n := parseAlphabeticNumber(str)
		if n == 0 {
			return 0, fmt.Errorf("cannot parse '%s' as alphabetic", str)
		}
		return float64(n), nil
	default:
		// Decimal pattern: strip ordinal suffix if needed, strip grouping separators.
		if ordinal {
			str = stripOrdinalSuffix(str)
		}
		// Determine zero-digit and validate the pattern.
		runes := []rune(token)
		var zeroDigit rune = '0'
		hasMandatory := false
		for _, r := range runes {
			if r >= '0' && r <= '9' {
				zeroDigit = '0'
				hasMandatory = true
				break
			}
			if family := getZeroDigitOfFamily(r); family != 0 {
				zeroDigit = family
				hasMandatory = true
				break
			}
		}
		// If no decimal digit chars in pattern, it's a sequence format (D3130).
		if !hasMandatory {
			return 0, fmt.Errorf("D3130: $parseInteger() does not support this numbering sequence: %s", token)
		}
		// Convert Unicode digits to ASCII.
		if zeroDigit != '0' {
			var converted strings.Builder
			for _, r := range str {
				if r >= zeroDigit && r < zeroDigit+10 {
					converted.WriteRune('0' + (r - zeroDigit))
				} else {
					converted.WriteRune(r)
				}
			}
			str = converted.String()
		}
		// Strip all non-digit chars (grouping separators, etc.).
		var digits strings.Builder
		for _, r := range str {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		n, err := strconv.ParseInt(digits.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("D3137: cannot parse '%s' as integer", str)
		}
		return float64(n), nil
	}
}

// stripOrdinalSuffix removes "st", "nd", "rd", "th" from the end of a numeric string.
func stripOrdinalSuffix(s string) string {
	suffixes := []string{"st", "nd", "rd", "th"}
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return strings.TrimSuffix(s, suf)
		}
	}
	return s
}

// fnParseInteger parses a string to an integer using an optional picture string.
// Signature: $parseInteger(string [, picture])

func fnParseInteger(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := strings.TrimSpace(e.toString(args[0]))

	if len(args) == 1 {
		// No picture: parse decimal integer.
		n, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("D3137: cannot parse '%s' as integer", str)
		}
		return float64(n), nil
	}

	picture := strings.TrimSpace(e.toString(args[1]))

	// Check if picture is a simple numeric radix (legacy mode: $parseInteger(str, 16) etc.).
	// Detect by checking if picture is a decimal integer string (radix).
	if radix, err := strconv.Atoi(picture); err == nil && radix >= 2 && radix <= 36 {
		n, err := strconv.ParseInt(str, radix, 64)
		if err != nil {
			return nil, fmt.Errorf("D3137: cannot parse '%s' as integer with radix %d", str, radix)
		}
		return float64(n), nil
	}

	n, err := parseIntegerFromPattern(str, picture)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// --- Enhanced Array Functions (Fase 5.2) ---

// fnDistinct removes duplicate values from an array.
// Signature: $distinct(array)
