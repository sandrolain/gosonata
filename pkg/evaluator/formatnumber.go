package evaluator

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DecimalFormat defines the symbols used in a FormatNumber picture string.
// Based on XPath/XSLT decimal format specification.
type DecimalFormat struct {
	DecimalSeparator  rune
	GroupSeparator    rune
	ExponentSeparator rune
	MinusSign         rune
	Infinity          string
	NaN               string
	Percent           string
	PerMille          string
	ZeroDigit         rune
	OptionalDigit     rune
	PatternSeparator  rune
}

// NewDecimalFormat returns a new DecimalFormat with default settings.
func NewDecimalFormat() DecimalFormat {
	return DecimalFormat{
		DecimalSeparator:  '.',
		GroupSeparator:    ',',
		ExponentSeparator: 'e',
		MinusSign:         '-',
		Infinity:          "Infinity",
		NaN:               "NaN",
		Percent:           "%",
		PerMille:          "‰",
		ZeroDigit:         '0',
		OptionalDigit:     '#',
		PatternSeparator:  ';',
	}
}

func (df *DecimalFormat) isZeroDigit(r rune) bool {
	return r == df.ZeroDigit
}

func (df *DecimalFormat) isDecimalDigit(r rune) bool {
	r -= df.ZeroDigit
	return r >= 0 && r <= 9
}

func (df *DecimalFormat) isDigit(r rune) bool {
	return r == df.OptionalDigit || df.isDecimalDigit(r)
}

func (df *DecimalFormat) isActive(r rune) bool {
	switch r {
	case df.DecimalSeparator, df.GroupSeparator, df.OptionalDigit, df.ExponentSeparator:
		return true
	}
	return df.isDecimalDigit(r)
}

// formatConfig holds the computed formatting configuration for a picture pattern.
type formatConfig struct {
	NumericType            int
	IntGroupPositions      []int
	GroupingInterval       int
	MinIntDigits           int
	ScaleFactor            int
	FracGroupPositions     []int
	MinFracDigits          int
	MaxFracDigits          int
	MinExpDigits           int
	PrefixText             string
	SuffixText             string
}

const (
	numTypeNormal = iota
	numTypePercent
	numTypePermille
)

// FormatNumberWithPicture formats a number using an XPath picture string.
func FormatNumberWithPicture(value float64, picture string, format DecimalFormat) (string, error) {
	if math.IsInf(value, 0) {
		if value > 0 {
			return format.Infinity, nil
		}
		return string(format.MinusSign) + format.Infinity, nil
	}

	if math.IsNaN(value) {
		return format.NaN, nil
	}

	isNegative := value < 0

	cfg, err := parsePictureString(picture, &format, isNegative)
	if err != nil {
		return "", err
	}

	switch cfg.NumericType {
	case numTypePercent:
		value *= 100
	case numTypePermille:
		value *= 1000
	}

	var exponent int
	if cfg.MinExpDigits == 0 {
		exponent = 0
	} else {
		minMantissa := math.Pow(10, float64(cfg.ScaleFactor-1))
		maxMantissa := math.Pow(10, float64(cfg.ScaleFactor))

		for value != 0 && math.Abs(value) < minMantissa {
			value *= 10
			exponent--
		}

		for math.Abs(value) >= maxMantissa {
			value /= 10
			exponent++
		}
	}

	var intPart, fracPart, expPart string

	value = roundToDecimalPlaces(value, cfg.MaxFracDigits)
	numStr := numberToCustomDigits(value, cfg.MaxFracDigits, &format)
	intStr, fracStr := splitAtByte(numStr, '.')
	if intStr != "" {
		intPart = formatIntegerDigits(intStr, &cfg, &format)
	}
	if fracStr != "" {
		fracPart = formatDecimalDigits(fracStr, &cfg, &format)
	}

	if cfg.MinExpDigits != 0 {
		expStr := numberToCustomDigits(float64(exponent), 0, &format)
		expPart = formatExponentDigits(expStr, &cfg, &format)
	}

	buf := make([]byte, 0, 128)
	buf = append(buf, cfg.PrefixText...)
	buf = append(buf, intPart...)

	if len(fracPart) > 0 {
		buf = append(buf, string(format.DecimalSeparator)...)
		buf = append(buf, fracPart...)
	}

	if len(expPart) > 0 {
		buf = append(buf, string(format.ExponentSeparator)...)
		if exponent < 0 {
			buf = append(buf, string(format.MinusSign)...)
		}
		buf = append(buf, expPart...)
	}

	buf = append(buf, cfg.SuffixText...)

	return string(buf), nil
}

