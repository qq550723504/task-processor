package listingkit

import (
	"context"
	"errors"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestTaskSubmissionExecutionServiceBuildSheinSubmitProductAPIUsesResolvedStoreID(t *testing.T) {
	t.Parallel()

	var lastStoreID int64
	var builderCtx context.Context
	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api:         &stubSheinProductAPI{},
			lastStoreID: &lastStoreID,
			lastCtx:     &builderCtx,
		},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 903, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{SheinStoreID: 903},
	}

	api, err := exec.buildSheinSubmitProductAPI(context.Background(), task)
	if err != nil {
		t.Fatalf("buildSheinSubmitProductAPI() error = %v", err)
	}
	if api == nil {
		t.Fatal("expected product api")
	}
	if lastStoreID != 903 {
		t.Fatalf("builder store id = %d, want 903", lastStoreID)
	}
	identity := openaiclient.IdentityFromContext(builderCtx)
	if identity.TenantID != task.TenantID {
		t.Fatalf("builder context tenant id = %q, want %q", identity.TenantID, task.TenantID)
	}
	if identity.UserID != task.UserID {
		t.Fatalf("builder context user id = %q, want %q", identity.UserID, task.UserID)
	}
}

func TestTaskSubmissionExecutionServiceBuildSheinSubmitProductAPIRequiresBuilder(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{})

	api, err := exec.buildSheinSubmitProductAPI(context.Background(), &Task{})
	if err == nil {
		t.Fatal("err = nil, want configuration error")
	}
	if api != nil {
		t.Fatalf("api = %+v, want nil", api)
	}
	if err.Error() != "shein product api builder is not configured" {
		t.Fatalf("error = %q, want builder configuration error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceBuildSheinSubmitProductAPIRejectsMissingStoreID(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder: stubSheinProductAPIBuilder{api: &stubSheinProductAPI{}},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 0, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{},
	}

	api, err := exec.buildSheinSubmitProductAPI(context.Background(), task)
	if err == nil {
		t.Fatal("err = nil, want missing store id error")
	}
	if api != nil {
		t.Fatalf("api = %+v, want nil", api)
	}
	if err.Error() != "shein store id is unavailable for submit" {
		t.Fatalf("error = %q, want missing store id error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceBuildSheinSubmitProductAPIRejectsBuilderFallback(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder: stubSheinProductAPIBuilder{msg: "login required"},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 903, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{SheinStoreID: 903},
	}

	api, err := exec.buildSheinSubmitProductAPI(context.Background(), task)
	if err == nil {
		t.Fatal("err = nil, want builder fallback error")
	}
	if api != nil {
		t.Fatalf("api = %+v, want nil", api)
	}
	if err.Error() != "shein submit unavailable: login required" {
		t.Fatalf("error = %q, want builder fallback error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceBuildSheinSubmitProductAPIReturnsStoreResolutionError(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("store resolver failed")
	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder: stubSheinProductAPIBuilder{api: &stubSheinProductAPI{}},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 0, resolveErr
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{},
	}

	api, err := exec.buildSheinSubmitProductAPI(context.Background(), task)
	if err == nil {
		t.Fatal("err = nil, want missing store id error")
	}
	if api != nil {
		t.Fatalf("api = %+v, want nil", api)
	}
	if err.Error() != "shein store id is unavailable for submit" {
		t.Fatalf("error = %q, want missing store id error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceBuildSheinSubmitTranslateAPISkipsBuilderWhenNotNeeded(t *testing.T) {
	t.Parallel()

	var lastStoreID int64
	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinTranslateAPIBuilder: stubSheinTranslateAPIBuilder{
			api:         &stubSheinTranslateAPI{},
			lastStoreID: &lastStoreID,
		},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 903, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{Country: "US"},
	}
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{
			{Language: "en", Name: "English Title"},
			{Language: "es", Name: "Spanish Title"},
		},
		MultiLanguageDescList: []sheinproduct.LanguageContent{
			{Language: "en", Name: "English Description"},
			{Language: "es", Name: "Spanish Description"},
		},
	}

	translateAPI := exec.buildSheinSubmitTranslateAPI(context.Background(), task, product)
	if translateAPI != nil {
		t.Fatalf("translateAPI = %+v, want nil", translateAPI)
	}
	if lastStoreID != 0 {
		t.Fatalf("builder store id = %d, want 0 because builder should not be called", lastStoreID)
	}
}

func TestTaskSubmissionExecutionServiceBuildSheinSubmitTranslateAPIUsesResolvedStoreIDWhenNeeded(t *testing.T) {
	t.Parallel()

	var lastStoreID int64
	expectedAPI := &stubSheinTranslateAPI{}
	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinTranslateAPIBuilder: stubSheinTranslateAPIBuilder{
			api:         expectedAPI,
			lastStoreID: &lastStoreID,
		},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 903, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{Country: "US"},
	}
	product := &sheinproduct.Product{}

	translateAPI := exec.buildSheinSubmitTranslateAPI(context.Background(), task, product)
	if translateAPI == nil {
		t.Fatal("translateAPI = nil, want assigned api")
	}
	if translateAPI != expectedAPI {
		t.Fatalf("translateAPI = %+v, want %+v", translateAPI, expectedAPI)
	}
	if lastStoreID != 903 {
		t.Fatalf("builder store id = %d, want 903", lastStoreID)
	}
}

func TestTaskSubmissionExecutionServiceUploadSheinSubmitImagesRequiresBuilder(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{})

	err := exec.uploadSheinSubmitImages(context.Background(), &Task{}, &SheinPackage{}, &sheinproduct.Product{})
	if err == nil {
		t.Fatal("err = nil, want configuration error")
	}
	if err.Error() != "shein image upload api builder is not configured" {
		t.Fatalf("error = %q, want builder configuration error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceUploadSheinSubmitImagesRejectsMissingStoreID(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 0, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{},
	}

	err := exec.uploadSheinSubmitImages(context.Background(), task, &SheinPackage{}, &sheinproduct.Product{})
	if err == nil {
		t.Fatal("err = nil, want missing store id error")
	}
	if err.Error() != "shein store id is unavailable for image upload" {
		t.Fatalf("error = %q, want missing store id error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceUploadSheinSubmitImagesRejectsBuilderFallback(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinImageAPIBuilder: stubSheinImageAPIBuilder{msg: "login required"},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 903, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{SheinStoreID: 903},
	}

	err := exec.uploadSheinSubmitImages(context.Background(), task, &SheinPackage{}, &sheinproduct.Product{})
	if err == nil {
		t.Fatal("err = nil, want builder fallback error")
	}
	if err.Error() != "shein image upload unavailable: login required" {
		t.Fatalf("error = %q, want builder fallback error", err.Error())
	}
}

func TestTaskSubmissionExecutionServiceNormalizeSheinSubmitPackageMarksConfirmedFinalDraft(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		currentSheinPricingRule: func() sheinpub.PricingRule { return sheinpub.PricingRule{} },
	})
	task := makeReadySheinTask()
	pkg := task.Result.Shein

	exec.normalizeSheinSubmitPackage(task, pkg, &SubmitTaskRequest{
		ConfirmedFinal: true,
	}, "publish")

	if pkg.FinalSubmissionDraft == nil {
		t.Fatal("FinalSubmissionDraft = nil, want initialized")
	}
	if !pkg.FinalSubmissionDraft.Confirmed {
		t.Fatal("Confirmed = false, want true")
	}
	if pkg.FinalSubmissionDraft.ConfirmedAt == nil {
		t.Fatal("ConfirmedAt = nil, want timestamp")
	}
	if pkg.FinalSubmissionDraft.UpdatedAt == nil {
		t.Fatal("UpdatedAt = nil, want timestamp")
	}
	if pkg.FinalSubmissionDraft.SubmitMode != "publish" {
		t.Fatalf("SubmitMode = %q, want publish", pkg.FinalSubmissionDraft.SubmitMode)
	}
}

