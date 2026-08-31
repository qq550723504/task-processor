package zitadelprotojson

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

// Uint64 decodes the canonical quoted decimal ProtoJSON representation while
// retaining compatibility with integer-valued JSON emitted by intermediaries.
type Uint64 uint64

func (value *Uint64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return errors.New("empty ProtoJSON uint64")
	}

	decimal := string(data)
	if data[0] == '"' {
		if err := json.Unmarshal(data, &decimal); err != nil {
			return err
		}
	}
	if !isUnsignedDecimal(decimal) {
		return errors.New("invalid ProtoJSON uint64")
	}
	parsed, err := strconv.ParseUint(decimal, 10, 64)
	if err != nil {
		return err
	}
	*value = Uint64(parsed)
	return nil
}

func isUnsignedDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range []byte(value) {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
