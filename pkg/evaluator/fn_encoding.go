package evaluator

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"github.com/sandrolain/gosonata/pkg/types"
)

func fnBase64Encode(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	encoded := base64.StdEncoding.EncodeToString([]byte(str))
	return encoded, nil
}

// fnBase64Decode decodes a base64 string.
// Signature: $base64decode(string)

func fnBase64Decode(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("D3137: invalid base64 string: %w", err)
	}
	return string(decoded), nil
}

// fnEncodeUrl encodes a URL string (like JS encodeURI).
// Signature: $encodeUrl(string)
// Encodes all chars except: letters, digits and -_.!~*'();/?:@&=+$,#%

func fnEncodeUrl(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	str := e.toString(args[0])
	return encodeURIJS(str, false)
}

// encodeURIJS implements JS encodeURI or encodeURIComponent semantics.
// isComponent=false: encodeURI - preserves ;/?:@&=+$,#%
// isComponent=true: encodeURIComponent - encodes those too

func encodeURIJS(str string, isComponent bool) (string, error) {
	// Characters not encoded by encodeURI:
	const encodeURIExcluded = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'();/?:@&=+$,#%"
	// Characters not encoded by encodeURIComponent:
	const encodeURIComponentExcluded = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'()"

	excluded := encodeURIExcluded
	if isComponent {
		excluded = encodeURIComponentExcluded
	}

	// Check for lone surrogates (U+D800-U+DFFF)
	// These appear in Go strings as replacement character U+FFFD (EF BF BD)
	// or as the raw surrogate bytes in invalid UTF-8
	for _, r := range str {
		if r == '\uFFFD' {
			// Could be a replacement for a lone surrogate
			return "", types.NewError("D3140", fmt.Sprintf("The argument of function encodeUrl contains an unpaired surrogate: %q", str), -1)
		}
		if r >= 0xD800 && r <= 0xDFFF {
			return "", types.NewError("D3140", fmt.Sprintf("The argument of function encodeUrl contains an unpaired surrogate: %q", str), -1)
		}
	}

	var buf strings.Builder
	bytes := []byte(str)
	for _, b := range bytes {
		if strings.ContainsRune(excluded, rune(b)) {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String(), nil
}

// fnDecodeUrl decodes a URL string.
// Signature: $decodeUrl(string)

func fnDecodeUrl(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	decoded, err := url.PathUnescape(str)
	if err != nil {
		return nil, fmt.Errorf("D3137: invalid URL encoding: %w", err)
	}
	return decoded, nil
}

// fnEncodeUrlComponent encodes a URL component (like JS encodeURIComponent).
// Signature: $encodeUrlComponent(string)

func fnEncodeUrlComponent(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}
	str := e.toString(args[0])
	result, err := encodeURIJS(str, true)
	if err != nil {
		// Change error message to mention encodeUrlComponent
		return nil, types.NewError("D3140", fmt.Sprintf("The argument of function encodeUrlComponent contains an unpaired surrogate: %q", str), -1)
	}
	return result, nil
}

// fnDecodeUrlComponent decodes a URL component.
// Signature: $decodeUrlComponent(string)

func fnDecodeUrlComponent(ctx context.Context, e *Evaluator, evalCtx *EvalContext, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return nil, nil
	}

	str := e.toString(args[0])
	decoded, err := url.QueryUnescape(str)
	if err != nil {
		return nil, fmt.Errorf("D3137: invalid URL component encoding: %w", err)
	}
	return decoded, nil
}

// --- Number Formatting Functions (Fase 5.3) ---

// fnFormatNumber formats a number with optional picture string and decimal format.
// Signature: $formatNumber(number [, picture [, options]])
// Simplified implementation without full XPath picture string support.
