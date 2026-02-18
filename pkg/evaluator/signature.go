package evaluator

import (
	"fmt"
	"strings"
)

// TypeCode represents a JSONata type code in signatures
type TypeCode string

const (
	TypeAny      TypeCode = "x" // any type
	TypeString   TypeCode = "s" // string
	TypeNumber   TypeCode = "n" // number
	TypeBoolean  TypeCode = "b" // boolean
	TypeNull     TypeCode = "l" // null
	TypeArray    TypeCode = "a" // array
	TypeObject   TypeCode = "o" // object
	TypeFunction TypeCode = "f" // function
)

// ParamType represents a parameter type in a signature
type ParamType struct {
	Type       TypeCode
	SubType    *ParamType  // For arrays like a<n> or functions like f<n:n>
	UnionTypes []TypeCode  // For union types like (ns) = number OR string
	FuncParams []ParamType // For function subtypes f<n-s:b>
	FuncReturn *ParamType  // For function return type
	Optional   bool
}

// Signature represents a parsed function signature
type Signature struct {
	Params     []ParamType
	ReturnType *ParamType
}

// ParseSignature parses a JSONata function signature string
// Examples: "<n-n:n>", "<s-s>", "<a<s>s?:s>", "<f<n:n>:f<n:n>>"
func ParseSignature(sig string) (*Signature, error) {
	if sig == "" {
		return nil, nil
	}

	// Remove < and >
	if !strings.HasPrefix(sig, "<") || !strings.HasSuffix(sig, ">") {
		return nil, fmt.Errorf("S0401: Invalid signature format")
	}

	sig = sig[1 : len(sig)-1]

	// Split by : to separate params from return type
	// But we need to respect nested brackets, so can't use strings.Split
	parts := splitByColonRespectingBrackets(sig)
	if len(parts) > 2 {
		return nil, fmt.Errorf("S0401: Invalid signature format")
	}

	result := &Signature{}

	// Parse parameters
	if len(parts) > 0 && parts[0] != "" {
		params, err := parseParamList(parts[0])
		if err != nil {
			return nil, err
		}
		result.Params = params
	}

	// Parse return type
	if len(parts) == 2 {
		returnType, err := parseParamType(parts[1])
		if err != nil {
			return nil, err
		}
		result.ReturnType = returnType
	}

	return result, nil
}

// splitByColonRespectingBrackets splits a string by : but respects nested < >
func splitByColonRespectingBrackets(s string) []string {
	var parts []string
	depth := 0
	start := 0

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ':':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}

	// Add remaining part
	if start < len(s) {
		parts = append(parts, s[start:])
	}

	return parts
}

// parseParamList parses the parameter list part of a signature
func parseParamList(params string) ([]ParamType, error) {
	var result []ParamType
	i := 0

	for i < len(params) {
		paramType, consumed, err := parseParamTypeAt(params, i)
		if err != nil {
			return nil, err
		}
		result = append(result, *paramType)
		i += consumed

		// Skip optional separator '-'
		if i < len(params) && params[i] == '-' {
			i++
		}
	}

	return result, nil
}