func TestTaskSubmissionExecutionServiceNormalizeSheinSubmitPackageRebuildsPricingFromFinalDraftOverrides(t *testing.T) {
	t.Parallel()

	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		currentSheinPricingRule: func() sheinpub.PricingRule {
			return sheinpub.PricingRule{
				SourceCurrency: "CNY",
				TargetCurrency: "USD",
			}
		},
	})
	task := makeReadySheinTask()
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	currentSKU := pkg.RequestDraft.SKCList[0].SKUList[0].SupplierSKU
	pkg.RequestDraft.SKCList[0].SKUList[0].CostPrice = "91.80"
	pkg.RequestDraft.SKCList[0].SKUList[0].BasePrice = "91.80"
	pkg.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "91.80",
		Currency:  "USD",
	}}
	pkg.PreviewPayload.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 91.8,
		Currency:  "USD",
	}}
	pkg.PreviewPayload.SKCList[0].SKUS[0].CostInfo = &sheinproduct.CostInfo{
		CostPrice: "91.80",
		Currency:  "USD",
	}
	pkg.Pricing = &sheinpub.PricingReview{
		Ready: true,
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: currentSKU,
			CostCNY:     91.8,
			FinalPrice:  91.8,
			Currency:    "USD",
		}},
	}
	pkg.FinalSubmissionDraft = &sheinpub.FinalDraft{
		Confirmed:            true,
		ManualPriceOverrides: map[string]float64{currentSKU: 99.99},
	}

	exec.normalizeSheinSubmitPackage(task, pkg, &SubmitTaskRequest{
		ConfirmedFinal: true,
	}, "publish")

	if got := pkg.Pricing.SKUPrices[0].FinalPrice; got != 99.99 {
		t.Fatalf("pricing final price = %v, want 99.99", got)
	}
	if got := pkg.RequestDraft.SKCList[0].SKUList[0].BasePrice; got != "99.99" {
		t.Fatalf("draft base price = %q, want 99.99", got)
	}
	if got := pkg.PreviewPayload.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 99.99 {
		t.Fatalf("preview base price = %v, want 99.99", got)
	}
}

