package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

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
	submitProduct := &sheinproduct.Product{SupplierCode: "SKU-1"}
	expectedPreview := &ListingKitPreview{TaskID: task.ID}
	var calls []string

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
			if phase != sheinpub.SubmissionPhasePrepareProduct {
				t.Fatalf("persist phase = %q, want %q", phase, sheinpub.SubmissionPhasePrepareProduct)
			}
			return nil
		},
		prepareSheinDirectSubmitProduct: func(_ context.Context, gotTaskID string, gotTask *Task, gotPkg *SheinPackage, gotOpts sheinDirectSubmitOptions) (*sheinproduct.Product, error) {
			calls = append(calls, "prepare_product")
			if gotTaskID != task.ID {
				t.Fatalf("prepare taskID = %q, want %q", gotTaskID, task.ID)
			}
			if gotTask != task {
				t.Fatalf("prepare task = %+v, want original task", gotTask)
			}
			if gotPkg != task.Result.Shein {
				t.Fatalf("prepare pkg = %+v, want task shein package", gotPkg)
			}
			if gotOpts.action != opts.action || gotOpts.requestID != opts.requestID {
				t.Fatalf("prepare opts = %+v, want %+v", gotOpts, opts)
			}
			return submitProduct, nil
		},
		completeSheinDirectRemoteSubmit: func(_ context.Context, gotTaskID string, gotTask *Task, gotPkg *SheinPackage, gotAPI sheinproduct.ProductAPI, gotProduct *sheinproduct.Product, gotOpts sheinDirectSubmitOptions) error {
			calls = append(calls, "complete_remote")
			if gotTaskID != task.ID {
				t.Fatalf("complete taskID = %q, want %q", gotTaskID, task.ID)
			}
			if gotTask != task {
				t.Fatalf("complete task = %+v, want original task", gotTask)
			}
			if gotPkg != task.Result.Shein {
				t.Fatalf("complete pkg = %+v, want task shein package", gotPkg)
			}
			if gotAPI != productAPI {
				t.Fatalf("complete api = %+v, want %+v", gotAPI, productAPI)
			}
			if gotProduct != submitProduct {
				t.Fatalf("complete product = %+v, want %+v", gotProduct, submitProduct)
			}
			if gotOpts.action != opts.action || gotOpts.requestID != opts.requestID {
				t.Fatalf("complete opts = %+v, want %+v", gotOpts, opts)
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

	wantCalls := []string{"normalize", "build_api", "persist_phase", "prepare_product", "complete_remote", "build_preview"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %+v, want %+v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls[%d] = %q, want %q; full calls = %+v", i, calls[i], wantCalls[i], calls)
		}
	}
}
