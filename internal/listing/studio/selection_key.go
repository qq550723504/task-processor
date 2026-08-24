package studio

import (
	"fmt"
	"strings"
)

// SelectionKeyInput contains the platform-neutral fields that identify a
// studio batch selection. Callers keep their transport or persistence models
// outside this package and adapt them to this value object.
type SelectionKeyInput struct {
	ProductID          int64
	ParentProductID    int64
	VariantID          int64
	PrototypeGroupID   int64
	LayerID            string
	PrintableWidth     int
	PrintableHeight    int
	SelectedVariantIDs []int64
}

// BuildSelectionKey returns the stable identity key for a studio batch
// selection. The field order is part of the persisted compatibility contract.
func BuildSelectionKey(input SelectionKeyInput) string {
	variantIDs := make([]string, 0, len(input.SelectedVariantIDs))
	for _, id := range input.SelectedVariantIDs {
		variantIDs = append(variantIDs, fmt.Sprintf("%d", id))
	}
	return fmt.Sprintf(
		"%d:%d:%d:%d:%s:%d:%d:%s",
		input.ProductID,
		input.ParentProductID,
		input.VariantID,
		input.PrototypeGroupID,
		strings.TrimSpace(input.LayerID),
		input.PrintableWidth,
		input.PrintableHeight,
		strings.Join(variantIDs, ","),
	)
}
