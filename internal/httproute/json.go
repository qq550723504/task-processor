package httproute

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

var ErrInvalidJSON = errors.New("invalid or ambiguous JSON")

// DecodeJSON extracts duplicate-key and depth validation from the payment
// adapter. Transport consumers may additionally reject unknown DTO fields.
func DecodeJSON(raw []byte, out any, limit int, rejectUnknown bool) error {
	if len(raw) == 0 || len(raw) > limit || !utf8.Valid(raw) {
		return ErrInvalidJSON
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 24 {
			return ErrInvalidJSON
		}
		tok, err := dec.Token()
		if err != nil {
			return ErrInvalidJSON
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				s, ok := key.(string)
				if err != nil || !ok || seen[s] {
					return ErrInvalidJSON
				}
				seen[s] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return ErrInvalidJSON
		}
		_, err = dec.Token()
		return err
	}
	if walk(0) != nil {
		return ErrInvalidJSON
	}
	if _, err := dec.Token(); err != io.EOF {
		return ErrInvalidJSON
	}
	dec = json.NewDecoder(bytes.NewReader(raw))
	if rejectUnknown {
		dec.DisallowUnknownFields()
		var payload any
		if json.Unmarshal(raw, &payload) != nil || !exactDTOKeys(payload, reflect.TypeOf(out)) {
			return ErrInvalidJSON
		}
	}
	if dec.Decode(out) != nil || dec.Decode(new(any)) != io.EOF {
		return ErrInvalidJSON
	}
	return nil
}

// encoding/json accepts case aliases for struct tags. Command transports must
// accept only the exact public contract, including nested onboarding fields.
func exactDTOKeys(value any, t reflect.Type) bool {
	if t == nil {
		return false
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch node := value.(type) {
	case map[string]any:
		if t.Kind() == reflect.Map {
			return true
		}
		if t.Kind() != reflect.Struct {
			return false
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			fields[name] = f.Type
		}
		for key, v := range node {
			field, ok := fields[key]
			if !ok || !exactDTOKeys(v, field) {
				return false
			}
		}
	case []any:
		if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
			return false
		}
		for _, v := range node {
			if !exactDTOKeys(v, t.Elem()) {
				return false
			}
		}
	}
	return true
}
