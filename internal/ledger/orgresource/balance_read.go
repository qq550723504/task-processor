package orgresource

import (
	"context"
	"errors"
	"time"
)

var ErrBalanceUnavailable = errors.New("organization resource balance unavailable")

// BalanceReader projects only canonical buckets and debts. Authorization belongs
// to the current application boundary; reading never settles owner reservations.
type BalanceReader interface {
	ReadBalances(context.Context, string) (Balances, error)
}

type Balances struct {
	SchemaVersion  string    `json:"schema_version"`
	OrganizationID string    `json:"organization_id"`
	ObservedAt     time.Time `json:"observed_at"`
	Resources      []Balance `json:"resources"`
}

type Balance struct {
	ResourceType ResourceType `json:"resource_type"`
	Unit         string       `json:"unit"`
	State        string       `json:"state"`
	Available    *string      `json:"available"`
	Allocated    *string      `json:"allocated"`
	Reserved     *string      `json:"reserved"`
	Consumed     *string      `json:"consumed"`
	Debt         *string      `json:"debt"`
	UpdatedAt    *time.Time   `json:"updated_at"`
}
