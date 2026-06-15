package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

func (r GenerateRequest) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *GenerateRequest) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}

func (r ListingKitResult) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *ListingKitResult) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}