func parsePictureString(picture string, format *DecimalFormat, isNegative bool) (formatConfig, error) {
	pattern1, pattern2 := splitAtRune(picture, format.PatternSeparator)
	if pattern1 == "" {
		return formatConfig{}, fmt.Errorf("D3080: picture string must contain 1 or 2 subpictures")
	}

	cfg1, err := parsePicturePattern(pattern1, format)
	if err != nil {
		return formatConfig{}, err
	}

	var cfg2 formatConfig
	if pattern2 != "" {
		cfg2, err = parsePicturePattern(pattern2, format)
		if err != nil {
			return formatConfig{}, err
		}
	}

	cfg := cfg1
	if isNegative {
		if pattern2 != "" {
			cfg = cfg2
		} else {
			cfg.PrefixText = string(format.MinusSign) + cfg.PrefixText
		}
	}

	return cfg, nil
}

// pictureComponents holds the decomposed parts of a picture pattern.
type pictureComponents struct {
	PrefixPart     string
	SuffixPart     string
	ActivePart     string
	MantissaPart   string
	ExponentPart   string
	IntegerPart    string
	FractionalPart string
	FullPattern    string
}

func parsePicturePattern(pattern string, format *DecimalFormat) (formatConfig, error) {
	components := splitPictureComponents(pattern, format)
	err := validateComponents(components, format)
	if err != nil {
		return formatConfig{}, err
	}

	return computeFormatConfig(components, format), nil
}

func splitPictureComponents(pattern string, format *DecimalFormat) pictureComponents {
	// Find prefix (passive characters before first active)
	prefixEnd := 0
	for i, r := range pattern {
		if format.isActive(r) || r == format.ExponentSeparator {
			prefixEnd = i
			break
		}
	}

	// Find suffix (passive characters after last active)
	suffixStart := len(pattern)
	for i := len(pattern); i > 0; {
		r, size := utf8.DecodeLastRuneInString(pattern[:i])
		if format.isActive(r) || r == format.ExponentSeparator {
			suffixStart = i
			break
		}
		i -= size
	}

	prefix := pattern[:prefixEnd]
	suffix := pattern[suffixStart:]
	activePart := pattern[prefixEnd:suffixStart]

	// Split active part by exponent separator
	mantissa := activePart
	var exponent string

	expIdx := strings.IndexRune(activePart, format.ExponentSeparator)
	if expIdx != -1 {
		mantissa = activePart[:expIdx]
		exponent = activePart[expIdx+1:]
	}

	// Split mantissa by decimal separator
	integerPart := mantissa
	var fractionalPart string

	decIdx := strings.IndexRune(mantissa, format.DecimalSeparator)
	if decIdx != -1 {
		integerPart = mantissa[:decIdx]
		fractionalPart = mantissa[decIdx+1:]
	}

	return pictureComponents{
		PrefixPart:     prefix,
		SuffixPart:     suffix,
		ActivePart:     activePart,
		MantissaPart:   mantissa,
		ExponentPart:   exponent,
		IntegerPart:    integerPart,
		FractionalPart: fractionalPart,
		FullPattern:    pattern,
	}
}

