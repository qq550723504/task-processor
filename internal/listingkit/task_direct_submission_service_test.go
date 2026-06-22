package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestTaskDirectSubmissionServiceSubmitSheinTaskDirectStopsOnReadinessFailure(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct = nil
	var normalizeCalled bool
	var failCalled bool
	direct := newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage: func(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
			normalizeCalled = true
		},
		failSheinDirectSubmit: func(_ context.Context, _ string, _ *Task, _ *SheinPackage, _ string, submitErr error) error {
			failCalled = true
			return submitErr
		},
		buildSheinSubmitProductAPI: func(context.Context, *Task) (sheinproduct.ProductAPI, error) {
			return nil, errors.New("should not reach product api build when readiness blocks")
		},
		prepareSheinSubmitProduct: func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error) {
			return nil, errors.New("should not prepare product when readiness blocks")
		},
	})

	_, err := direct.submitSheinTaskDirect(context.Background(), "task-1", task, &SubmitTaskRequest{Platform: "shein", Action: "publish"}, sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-1",
		startedAt: time.Now(),
	})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submitSheinTaskDirect() err = %v, want ErrSubmitBlocked", err)
	}
	if !normalizeCalled {
		t.Fatal("expected normalizeSheinSubmitPackage to be called")
	}
	if !failCalled {
		t.Fatal("expected failSheinDirectSubmit to be called")
	}
}

func TestTaskDirectSubmissionServiceSubmitSheinTaskDirectStopsWhenMultipleSKUsLackSaleAttributes(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID = 0
	task.Result.Shein.SaleAttributeResolution.SKUAttributes = nil
	task.Result.Shein.SaleAttributeResolution.SecondarySourceDimension = "尺码"
	task.Result.Shein.SaleAttributeResolution.TemplateOptions = []sheinpub.SaleAttributeTemplateOption{
		{
			AttributeID: 27,
			Name:        "Color",
			NameEn:      "Color",
			SKCScope:    true,
			Important:   true,
		},
		{
			AttributeID: 87,
			Name:        "Size",
			NameEn:      "Size",
		},
	}
	task.Result.Shein.SaleAttributeResolution.Candidates = []sheinpub.SaleAttributeCandidateInfo{
		{
			SourceDimension: "颜色",
			Name:            "Color",
			AttributeID:     27,
			SKCScope:        true,
			SelectedScope:   "primary",
		},
		{
			SourceDimension: "尺码",
			Name:            "Size",
			AttributeID:     87,
			SelectedScope:   "secondary",
		},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList = append(
		task.Result.Shein.RequestDraft.SKCList[0].SKUList,
		SheinSKUDraft{
			SupplierSKU: "SKU-2",
			Currency:    "USD",
			CostPrice:   "12.00",
			BasePrice:   "21.99",
			StockCount:  18,
		},
	)
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS = append(
		task.Result.Shein.PreviewProduct.SKCList[0].SKUS,
		sheinproduct.SKU{
			SupplierSKU: "SKU-2",
			CostInfo: &sheinproduct.CostInfo{
				CostPrice: "12.00",
				Currency:  "USD",
			},
			PriceInfoList: []sheinproduct.PriceInfo{{
				SubSite:   "US",
				BasePrice: 21.99,
				Currency:  "USD",
			}},
			StockInfoList: []sheinproduct.StockInfo{{
				MerchantWarehouseCode: "US",
				InventoryNum:          18,
			}},
		},
	)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil
	task.Result.Shein.SkcList[0].SKUs = append(
		task.Result.Shein.SkcList[0].SKUs,
		PlatformVariant{
			SKU: "SKU-2",
			Attributes: map[string]string{
				"颜色": "Black",
				"尺码": "40",
			},
		},
	)

	var normalizeCalled bool
	var failCalled bool
	direct := newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage: func(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
			normalizeCalled = true
		},
		failSheinDirectSubmit: func(_ context.Context, _ string, _ *Task, _ *SheinPackage, _ string, submitErr error) error {
			failCalled = true
			return submitErr
		},
		buildSheinSubmitProductAPI: func(context.Context, *Task) (sheinproduct.ProductAPI, error) {
			return nil, errors.New("should not reach product api build when readiness blocks")
		},
		prepareSheinSubmitProduct: func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error) {
			return nil, errors.New("should not prepare product when readiness blocks")
		},
	})

	_, err := direct.submitSheinTaskDirect(context.Background(), "task-1", task, &SubmitTaskRequest{Platform: "shein", Action: "publish"}, sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-multi-sku",
		startedAt: time.Now(),
	})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submitSheinTaskDirect() err = %v, want ErrSubmitBlocked", err)
	}
	if !normalizeCalled {
		t.Fatal("expected normalizeSheinSubmitPackage to be called")
	}
	if !failCalled {
		t.Fatal("expected failSheinDirectSubmit to be called")
	}
}

