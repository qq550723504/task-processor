package dataservicesapp

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/dataservice"
	datahttp "task-processor/internal/dataservice/httpapi"
	"task-processor/internal/httproute"
	keystore "task-processor/internal/integration/persistence/dataservice"
	jobstore "task-processor/internal/integration/persistence/product/dataacquisition"
	sourcestore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
)

// Runtime owns installation, canonical IAM, charge owner registration and worker start.
// This factory verifies the installed Product schema; it performs no serving DDL.
type RuntimeProvider interface {
	dataacquisition.Provider
	Sites() []dataacquisition.Site
	Ready(context.Context) error
}
type FundingResolver interface {
	Funding(context.Context, collection.Scope) (orgresource.ResourceFunding, error)
}
type Dependencies struct {
	ProductDB         *gorm.DB
	Access            dataservice.Access
	Live              dataacquisition.LiveAccess
	Specialist        dataservice.SpecialistAccess
	Funding           FundingResolver
	Provider          RuntimeProvider
	Starter           dataacquisition.ExecutionStarter
	Charges           func(orgresource.ConsumerChargeOwner) (dataacquisition.Charges, error)
	TrustedProxyCIDRs []string
}
type Module struct {
	keys              *dataservice.CredentialService
	custom            *dataservice.CustomService
	acquisition       *dataacquisition.Service
	repo              *jobstore.Repository
	access            dataservice.Access
	live              dataacquisition.LiveAccess
	funding           FundingResolver
	provider          RuntimeProvider
	results           dataacquisition.CapturedResultReader
	trustedProxyCIDRs []string
}

func NewModule(ctx context.Context, d Dependencies) (*Module, error) {
	if d.ProductDB == nil || d.Access == nil || d.Live == nil || d.Specialist == nil || d.Funding == nil || d.Provider == nil || d.Starter == nil || d.Charges == nil {
		return nil, dataservice.ErrUnavailable
	}
	if err := datahttp.ValidateProxyCIDRs(d.TrustedProxyCIDRs); err != nil {
		return nil, err
	}
	kr, err := keystore.NewCredentialRepository(ctx, d.ProductDB)
	if err != nil {
		return nil, err
	}
	keys, err := dataservice.NewCredentialService(kr, d.Access)
	if err != nil {
		return nil, err
	}
	publisher, err := NewProductPublisher(d.Live)
	if err != nil {
		return nil, err
	}
	repo, err := jobstore.NewRepository(ctx, d.ProductDB, d.Live, publisher, transactionResultReader)
	if err != nil {
		return nil, err
	}
	charges, err := d.Charges(dataacquisition.ChargeOwner{Repository: repo})
	if err != nil {
		return nil, err
	}
	acquisition, err := dataacquisition.NewService(repo, d.Live, d.Provider, charges, d.Starter)
	if err != nil {
		return nil, err
	}
	customPublisher, err := NewCustomProductPublisher()
	if err != nil {
		return nil, err
	}
	cr, err := keystore.NewCustomRepository(ctx, d.ProductDB, d.Specialist, customPublisher)
	if err != nil {
		return nil, err
	}
	custom, err := dataservice.NewCustomService(cr, d.Access, d.Specialist)
	if err != nil {
		return nil, err
	}
	sr, err := sourcestore.NewRepository(d.ProductDB, newCatalogBridge)
	if err != nil {
		return nil, err
	}
	return &Module{keys: keys, custom: custom, acquisition: acquisition, repo: repo, access: d.Access, live: d.Live, funding: d.Funding, provider: d.Provider, results: capturedResultReader{sr}, trustedProxyCIDRs: append([]string(nil), d.TrustedProxyCIDRs...)}, nil
}
func (m *Module) Runner() *dataacquisition.Service { return m.acquisition }
func (m *Module) ChargeOwner() orgresource.ConsumerChargeOwner {
	return dataacquisition.ChargeOwner{Repository: m.repo}
}

const (
	ConsoleBase    = datahttp.ConsoleBase
	SpecialistBase = datahttp.SpecialistBase
	APIBase        = datahttp.APIBase
)

type options = datahttp.Options

func (m *Module) BuildRoutes() []httproute.Descriptor {
	return datahttp.BuildRoutes(datahttp.Dependencies{Keys: m.keys, Custom: m.custom, Acquisition: m.acquisition, Access: m.access, Live: m.live, Funding: m.funding, Provider: m.provider, Results: m.results, Repository: m.repo, Options: m.options, TrustedProxyCIDRs: append([]string(nil), m.trustedProxyCIDRs...)})
}

func (m *Module) options(ctx context.Context) options {
	o := options{Sites: []dataacquisition.Site{}, CustomSites: dataacquisition.Sites(), Fields: dataacquisition.Fields(), PriceFen: dataacquisition.PriceFen, MaximumRows: 200, Formats: []string{"csv", "json", "xlsx"}}
	if err := m.provider.Ready(ctx); err != nil {
		o.UnavailableReason = "实时抓取运行环境未就绪"
		return o
	}
	o.Sites = m.provider.Sites()
	o.AcquisitionReady = len(o.Sites) > 0
	if !o.AcquisitionReady {
		o.UnavailableReason = "尚未开放 Amazon 站点"
	}
	return o
}