func validateComponents(comp pictureComponents, format *DecimalFormat) error {
	if strings.Count(comp.FullPattern, string(format.DecimalSeparator)) > 1 {
		return fmt.Errorf("D3081: subpicture cannot contain more than one decimal separator")
	}

	percentCount := strings.Count(comp.FullPattern, format.Percent)
	if percentCount > 1 {
		return fmt.Errorf("D3082: subpicture cannot contain more than one percent character")
	}

	permilleCount := strings.Count(comp.FullPattern, format.PerMille)
	if permilleCount > 1 {
		return fmt.Errorf("D3083: subpicture cannot contain more than one per-mille character")
	}

	if percentCount > 0 && permilleCount > 0 {
		return fmt.Errorf("D3084: subpicture cannot contain both percent and per-mille characters")
	}

	if strings.IndexFunc(comp.MantissaPart, func(r rune) bool {
		return format.isDigit(r)
	}) == -1 {
		return fmt.Errorf("D3085: mantissa part must contain at least one digit")
	}

	isPassive := func(r rune) bool {
		return !format.isActive(r)
	}
	if strings.IndexFunc(comp.ActivePart, isPassive) != -1 {
		return fmt.Errorf("D3086: subpicture cannot contain passive character between active characters")
	}

	if lastRune(comp.IntegerPart) == format.GroupSeparator ||
		firstRune(comp.FractionalPart) == format.GroupSeparator {
		return fmt.Errorf("D3087: group separator cannot be adjacent to decimal separator")
	}

	if strings.Contains(comp.FullPattern, string([]rune{format.GroupSeparator, format.GroupSeparator})) {
		return fmt.Errorf("D3088: subpicture cannot contain adjacent group separators")
	}

	isDecDigit := func(r rune) bool {
		return format.isDecimalDigit(r)
	}

	pos := strings.IndexFunc(comp.IntegerPart, isDecDigit)
	if pos != -1 {
		pos += utf8.RuneLen(format.ZeroDigit)
		if strings.ContainsRune(comp.IntegerPart[pos:], format.OptionalDigit) {
			return fmt.Errorf("D3089: integer part cannot contain decimal digit followed by optional digit")
		}
	}

	pos = strings.IndexRune(comp.FractionalPart, format.OptionalDigit)
	if pos != -1 {
		pos += utf8.RuneLen(format.OptionalDigit)
		if strings.IndexFunc(comp.FractionalPart[pos:], isDecDigit) != -1 {
			return fmt.Errorf("D3090: fractional part cannot contain optional digit followed by decimal digit")
		}
	}

	exponentCount := strings.Count(comp.FullPattern, string(format.ExponentSeparator))
	if exponentCount > 1 {
		return fmt.Errorf("D3091: subpicture cannot contain more than one exponent separator")
	}

	if exponentCount > 0 && (percentCount > 0 || permilleCount > 0) {
		return fmt.Errorf("D3092: subpicture cannot contain percent/per-mille and exponent separator")
	}

	if exponentCount > 0 {
		isNotDecDigit := func(r rune) bool {
			return !format.isDecimalDigit(r)
		}
		if strings.IndexFunc(comp.ExponentPart, isNotDecDigit) != -1 {
			return fmt.Errorf("D3093: exponent part must consist solely of decimal digits")
		}
	}

	return nil
}

