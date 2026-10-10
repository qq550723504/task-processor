package reportcenterapp

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	store "task-processor/internal/integration/persistence/reportcenter"
	"task-processor/internal/listing/record"
	rc "task-processor/internal/reportcenter"
	reporthttp "task-processor/internal/reportcenter/httpapi"
)

type Dependencies struct {
	DB       *gorm.DB
	Resolver OrganizationResolver
	Policy   authz.StaticAuthorizer
	Reviews  ReviewReader
	Records  record.Reader
}

// New supplies descriptors to the current application owner, never a second server.
// Source readers are optional; saved reports remain usable when sources are offline.
func New(ctx context.Context, d Dependencies) (*reporthttp.Handler, error) {
	if d.Resolver == nil || d.Policy == nil || store.VerifySchema(ctx, d.DB) != nil {
		return nil, rc.ErrUnavailable
	}
	repository, e := store.New(d.DB)
	if e != nil {
		return nil, e
	}
	a := &authorization{d.Resolver, d.Policy}
	return &reporthttp.Handler{Service: &rc.Service{Repository: repository, Sources: &Sources{Reviews: d.Reviews, Records: d.Records, Policy: d.Policy}, Authorize: a.Authorize}, Bind: a.Bind}, nil
}
