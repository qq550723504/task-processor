package review

import (
	"encoding/json"
	"strings"
	"time"
)

// ValidateStorage reserves the exact remaining shape of a successful accept
// and Apply. A pending or accepted object must never exhaust storage only when
// its decision or receipt is appended. Oversized edits leave the old state intact.
func (r Record) ValidateStorage() error {
	future := r
	at := time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	actor := strings.Repeat("<", 128) // JSON's maximum escaping expansion per actor byte.
	if future.State == "pending" {
		future.Revision++
		future.History = append(append([]Decision(nil), r.History...), Decision{"accept", actor, future.Revision, r.Title, r.Title, at})
	}
	if future.State == "pending" || future.State == "accepted" {
		future.State = "applied"
		future.Receipt = &Receipt{r.ID, future.Revision, 1<<63 - 1, "review:" + strings.Repeat("f", 64), actor, at}
	}
	for _, value := range []any{r, r.View(), future, future.View()} {
		raw, err := json.Marshal(value)
		if err != nil {
			return ErrInvalid
		}
		if len(raw) > MaxRecordBytes {
			return ErrTooLarge
		}
	}
	return nil
}
