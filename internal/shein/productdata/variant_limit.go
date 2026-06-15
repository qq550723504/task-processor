package productdata

import (
	"fmt"

	shein "task-processor/internal/shein"
)

const maxVariantASINs = 1000

func validateVariantASINCount(variantAsins []string) error {
	if len(variantAsins) > maxVariantASINs {
		return shein.NewNonRetryableError(
			fmt.Sprintf("too many variant ASINs: %d > %d", len(variantAsins), maxVariantASINs),
			nil,
		)
	}
	return nil
}