func TestTaskDirectSubmissionServiceSubmitSheinTaskDirectAllowsPrimaryOnlyMultiSKUWhenSecondaryIsOptional(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	primaryValueID := 739
	task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID = 1001184
	task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID = 0
	task.Result.Shein.SaleAttributeResolution.PrimarySourceDimension = "Color"
	task.Result.Shein.SaleAttributeResolution.SecondarySourceDimension = "Size"
	task.Result.Shein.SaleAttributeResolution.SKUAttributes = nil
	task.Result.Shein.SaleAttributeResolution.TemplateOptions = []sheinpub.SaleAttributeTemplateOption{
		{
			AttributeID: 1001184,
			Name:        "Style Type",
			NameEn:      "Style Type",
			Important:   true,
		},
		{
			AttributeID: 27,
			Name:        "Color",
			NameEn:      "Color",
		},
	}
	task.Result.Shein.SaleAttributeResolution.Candidates = []sheinpub.SaleAttributeCandidateInfo{
		{
			SourceDimension: "Color",
			Name:            "Style Type",
			AttributeID:     1001184,
			SelectedScope:   "primary",
		},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute = &SheinResolvedSaleAttribute{
		Scope:            "skc",
		Name:             "Style Type",
		Value:            "white",
		AttributeID:      1001184,
		AttributeValueID: &primaryValueID,
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList = append(
		task.Result.Shein.RequestDraft.SKCList[0].SKUList,
		SheinSKUDraft{
			SupplierSKU: "SKU-2",
			Currency:    "USD",
			CostPrice:   "12.00",
			BasePrice:   "21.99",
			StockCount:  18,
		},
	)
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute = sheinproduct.SaleAttribute{
		AttributeID:      1001184,
		AttributeValueID: 11,
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS = append(
		task.Result.Shein.PreviewProduct.SKCList[0].SKUS,
		sheinproduct.SKU{
			SupplierSKU: "SKU-2",
			CostInfo: &sheinproduct.CostInfo{
				CostPrice: "12.00",
				Currency:  "USD",
			},
			PriceInfoList: []sheinproduct.PriceInfo{{
				SubSite:   "US",
				BasePrice: 21.99,
				Currency:  "USD",
			}},
			StockInfoList: []sheinproduct.StockInfo{{
				MerchantWarehouseCode: "US",
				InventoryNum:          18,
			}},
		},
	)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil
	task.Result.Shein.SkcList[0].SKUs = append(
		task.Result.Shein.SkcList[0].SKUs,
		PlatformVariant{
			SKU: "SKU-2",
			Attributes: map[string]string{
				"Color": "white",
				"Size":  "35×50cm",
			},
		},
	)

	expectedErr := errors.New("reached-api-build")
	var normalizeCalled bool
	var failCalled bool
	direct := newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage: func(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
			normalizeCalled = true
		},
		failSheinDirectSubmit: func(_ context.Context, _ string, _ *Task, _ *SheinPackage, _ string, submitErr error) error {
			failCalled = true
			return submitErr
		},
		buildSheinSubmitProductAPI: func(context.Context, *Task) (sheinproduct.ProductAPI, error) {
			return nil, expectedErr
		},
		prepareSheinSubmitProduct: func(context.Context, *Task, *SheinPackage, string) (*sheinproduct.Product, error) {
			return nil, errors.New("should not prepare product before api builder in this test")
		},
	})

	_, err := direct.submitSheinTaskDirect(context.Background(), "task-1", task, &SubmitTaskRequest{Platform: "shein", Action: "publish"}, sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-primary-only",
		startedAt: time.Now(),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("submitSheinTaskDirect() err = %v, want %v", err, expectedErr)
	}
	if !normalizeCalled {
		t.Fatal("expected normalizeSheinSubmitPackage to be called")
	}
	if !failCalled {
		t.Fatal("expected failSheinDirectSubmit to be called for downstream api error")
	}
}

func TestTaskDirectSubmissionServiceSubmitSheinTaskDirectCompletesExecutionFlow(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	now := time.Now()
	opts := sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-success",
		startedAt: now,
	}
	productAPI := &stubSheinProductAPI{}
	submitProduct := &sheinproduct.Product{
		SupplierCode:          "SKU-1",
		CategoryID:            100,
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Ready Product"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Ready product description"}},
		SKCList: []sheinproduct.SKC{{
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: "SKU-1",
				CostInfo: &sheinproduct.CostInfo{
					CostPrice: "10.00",
					Currency:  "USD",
				},
				PriceInfoList: []sheinproduct.PriceInfo{{
					SubSite:   "US",
					BasePrice: 19.99,
					Currency:  "USD",
				}},
				StockInfoList: []sheinproduct.StockInfo{{
					MerchantWarehouseCode: "DEFAULT",
					InventoryNum:          10,
				}},
			}},
		}},
	}
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	var calls []string
	var phases []string

	direct := newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage: func(gotTask *Task, gotPkg *SheinPackage, gotReq *SubmitTaskRequest, gotAction string) {
			calls = append(calls, "normalize")
			if gotTask != task {
				t.Fatalf("normalize task = %+v, want original task", gotTask)
			}
			if gotPkg != task.Result.Shein {
				t.Fatalf("normalize pkg = %+v, want task shein package", gotPkg)
			}
			if gotReq == nil || gotReq.Action != "publish" {
				t.Fatalf("normalize req = %+v, want publish request", gotReq)
			}
			if gotAction != "publish" {
				t.Fatalf("normalize action = %q, want publish", gotAction)
			}
		},
		buildSheinSubmitProductAPI: func(_ context.Context, gotTask *Task) (sheinproduct.ProductAPI, error) {
			calls = append(calls, "build_api")
			if gotTask != task {
				t.Fatalf("build api task = %+v, want original task", gotTask)
			}
			return productAPI, nil
		},
		persistSheinDirectSubmitPhase: func(_ context.Context, gotTaskID string, gotTask *Task, gotPkg *SheinPackage, gotOpts sheinDirectSubmitOptions, phase string) error {
			calls = append(calls, "persist_phase")
			phases = append(phases, phase)
			if gotTaskID != task.ID {
				t.Fatalf("persist taskID = %q, want %q", gotTaskID, task.ID)
			}
			if gotTask != task {
				t.Fatalf("persist task = %+v, want original task", gotTask)
			}
			if gotPkg != task.Result.Shein {
				t.Fatalf("persist pkg = %+v, want task shein package", gotPkg)
			}
			if gotOpts.action != opts.action || gotOpts.requestID != opts.requestID {
				t.Fatalf("persist opts = %+v, want %+v", gotOpts, opts)
			}
			return nil
		},
		prepareSheinSubmitProduct: func(_ context.Context, gotTask *Task, gotPkg *SheinPackage, gotAction string) (*sheinproduct.Product, error) {
			calls = append(calls, "prepare_product")
			if gotTask != task {
				t.Fatalf("prepare task = %+v, want original task", gotTask)
			}
			if gotPkg != task.Result.Shein {
				t.Fatalf("prepare pkg = %+v, want task shein package", gotPkg)
			}
			if gotAction != opts.action {
				t.Fatalf("prepare action = %q, want %q", gotAction, opts.action)
			}
			return submitProduct, nil
		},
		uploadSheinSubmitImages: func(context.Context, *Task, *SheinPackage, *sheinproduct.Product) error {
			return nil
		},
		resolveSubmitSettings: func(context.Context, *Task) SheinSettings {
			return SheinSettings{}
		},
		executeSheinSubmitRemote: func(gotAPI sheinproduct.ProductAPI, gotAction string, gotProduct *sheinproduct.Product) (*sheinpub.SubmissionResponse, error) {
			calls = append(calls, "complete_remote")
			if gotAPI != productAPI {
				t.Fatalf("complete api = %+v, want %+v", gotAPI, productAPI)
			}
			if gotAction != opts.action {
				t.Fatalf("complete action = %q, want %q", gotAction, opts.action)
			}
			if gotProduct != submitProduct {
				t.Fatalf("complete product = %+v, want %+v", gotProduct, submitProduct)
			}
			return &sheinpub.SubmissionResponse{Code: "0", Success: true, SPUName: "SPU-123"}, nil
		},
		retrySheinSensitiveWordSubmit: func(_ context.Context, gotTaskID string, gotPkg *SheinPackage, action string, requestID string, gotAPI sheinproduct.ProductAPI, gotProduct *sheinproduct.Product, response *sheinpub.SubmissionResponse, responseErr error) (*sheinpub.SubmissionResponse, error, bool) {
			if gotTaskID != task.ID || gotPkg != task.Result.Shein || action != opts.action || requestID != opts.requestID || gotAPI != productAPI || gotProduct != submitProduct {
				t.Fatalf("retry args mismatch")
			}
			return response, responseErr, false
		},
		persistSuccessfulDirectResponse: func(_ context.Context, gotTaskID string, gotTask *Task, gotPkg *SheinPackage, gotOpts sheinDirectSubmitOptions, supplierCode string, response *sheinpub.SubmissionResponse) error {
			if gotTaskID != task.ID || gotTask != task || gotPkg != task.Result.Shein {
				t.Fatalf("persist success args mismatch")
			}
			if gotOpts.action != opts.action || gotOpts.requestID != opts.requestID {
				t.Fatalf("persist success opts = %+v, want %+v", gotOpts, opts)
			}
			if supplierCode != "SKU-1" {
				t.Fatalf("supplierCode = %q, want SKU-1", supplierCode)
			}
			if response == nil || !response.Success {
				t.Fatalf("response = %+v, want success", response)
			}
			return nil
		},
		finishSheinDirectSubmitAttempt: func(_ context.Context, gotTaskID string, gotTask *Task, gotPkg *SheinPackage, gotOpts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {
			if gotTaskID != task.ID || gotTask != task || gotPkg != task.Result.Shein {
				t.Fatalf("finish args mismatch")
			}
			if gotOpts.action != opts.action || gotOpts.requestID != opts.requestID {
				t.Fatalf("finish opts = %+v, want %+v", gotOpts, opts)
			}
			if response == nil || !response.Success || responseErr != nil {
				t.Fatalf("finish response/err = %+v/%v, want success/nil", response, responseErr)
			}
			return nil
		},
		buildTaskPreview: func(_ context.Context, gotTask *Task, platform string) (*ListingKitPreview, error) {
			calls = append(calls, "build_preview")
			if gotTask != task {
				t.Fatalf("preview task = %+v, want original task", gotTask)
			}
			if platform != "shein" {
				t.Fatalf("platform = %q, want shein", platform)
			}
			return expectedPreview, nil
		},
	})

	preview, err := direct.submitSheinTaskDirect(context.Background(), task.ID, task, &SubmitTaskRequest{
		Platform: "shein",
		Action:   "publish",
	}, opts)
	if err != nil {
		t.Fatalf("submitSheinTaskDirect() err = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}

	assertSubmissionPhasesContainOrderedSubsequence(t, calls, []string{
		"normalize",
		"build_api",
		"persist_phase",
		"prepare_product",
		"complete_remote",
		"build_preview",
	})
	assertSubmissionPhasesContainOrderedSubsequence(t, phases, []string{
		sheinpub.SubmissionPhasePrepareProduct,
		sheinpub.SubmissionPhasePreValidate,
		sheinpub.SubmissionPhaseSubmitRemote,
	})
}

func TestTaskDirectSubmissionServiceUsesSubmissionDomainRunner(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	var runnerCalls int
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	direct := newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage: func(*Task, *SheinPackage, *SubmitTaskRequest, string) {},
		runner: submissiondomain.NewDirectSubmitFlowService(submissiondomain.DirectSubmitFlowServiceConfig[*Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, *ListingKitPreview]{
			Phases: submissiondomain.DirectSubmitFlowPhases{
				PrepareProduct: "prepare",
				PreValidate:    "validate",
				SubmitRemote:   "remote",
			},
			BuildProductAPI: func(context.Context, string, *Task, *SheinPackage, submissiondomain.DirectSubmitFlowOptions) (sheinproduct.ProductAPI, error) {
				runnerCalls++
				return &stubSheinProductAPI{}, nil
			},
			PersistPhase: func(context.Context, string, *Task, *SheinPackage, submissiondomain.DirectSubmitFlowOptions, string) error {
				return nil
			},
			PrepareProduct: func(context.Context, string, *Task, *SheinPackage, submissiondomain.DirectSubmitFlowOptions) (*sheinproduct.Product, error) {
				return &sheinproduct.Product{}, nil
			},
			NeedsImageUpload: func(*sheinproduct.Product) bool { return false },
			PreValidate: func(context.Context, string, *Task, *SheinPackage, *sheinproduct.Product, submissiondomain.DirectSubmitFlowOptions) error {
				return nil
			},
			SubmitRemote: func(context.Context, string, *Task, *SheinPackage, sheinproduct.ProductAPI, *sheinproduct.Product, submissiondomain.DirectSubmitFlowOptions) error {
				return nil
			},
			BuildTaskPreview: func(context.Context, *Task, string) (*ListingKitPreview, error) { return expectedPreview, nil },
		}),
	})

	preview, err := direct.submitSheinTaskDirect(context.Background(), task.ID, task, &SubmitTaskRequest{
		Platform: "shein",
		Action:   "publish",
	}, sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-runner",
		startedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("submitSheinTaskDirect() err = %v", err)
	}
	if preview != expectedPreview {
		t.Fatalf("preview = %+v, want %+v", preview, expectedPreview)
	}
	if runnerCalls != 1 {
		t.Fatalf("runner calls = %d, want 1", runnerCalls)
	}
}
