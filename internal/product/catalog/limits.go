package catalog

import "fmt"

// MaxEncodedSnapshotBytes bounds a canonical snapshot before JSON
// materialization. Tool projections may impose a stricter consumer limit.
const MaxEncodedSnapshotBytes = 8 << 20

func ValidateEncodedSnapshotSize(encoded []byte) error {
	if len(encoded) == 0 || len(encoded) > MaxEncodedSnapshotBytes {
		return fmt.Errorf("%w: encoded snapshot exceeds size limit", ErrInvalidSnapshot)
	}
	return nil
}
