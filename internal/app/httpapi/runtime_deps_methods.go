package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
)

func (d *runtimeDeps) ensureListingKitSupport() *listingKitSupport {
	if d == nil {
		return nil
	}
	if d.features == nil {
		d.features = &featureRuntimeState{}
	}
	if d.features.listingKitSupport == nil {
		d.features.listingKitSupport = &listingKitSupport{}
	}
	return d.features.listingKitSupport
}

func (d *runtimeDeps) addClosers(closers ...func() error) {
	if d == nil {
		return
	}
	if d.shared == nil {
		d.shared = &sharedRuntimeDeps{}
	}
	for _, closer := range closers {
		if closer == nil {
			continue
		}
		d.shared.closers = append(d.shared.closers, closer)
		d.constructionClosers = append(d.constructionClosers, closer)
	}
}

func (d *runtimeDeps) attachAmazonListingModule(module *amazonlistinghttpapi.Module) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
}

func (d *runtimeDeps) attachListingKitModule(module *listingkithttpapi.Module) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
}

func (d *runtimeDeps) attachSDSLoginResult(result *sdsloginbootstrap.BuildResult) {
	if d == nil || result == nil {
		return
	}
	if d.features == nil {
		d.features = &featureRuntimeState{}
	}
	d.features.sdsLoginStatusProvider = result.StatusProvider
}