// parseParamTypeAt parses a parameter type starting at position i
// Returns the parsed type, number of characters consumed, and error
func parseParamTypeAt(s string, i int) (*ParamType, int, error) {
	if i >= len(s) {
		return nil, 0, fmt.Errorf("S0401: Unexpected end of signature")
	}

	start := i
	paramType := &ParamType{}

	// Check for union type (ns) = number OR string
	if s[i] == '(' {
		// Find closing )
		j := i + 1
		for j < len(s) && s[j] != ')' {
			j++
		}
		if j >= len(s) {
			return nil, 0, fmt.Errorf("S0401: Unmatched ( in signature")
		}

		// Parse union types
		unionStr := s[i+1 : j]
		for _, char := range unionStr {
			typeCode := TypeCode(string(char))
			switch typeCode {
			case TypeAny, TypeString, TypeNumber, TypeBoolean, TypeNull, TypeArray, TypeObject, TypeFunction:
				paramType.UnionTypes = append(paramType.UnionTypes, typeCode)
			default:
				return nil, 0, fmt.Errorf("S0401: Unknown type code in union: %s", typeCode)
			}
		}
		// Use first type as main type for now
		if len(paramType.UnionTypes) > 0 {
			paramType.Type = paramType.UnionTypes[0]
		}
		i = j + 1

		// Check for optional marker after union type
		if i < len(s) && s[i] == '?' {
			paramType.Optional = true
			i++
		}

		return paramType, i - start, nil
	}

	// Get the base type code
	typeCode := TypeCode(s[i : i+1])
	i++

	// Validate type code
	switch typeCode {
	case TypeAny, TypeString, TypeNumber, TypeBoolean, TypeNull, TypeArray, TypeObject, TypeFunction:
		paramType.Type = typeCode
	default:
		return nil, 0, fmt.Errorf("S0401: Unknown type code: %s", typeCode)
	}

	// Check for subtype (e.g., a<n> for array of numbers, f<n:n> for function)
	if i < len(s) && s[i] == '<' {
		// Only arrays and functions can have subtypes
		if typeCode != TypeArray && typeCode != TypeFunction {
			return nil, 0, fmt.Errorf("S0401: Type %s cannot have subtypes", typeCode)
		}

		// Find matching >
		depth := 1
		j := i + 1
		for j < len(s) && depth > 0 {
			if s[j] == '<' {
				depth++
			} else if s[j] == '>' {
				depth--
			}
			j++
		}

		if depth != 0 {
			return nil, 0, fmt.Errorf("S0401: Unmatched < in signature")
		}

		subSig := s[i+1 : j-1]
		if subSig == "" {
			return nil, 0, fmt.Errorf("S0401: Empty subtype")
		}

		if typeCode == TypeFunction {
			// Parse function signature: f<params:return>
			// Split by : to get params and return type
			parts := strings.Split(subSig, ":")
			if len(parts) != 2 {
				return nil, 0, fmt.Errorf("S0401: Function signature must have format f<params:return>")
			}

			// Parse function parameters
			if parts[0] != "" {
				funcParams, err := parseParamList(parts[0])
				if err != nil {
					return nil, 0, err
				}
				paramType.FuncParams = funcParams
			}

			// Parse function return type
			if parts[1] != "" {
				funcReturn, err := parseParamType(parts[1])
				if err != nil {
					return nil, 0, err
				}
				paramType.FuncReturn = funcReturn
			}
		} else {
			// Array subtype - can be nested like a<a<n>>
			subType, _, err := parseParamTypeAt(subSig, 0)
			if err != nil {
				return nil, 0, err
			}
			paramType.SubType = subType
		}

		i = j
	}

	// Check for optional marker ?
	if i < len(s) && s[i] == '?' {
		paramType.Optional = true
		i++
	}

	return paramType, i - start, nil
}

// parseParamType parses a single parameter type (helper for return type)
func parseParamType(s string) (*ParamType, error) {
	paramType, consumed, err := parseParamTypeAt(s, 0)
	if err != nil {
		return nil, err
	}
	if consumed != len(s) {
		return nil, fmt.Errorf("S0401: Unexpected characters after type")
	}
	return paramType, nil
}

// ValidateArgument validates that a value matches a parameter type
func (pt *ParamType) ValidateArgument(value interface{}) error {
	// Handle union types - value must match at least one type
	if len(pt.UnionTypes) > 0 {
		var lastErr error
		for _, typeCode := range pt.UnionTypes {
			singleType := &ParamType{Type: typeCode}
			if err := singleType.ValidateArgument(value); err == nil {
				return nil // Matches one of the union types
			} else {
				lastErr = err
			}
		}
		// If none matched, return error for first type
		return lastErr
	}

	if value == nil {
		if pt.Type == TypeNull || pt.Type == TypeAny {
			return nil
		}
		// Null is not valid for other types
		return fmt.Errorf("T0410: Expected %s, got null", pt.Type)
	}

	switch pt.Type {
	case TypeAny:
		return nil

	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("T0410: Expected string, got %T", value)
		}

	case TypeNumber:
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("T0410: Expected number, got %T", value)
		}

	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("T0410: Expected boolean, got %T", value)
		}

	case TypeArray:
		arr, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("T0412: Expected array, got %T", value)
		}

		// If there's a subtype, validate each element
		if pt.SubType != nil {
			for i, elem := range arr {
				if err := pt.SubType.ValidateArgument(elem); err != nil {
					return fmt.Errorf("T0412: Array element %d: %v", i, err)
				}
			}
		}

	case TypeObject:
		// Accept both map[string]interface{} and *OrderedObject
		switch value.(type) {
		case map[string]interface{}, *OrderedObject:
			// Valid
		default:
			return fmt.Errorf("T0410: Expected object, got %T", value)
		}

	case TypeFunction:
		// Check for Lambda or FunctionDef
		switch value.(type) {
		case *Lambda:
			// If we have function signature constraints, validate them
			if pt.FuncParams != nil || pt.FuncReturn != nil {
				// TODO: Validate lambda signature matches expected signature
				// For now, just accept any lambda
			}
		case *FunctionDef:
			// Built-in functions are always valid
		default:
			return fmt.Errorf("T0410: Expected function, got %T", value)
		}

	default:
		return fmt.Errorf("S0401: Unknown type code: %s", pt.Type)
	}

	return nil
}
