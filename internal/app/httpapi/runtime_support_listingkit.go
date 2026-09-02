package httpapi

import (
	"context"
	"errors"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog"
	sdsadapter "task-processor/internal/sds/adapter"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsbootstrap "task-processor/internal/sds/httpbootstrap"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sheinlogin"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
)

var newSDSSyncServiceForHTTPAPI = sdsbootstrap.NewSyncService

type listingKitProductSnapshotReader struct {
	reader catalog.SnapshotReader
}

func newListingKitProductSnapshotReader(reader catalog.SnapshotReader) listingkit.ProductSnapshotReader {
	return listingKitProductSnapshotReader{reader: reader}
}

func (r listingKitProductSnapshotReader) GetProductSnapshot(ctx context.Context, query listingkit.ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	published, err := r.reader.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{
		TenantID: query.TenantID, ProductKey: query.ProductKey,
	})
	if errors.Is(err, catalog.ErrSnapshotNotReady) {
		return catalog.ProductSnapshot{}, listingkit.ErrProductSnapshotNotReady
	}
	if err != nil {
		return catalog.ProductSnapshot{}, err
	}
	return catalog.CloneProductSnapshot(published.Snapshot)
}

func isProductSnapshotNotReadyForHTTPAPI(err error) bool {
	return errors.Is(err, listingkit.ErrProductSnapshotNotReady) || errors.Is(err, catalog.ErrSnapshotNotReady)
}

func readProductSnapshotForHTTPAPI(ctx context.Context, deps *runtimeDeps, tenantID, productKey string) (catalog.ProductSnapshot, error) {
	if deps == nil || deps.features == nil || deps.features.productSnapshotReader == nil {
		return catalog.ProductSnapshot{}, listingkit.ErrProductSnapshotNotReady
	}
	return deps.features.productSnapshotReader.GetProductSnapshot(ctx, listingkit.ProductSnapshotQuery{TenantID: tenantID, ProductKey: productKey})
}

func ensureListingKitSheinCookieStore(logger *logrus.Logger, deps *runtimeDeps) *sheinlogin.RedisStore {
	if deps == nil || deps.shared == nil || deps.shared.cfg == nil {
		return nil
	}
	support := deps.ensureListingKitSupport()
	if support == nil {
		return nil
	}
	if support.sheinCookieStore != nil {
		return support.sheinCookieStore
	}
	store, err := sheinloginbootstrap.BuildRedisStore(deps.shared.cfg)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("failed to initialize listingkit shein cookie store; shein runtime will degrade")
		}
		return nil
	}
	if store == nil {
		return nil
	}
	support.sheinCookieStore = store
	deps.addClosers(store.Close)
	return store
}

func buildSDSSyncService(logger *logrus.Logger, deps *runtimeDeps) sdsusecase.Service {
	if deps == nil || deps.shared == nil || deps.features == nil {
		return nil
	}
	approvedAssets := ensureApprovedAssetReader(logger, deps)
	if approvedAssets == nil {
		return nil
	}

	svc, authState, err := newSDSSyncServiceForHTTPAPI(approvedAssets, sdshttpapi.BuildClientConfig(deps.shared.cfg))
	if err != nil {
		logger.WithError(err).Warn("failed to initialize SDS client; SDS sync disabled")
		return nil
	}
	if svc == nil {
		logger.Warn("SDS sync service not initialized; SDS sync disabled")
		return nil
	}

	if authState == nil || strings.TrimSpace(authState.AccessToken) == "" {
		logger.Info("SDS auth state not found at startup; keeping SDS sync enabled for request-time auth bootstrap")
	}

	return svc
}

func ensureApprovedAssetReader(logger *logrus.Logger, deps *runtimeDeps) sdsadapter.ApprovedAssetReader {
	if deps == nil || deps.shared == nil || deps.features == nil {
		return nil
	}
	support := deps.ensureListingKitSupport()
	if support == nil {
		return nil
	}
	if support.approvedAssetReader != nil {
		return support.approvedAssetReader
	}
	reader, closers, err := newApprovedAssetReaderForHTTPAPI(deps.shared.cfg, logger)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("failed to initialize approved product asset reader; SDS sync disabled")
		}
		return nil
	}
	if reader == nil {
		return nil
	}
	support.approvedAssetReader = reader
	deps.addClosers(closers...)
	return support.approvedAssetReader
}

func buildSDSBaselineRemoteProvider(logger *logrus.Logger, deps *runtimeDeps) listingkit.SDSBaselineRemoteProvider {
	if deps == nil || deps.shared == nil {
		return nil
	}
	support := deps.ensureListingKitSupport()
	if support == nil {
		return nil
	}
	if support.sdsBaselineRemoteProvider != nil {
		return support.sdsBaselineRemoteProvider
	}
	provider, err := sdsbootstrap.NewBaselineRemoteProvider(sdshttpapi.BuildClientConfig(deps.shared.cfg))
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("failed to initialize SDS baseline remote provider; online baseline validation disabled")
		}
		return nil
	}
	support.sdsBaselineRemoteProvider = provider
	return support.sdsBaselineRemoteProvider
}
