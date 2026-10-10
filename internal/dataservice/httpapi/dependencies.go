package dataservicehttpapi

import (
	"context"
	"task-processor/internal/dataservice"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
)

// The HTTP adapter consumes owner services/read projections. It owns no schema,
// funds, Product facts or startup lifecycle.
type ReadRepository interface {
	Usage(context.Context, collection.Scope) (dataacquisition.Usage, error)
	KeyQuotas(context.Context, collection.Scope) ([]dataacquisition.KeyQuota, error)
	List(context.Context, collection.Scope, int) ([]dataacquisition.Job, error)
	Cancel(context.Context, collection.Scope, string, string) (dataacquisition.Job, error)
}
type Dependencies struct {
	Keys        *dataservice.CredentialService
	Custom      *dataservice.CustomService
	Acquisition *dataacquisition.Service
	Access      dataservice.Access
	Live        dataacquisition.LiveAccess
	Funding     interface {
		Funding(context.Context, collection.Scope) (orgresource.ResourceFunding, error)
	}
	Provider interface {
		Ready(context.Context) error
		Sites() []dataacquisition.Site
	}
	Results           dataacquisition.CapturedResultReader
	Repository        ReadRepository
	Options           func(context.Context) Options
	TrustedProxyCIDRs []string
}
type handler struct{ Dependencies }
type Options struct {
	Sites             []dataacquisition.Site `json:"sites"`
	CustomSites       []dataacquisition.Site `json:"customSites"`
	Fields            []string               `json:"fields"`
	PriceFen          int64                  `json:"priceFen"`
	MaximumRows       int                    `json:"maximumRows"`
	Formats           []string               `json:"formats"`
	AcquisitionReady  bool                   `json:"acquisitionReady"`
	UnavailableReason string                 `json:"unavailableReason,omitempty"`
}

func ValidateProxyCIDRs(values []string) error { return validateProxyCIDRs(values) }