func TestTaskSubmissionExecutionServiceExecuteSheinSubmitRemoteRoutesByAction(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{SupplierCode: "SKU-1"}
	cases := []struct {
		name       string
		action     string
		api        stubSheinProductAPI
		wantCalled string
		wantCode   string
		wantMsg    string
	}{
		{
			name:   "publish",
			action: "publish",
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "publish ok"},
			},
			wantCalled: "publish",
			wantCode:   "0",
			wantMsg:    "publish ok",
		},
		{
			name:   "save draft",
			action: "save_draft",
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "draft ok"},
			},
			wantCalled: "save_draft",
			wantCode:   "0",
			wantMsg:    "draft ok",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := ""
			api := tc.api
			api.publishHook = func(*sheinproduct.Product) { called = "publish" }
			api.saveHook = func(*sheinproduct.Product) { called = "save_draft" }
			exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{})

			response, err := exec.executeSheinSubmitRemote(api, tc.action, product)
			if err != nil {
				t.Fatalf("executeSheinSubmitRemote() error = %v", err)
			}
			if response == nil {
				t.Fatal("executeSheinSubmitRemote() response = nil, want summary")
			}
			if response.Code != tc.wantCode || response.Message != tc.wantMsg {
				t.Fatalf("executeSheinSubmitRemote() response = %+v, want code=%q msg=%q", response, tc.wantCode, tc.wantMsg)
			}
			if called != tc.wantCalled {
				t.Fatalf("called api method = %q, want %q", called, tc.wantCalled)
			}
		})
	}
}
