package podapp

import (
	"context"
	"net/http"
	"task-processor/internal/integration/httpimage"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	podstore "task-processor/internal/integration/persistence/product/pod"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/integration/sds"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"

	"gorm.io/gorm"
)

type Dependencies struct {
	ProductDB, AssetDB    *gorm.DB
	Authorization         collection.Authorizer
	OriginalAuthorization collection.ExecutionAuthorizer
	DesignAuthorization   DesignExecutionAuthorizer
	Collections           *collection.Service
	Credentials           sds.CredentialSource
	HTTP                  *http.Client
	OSSHosts              []string
	Starter               Starter
	ReceiveProduct        ProductReceiver
}
type Application struct {
	Service   *Service
	Processor *Processor
}

func NewApplication(ctx context.Context, d Dependencies) (*Application, error) {
	if ctx == nil || d.ProductDB == nil || d.AssetDB == nil || d.Authorization == nil || d.OriginalAuthorization == nil || d.DesignAuthorization == nil || d.Collections == nil || d.Credentials == nil || d.HTTP == nil || d.Starter == nil || d.ReceiveProduct == nil || collectionstore.VerifyReceiverSchema(ctx, d.ProductDB) != nil {
		return nil, pod.ErrUnavailable
	}
	repo, e := podstore.NewRepository(ctx, d.ProductDB)
	if e != nil {
		return nil, e
	}
	attempts, e := submissionstore.NewRepository(d.ProductDB)
	if e != nil {
		return nil, e
	}
	kernel, e := submission.NewExecutionKernel(attempts)
	if e != nil {
		return nil, e
	}
	approvals, e := assetstore.NewBoundedApprovalCommitReader(d.AssetDB, 2<<20)
	if e != nil {
		return nil, e
	}
	assets, e := assetstore.NewRepository(d.AssetDB)
	if e != nil {
		return nil, e
	}
	snapshots, e := catalogstore.NewBoundedSnapshotReader(d.ProductDB, 2<<20)
	if e != nil {
		return nil, e
	}
	applied, e := reviewstore.NewAppliedPublicationReader(d.ProductDB)
	if e != nil {
		return nil, e
	}
	inputs := OriginalInputs{RequestAuthorization: d.Authorization, Collections: d.Collections, Authorization: d.OriginalAuthorization, DesignAuthorization: d.DesignAuthorization, Approvals: approvals, Snapshots: snapshots, Applied: applied, ImageHTTP: httpimage.NewPublicImageHTTPClient()}
	approvalService, e := asset.NewSourceApprovalService(inputs, assets, approvals)
	if e != nil {
		return nil, e
	}
	templates, e := sds.NewTemplateClient(d.HTTP, d.Credentials)
	if e != nil {
		return nil, e
	}
	mutations, e := sds.NewMutationClient(d.HTTP, d.Credentials, d.OSSHosts)
	if e != nil {
		return nil, e
	}
	observer, e := sds.NewReadbackClient(d.HTTP, d.Credentials)
	if e != nil {
		return nil, e
	}
	processor := &Processor{Repository: repo, Inputs: inputs, Templates: templates, Credentials: d.Credentials, Kernel: kernel, Mutations: mutations, Observer: observer}
	service := &Service{Authorization: d.Authorization, Repository: repo, Inputs: inputs, Templates: templates, Credentials: d.Credentials, Approvals: approvalService, Processor: processor, Starter: d.Starter, Intents: kernel, ReceiveProduct: d.ReceiveProduct}
	return &Application{service, processor}, nil
}
