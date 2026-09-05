package shein

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	strictjson "sigs.k8s.io/json"
)

const MaxPersistedPackageBytes = 2 << 20
const MaxPersistedPackageDepth = 64

var ErrInvalidPersistedPackage = errors.New("invalid bounded SHEIN persisted package")

// DecodePersistedPackageStrict accepts only a persisted JSON copy. A distinct
// defined type bypasses Package.UnmarshalJSON so its permissive decoder cannot
// swallow unknown/duplicate fields. No resolver-private state can be injected.
func DecodePersistedPackageStrict(raw []byte) (*Package, error) {
	if len(raw) == 0 || len(raw) > MaxPersistedPackageBytes || !utf8.Valid(raw) || !json.Valid(raw) || !validPersistedSyntaxBounds(raw) || !validPersistedNumbers(raw) {
		return nil, ErrInvalidPersistedPackage
	}
	if bytes.TrimSpace(raw)[0] != '{' {
		return nil, ErrInvalidPersistedPackage
	}
	type persistedPackage Package
	var decoded persistedPackage
	strictErrors, err := strictjson.UnmarshalStrict(raw, &decoded)
	if err != nil || len(strictErrors) > 0 {
		return nil, ErrInvalidPersistedPackage
	}
	pkg := Package(decoded)
	if conflictingPersistedAliases(&pkg) {
		return nil, ErrInvalidPersistedPackage
	}
	NormalizePackageSemanticFields(&pkg)
	return &pkg, nil
}

// Binary64 rounding belongs to the persisted Go schema; nonzero JSON numbers
// must not silently underflow to zero before they can be content-bound.
func validPersistedNumbers(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		number, ok := token.(json.Number)
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(string(number), 64)
		if err != nil {
			return false
		}
		if value != 0 {
			continue
		}
		mantissa := strings.FieldsFunc(string(number), func(r rune) bool { return r == 'e' || r == 'E' })[0]
		if strings.ContainsAny(mantissa, "123456789") {
			return false
		}
	}
}

func equalPersistedAliases(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && bytes.Equal(left, right)
}

// Syntax is parsed by encoding/json and typed decoding by sigs.k8s.io/json.
// This small preflight supplies only the depth and UTF-16 rejection missing
// from those decoders (which otherwise replace lone surrogates with U+FFFD).
func validPersistedSyntaxBounds(raw []byte) bool {
	depth := 0
	quoted := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if !quoted {
			switch c {
			case '"':
				quoted = true
			case '{', '[':
				depth++
				if depth > MaxPersistedPackageDepth {
					return false
				}
			case '}', ']':
				depth--
			}
			continue
		}
		if c == '"' {
			quoted = false
			continue
		}
		if c != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		if value < 0xd800 || value > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return depth == 0 && !quoted
}