func computeFormatConfig(comp pictureComponents, format *DecimalFormat) formatConfig {
	var numType int
	switch {
	case strings.Contains(comp.FullPattern, format.Percent):
		numType = numTypePercent
	case strings.Contains(comp.FullPattern, format.PerMille):
		numType = numTypePermille
	}

	isDigit := func(r rune) bool {
		return format.isDigit(r)
	}
	isDecDigit := func(r rune) bool {
		return format.isDecimalDigit(r)
	}

	intGroupPos := findGroupingSeparators(comp.IntegerPart, format.GroupSeparator, isDigit, false)
	fracGroupPos := findGroupingSeparators(comp.FractionalPart, format.GroupSeparator, isDigit, true)
	groupInterval := calculateGroupingInterval(intGroupPos)

	minIntDigits := countRunesWhere(comp.IntegerPart, isDecDigit)
	scaleFactor := minIntDigits

	minFracDigits := countRunesWhere(comp.FractionalPart, isDecDigit)
	maxFracDigits := countRunesWhere(comp.FractionalPart, isDigit)

	if minIntDigits == 0 && maxFracDigits == 0 {
		if comp.ExponentPart != "" {
			minFracDigits = 1
			maxFracDigits = 1
		} else {
			minIntDigits = 1
		}
	}

	if comp.ExponentPart != "" && minIntDigits == 0 && strings.ContainsRune(comp.IntegerPart, format.OptionalDigit) {
		minIntDigits = 1
	}

	if minIntDigits == 0 && minFracDigits == 0 {
		minFracDigits = 1
	}

	minExpDigits := 0
	if comp.ExponentPart != "" {
		minExpDigits = countRunesWhere(comp.ExponentPart, isDecDigit)
	}

	return formatConfig{
		NumericType:        numType,
		IntGroupPositions:  intGroupPos,
		GroupingInterval:   groupInterval,
		MinIntDigits:       minIntDigits,
		ScaleFactor:        scaleFactor,
		FracGroupPositions: fracGroupPos,
		MinFracDigits:      minFracDigits,
		MaxFracDigits:      maxFracDigits,
		MinExpDigits:       minExpDigits,
		PrefixText:         comp.PrefixPart,
		SuffixText:         comp.SuffixPart,
	}
}

func findGroupingSeparators(s string, sep rune, predicate func(rune) bool, lookLeft bool) []int {
	var positions []int

	for {
		idx := strings.IndexRune(s, sep)
		if idx == -1 {
			break
		}

		sepLen := utf8.RuneLen(sep)
		remainder := s[idx+sepLen:]
		positions = append(positions, countRunesWhere(remainder, predicate))

		if lookLeft {
			if l := len(positions); l > 1 {
				positions[l-1] += positions[l-2]
			}
		}

		s = s[idx+sepLen:]
	}

	return positions
}

func calculateGroupingInterval(positions []int) int {
	if len(positions) == 0 {
		return 0
	}

	commonDivisor := gcdSlice(positions)
	for i := 0; i < len(positions); i++ {
		if findInt(positions, commonDivisor*(i+1)) == -1 {
			return 0
		}
	}

	return commonDivisor
}

func formatIntegerDigits(integerStr string, cfg *formatConfig, format *DecimalFormat) string {
	integerStr = strings.TrimLeftFunc(integerStr, func(r rune) bool {
		return format.isZeroDigit(r)
	})

	paddingRequired := cfg.MinIntDigits - utf8.RuneCountInString(integerStr)

	if paddingRequired > 0 {
		integerStr = strings.Repeat(string(format.ZeroDigit), paddingRequired) + integerStr
	}

	if cfg.GroupingInterval > 0 {
		return addPeriodicSeparators(integerStr, format.GroupSeparator, cfg.GroupingInterval)
	}

	if len(cfg.IntGroupPositions) > 0 {
		return addSeparatorsAtPositions(integerStr, format.GroupSeparator, cfg.IntGroupPositions, true)
	}

	return integerStr
}

func formatDecimalDigits(fracStr string, cfg *formatConfig, format *DecimalFormat) string {
	fracStr = strings.TrimRightFunc(fracStr, func(r rune) bool {
		return format.isZeroDigit(r)
	})

	paddingRequired := cfg.MinFracDigits - utf8.RuneCountInString(fracStr)

	if paddingRequired > 0 {
		fracStr += strings.Repeat(string(format.ZeroDigit), paddingRequired)
	}

	if len(cfg.FracGroupPositions) > 0 {
		return addSeparatorsAtPositions(fracStr, format.GroupSeparator, cfg.FracGroupPositions, false)
	}

	return fracStr
}

