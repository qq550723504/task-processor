package servicepayments

import (
	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/paymentsecurity"
)

func NewPayloadProtection(key []byte) (billing.ServicePayloadProtection, error) {
	return paymentsecurity.NewPayloadProtection(key, "ecoservices-payment:v1:")
}
func NewMerchantPayloadProtection(key []byte) (e.MerchantProtection, error) {
	return paymentsecurity.NewPayloadProtection(key, "ecoservices-merchant:v1:")
}
