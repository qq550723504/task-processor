package httpapi

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
	}
}

func (d *runtimeDeps) attachProductModule(module *productModuleResult) {
	if d == nil || module == nil {
		return
	}
	if d.features == nil {
		d.features = &featureRuntimeState{}
	}
	d.addClosers(module.Closers...)
	d.features.productService = module.Service
}

func (d *runtimeDeps) attachImageModule(module *imageModuleResult) {
	if d == nil || module == nil {
		return
	}
	if d.features == nil {
		d.features = &featureRuntimeState{}
	}
	d.addClosers(module.Closers...)
	d.features.imageService = module.Service
	d.features.imageSubjectExtractor = module.SubjectExtractor
	d.features.imageWhiteBgRenderer = module.WhiteBackgroundRender
	d.features.imageSceneRenderer = module.SceneRenderer
}

func (d *runtimeDeps) attachAmazonListingModule(module *amazonListingModuleResult) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
}

func (d *runtimeDeps) attachListingKitModule(module *listingKitModuleResult) {
	if d == nil || module == nil {
		return
	}
	d.addClosers(module.Closers...)
}

func (d *runtimeDeps) attachSDSLoginResult(result *sdsLoginModuleResult) {
	if d == nil || result == nil {
		return
	}
	if d.features == nil {
		d.features = &featureRuntimeState{}
	}
	d.features.sdsLoginStatusProvider = result.StatusProvider
}
