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

	var parsed uint64
	if data[0] == '"' {
		var decimal string
		if err := json.Unmarshal(data, &decimal); err != nil {
			return err
		}
		converted, err := strconv.ParseUint(decimal, 10, 64)
		if err != nil {
			return err
		}
		parsed = converted
	} else if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*value = Uint64(parsed)
	return nil
}
