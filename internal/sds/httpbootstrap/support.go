package httpbootstrap

import (
	"context"

	"task-processor/internal/productimage"
	sdsclient "task-processor/internal/sds/client"
	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
)

func NewSyncService(imageSvc productimage.Service, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
	if cfg == nil {
		cfg = sdsclient.DefaultConfig()
	}
	sdsHTTPClient, err := sdsclient.New(cfg)
	if err != nil {
		return nil, nil, err
	}

	authState := sdsHTTPClient.AuthState()
	svc, err := sdsusecase.NewService(sdsusecase.Config{
		SDSClient:    sdsHTTPClient,
		ImageService: imageSvc,
	})
	if err != nil {
		return nil, authState, err
	}

	return svc, authState, nil
}

type BaselineRemoteProvider struct {
	design *sdsdesign.Service
}

func NewBaselineRemoteProvider(cfg *sdsclient.Config) (*BaselineRemoteProvider, error) {
	if cfg == nil {
		cfg = sdsclient.DefaultConfig()
	}
	sdsHTTPClient, err := sdsclient.New(cfg)
	if err != nil {
		return nil, err
	}
	return &BaselineRemoteProvider{
		design: sdsdesign.NewService(sdsHTTPClient),
	}, nil
}

func (p *BaselineRemoteProvider) GetProductDetail(ctx context.Context, parentProductID int64) (*sdstemplate.ProductDetail, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetProductDetail(ctx, parentProductID)
}

func (p *BaselineRemoteProvider) GetDesignProduct(ctx context.Context, variantID int64) (*sdsdesign.DesignProductPage, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetDesignProduct(ctx, variantID)
}

func (p *BaselineRemoteProvider) GetDesignProductForPrototypeGroup(ctx context.Context, variantID, prototypeGroupID int64) (*sdsdesign.DesignProductPage, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetDesignProductForPrototypeGroup(ctx, variantID, prototypeGroupID)
}

func (p *BaselineRemoteProvider) GetPrototypeGroups(ctx context.Context, parentProductID int64) ([]sdsdesign.PrototypeGroup, error) {
	if p == nil || p.design == nil {
		return nil, nil
	}
	return p.design.GetPrototypeGroups(ctx, parentProductID)
}
