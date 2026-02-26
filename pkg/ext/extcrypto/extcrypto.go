// Package extcrypto provides cryptographic and hashing functions for GoSonata.
// All functions use only the Go standard library (no external dependencies).
//
// Security note: MD5 and SHA-1 are provided for compatibility/fingerprinting only
// and should NOT be used for cryptographic security purposes.
package extcrypto

import (
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // intentional: provided for non-security fingerprinting
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // intentional
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"github.com/sandrolain/gosonata/pkg/functions"
)

// All returns all extended cryptographic function definitions.
func All() []functions.CustomFunctionDef {
	return []functions.CustomFunctionDef{
		UUID(),
		Hash(),
		HMAC(),
	}
}

// AllEntries returns all crypto function definitions as [functions.FunctionEntry],
// suitable for spreading into [gosonata.WithFunctions].
func AllEntries() []functions.FunctionEntry {
	all := All()
	out := make([]functions.FunctionEntry, len(all))
	for i, f := range all {
		out[i] = f
	}
	return out
}

// UUID returns the definition for $uuid().
// Generates a random UUID v4 string.
func UUID() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "uuid",
		Signature: "<:s>",
		Fn: func(_ context.Context, _ ...interface{}) (interface{}, error) {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				return nil, fmt.Errorf("$uuid: failed to generate random bytes: %w", err)
			}
			// Set version 4
			b[6] = (b[6] & 0x0f) | 0x40
			// Set variant bits
			b[8] = (b[8] & 0x3f) | 0x80
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
		},
	}
}

// Hash returns the definition for $hash(str, algorithm).
// Supported algorithms: "md5", "sha1", "sha256", "sha384", "sha512".
// Returns a lowercase hex-encoded digest.
func Hash() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "hash",
		Signature: "<s-s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$hash: first argument must be a string")
			}
			algorithm, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("$hash: second argument (algorithm) must be a string")
			}
			h, err := newHasher(strings.ToLower(algorithm))
			if err != nil {
				return nil, fmt.Errorf("$hash: %w", err)
			}
			h.Write([]byte(str))
			return hex.EncodeToString(h.Sum(nil)), nil
		},
	}
}

// HMAC returns the definition for $hmac(str, key, algorithm).
// Returns a lowercase hex-encoded HMAC.
// Supported algorithms: "md5", "sha1", "sha256", "sha384", "sha512".
func HMAC() functions.CustomFunctionDef {
	return functions.CustomFunctionDef{
		Name:      "hmac",
		Signature: "<s-s-s:s>",
		Fn: func(_ context.Context, args ...interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("$hmac: first argument must be a string")
			}
			key, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("$hmac: second argument (key) must be a string")
			}
			algorithm, ok := args[2].(string)
			if !ok {
				return nil, fmt.Errorf("$hmac: third argument (algorithm) must be a string")
			}
			var mac hash.Hash
			switch strings.ToLower(algorithm) {
			case "md5":
				mac = hmac.New(md5.New, []byte(key)) //nolint:gosec
			case "sha1":
				mac = hmac.New(sha1.New, []byte(key)) //nolint:gosec
			case "sha256":
				mac = hmac.New(sha256.New, []byte(key))
			case "sha384":
				mac = hmac.New(sha512.New384, []byte(key))
			case "sha512":
				mac = hmac.New(sha512.New, []byte(key))
			default:
				return nil, fmt.Errorf("$hmac: unsupported algorithm %q; use md5, sha1, sha256, sha384, or sha512", algorithm)
			}
			mac.Write([]byte(str))
			return hex.EncodeToString(mac.Sum(nil)), nil
		},
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func newHasher(algorithm string) (hash.Hash, error) {
	switch algorithm {
	case "md5":
		return md5.New(), nil //nolint:gosec
	case "sha1":
		return sha1.New(), nil //nolint:gosec
	case "sha256":
		return sha256.New(), nil
	case "sha384":
		return sha512.New384(), nil
	case "sha512":
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unsupported algorithm %q; use md5, sha1, sha256, sha384, or sha512", algorithm)
	}
}
