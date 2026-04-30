package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

func marshalStudioSessionJSON(value any) (driver.Value, error) {
	if value == nil {
		return "[]", nil
	}
	return json.Marshal(value)
}

func unmarshalStudioSessionJSON(value any, target any) error {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		if len(typed) == 0 {
			return nil
		}
		return json.Unmarshal(typed, target)
	case string:
		if typed == "" {
			return nil
		}
		return json.Unmarshal([]byte(typed), target)
	default:
		return fmt.Errorf("unsupported studio session json type: %T", value)
	}
}