func formatExponentDigits(expStr string, cfg *formatConfig, format *DecimalFormat) string {
	paddingRequired := cfg.MinExpDigits - utf8.RuneCountInString(expStr)

	if paddingRequired > 0 {
		expStr = strings.Repeat(string(format.ZeroDigit), paddingRequired) + expStr
	}

	return expStr
}

func numberToCustomDigits(value float64, precision int, format *DecimalFormat) string {
	byteStr := strconv.AppendFloat(make([]byte, 0, 24), math.Abs(value), 'f', precision, 64)

	if format.ZeroDigit != '0' {
		byteStr = bytes.Map(func(r rune) rune {
			offset := r - '0'
			if offset < 0 || offset > 9 {
				return r
			}
			return format.ZeroDigit + offset
		}, byteStr)
	}

	return string(byteStr)
}

func addPeriodicSeparators(s string, sep rune, interval int) string {
	runeCount := utf8.RuneCountInString(s)
	if interval <= 0 || runeCount <= interval {
		return s
	}

	endPos := len(s)
	chunkCount := (runeCount - 1) / interval
	chunks := make([]string, chunkCount+1)

	for chunkCount > 0 {
		bytePos := 0
		for i := 0; i < interval; i++ {
			_, width := utf8.DecodeLastRuneInString(s[:endPos])
			bytePos += width
		}
		chunks[chunkCount] = s[endPos-bytePos : endPos]
		endPos -= bytePos
		chunkCount--
	}

	chunks[chunkCount] = s[:endPos]
	return strings.Join(chunks, string(sep))
}

func addSeparatorsAtPositions(s string, sep rune, positions []int, fromRight bool) string {
	chunks := make([]string, 0, len(positions)+1)

	for i := range positions {
		runeNum := positions[i]
		if fromRight {
			runeNum = utf8.RuneCountInString(s) - runeNum
		}

		bytePos := 0
		for runeNum > 0 && bytePos < len(s) {
			_, width := utf8.DecodeRuneInString(s[bytePos:])
			bytePos += width
			runeNum--
		}

		chunks = append(chunks, s[:bytePos])
		s = s[bytePos:]
	}

	chunks = append(chunks, s)
	return strings.Join(chunks, string(sep))
}

func splitAtRune(s string, r rune) (string, string) {
	idx := strings.IndexRune(s, r)
	if idx == -1 {
		return s, ""
	}

	if remaining := s[idx+utf8.RuneLen(r):]; !strings.ContainsRune(remaining, r) {
		return s[:idx], remaining
	}

	return "", ""
}

func splitAtByte(s string, b byte) (string, string) {
	idx := strings.IndexByte(s, b)
	if idx == -1 {
		return s, ""
	}

	if remaining := s[idx+1:]; strings.IndexByte(remaining, b) == -1 {
		return s[:idx], remaining
	}

	return "", ""
}

func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func countRunesWhere(s string, predicate func(rune) bool) int {
	var count int
	for _, r := range s {
		if predicate(r) {
			count++
		}
	}
	return count
}

func gcd(a, b int) int {
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

func gcdSlice(numbers []int) int {
	result := 0
	for _, num := range numbers {
		result = gcd(result, num)
	}
	return result
}

func findInt(numbers []int, target int) int {
	for idx, num := range numbers {
		if num == target {
			return idx
		}
	}
	return -1
}

func roundToDecimalPlaces(x float64, precision int) float64 {
	if x == 0 {
		return 0
	}
	if precision >= 0 && x == math.Trunc(x) {
		return x
	}
	multiplier := math.Pow10(precision)
	scaled := x * multiplier
	if math.IsInf(scaled, 0) {
		return x
	}
	if x < 0 {
		x = math.Ceil(scaled - 0.5)
	} else {
		x = math.Floor(scaled + 0.5)
	}
	if x == 0 {
		return 0
	}
	return x / multiplier
}
