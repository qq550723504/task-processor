package servicepayments

import (
	"task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
)

func NewPayloadProtection(key []byte) (billing.ServicePayloadProtection, error) {
	return paymentsecurity.NewPayloadProtection(key, "ecoservices-payment:v1:")
}
