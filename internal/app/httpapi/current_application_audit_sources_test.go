package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

func TestCurrentApplicationAdmitsProductExecutionWithImageAuditReader(t *testing.T) {
	source, resource, acquisition, run, image := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
	stop := errors.New("assembly reached the workbench after source admission")
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{}, stop
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, stop },
		buildCommercial:    func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, stop },
		buildResourceCharges: func(context.Context, *gorm.DB, *gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer, storecenter.RuntimeCapabilities) (*orgresource.ConsumerChargeService, error) {
			return &orgresource.ConsumerChargeService{}, nil
		},
	}
	_, err := buildCurrentApplication(context.Background(), source, currentApplicationTestConfig(), logrus.New(), factories,
		WithCommercialOwnerDatabase(resource), WithProductAcquisition(acquisition),
		WithProductAgent(ProductAgentDependencies{RunDB: run}), WithAccountAuditUsageSources(image, nil))
	if !errors.Is(err, stop) {
		t.Fatalf("distinct image reader and Product Agent source rejected before assembly: %v", err)
	}
}

func TestCurrentInvocationAuditSourcesBindEachNamespaceExactlyOnce(t *testing.T) {
	image, product := &gorm.DB{}, &gorm.DB{}
	for _, mode := range []string{"product-execution", "image-execution", "duplicate-product", "duplicate-image", "missing-product", "shared-owner", "empty-readers"} {
		t.Run(mode, func(t *testing.T) {
			options := currentApplicationOptions{accountAuditSources: 1, accountAuditImageDB: image}
			switch mode {
			case "product-execution", "duplicate-product":
				options.productAgent = &ProductAgentDependencies{RunDB: product}
				if mode == "duplicate-product" {
					options.accountAuditProductDB = &gorm.DB{}
				}
			case "image-execution", "duplicate-image":
				options.accountAuditImageDB, options.accountAuditProductDB = nil, product
				options.imageAgentDB = image
				if mode == "duplicate-image" {
					options.accountAuditImageDB = &gorm.DB{}
				}
			case "shared-owner":
				options.productAgent = &ProductAgentDependencies{RunDB: image}
			case "empty-readers":
				options.accountAuditImageDB = nil
			}
			sources, err := currentInvocationAuditSources(options)
			if mode == "product-execution" || mode == "image-execution" {
				if err != nil || len(sources) != 2 || sources["image"] != image || sources["product"] != product {
					t.Fatalf("wrong current owner bindings: %v, %v", sources, err)
				}
			} else if err == nil {
				t.Fatal("ambiguous or incomplete invocation sources admitted")
			}
		})
	}
}
