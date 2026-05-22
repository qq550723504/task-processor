package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestSubmitTaskPublishesSDSRenderedImages(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	rendered := []string{
		"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-1.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-2.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-3.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-4.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-5.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-6.jpg",
	}
	task.Result.SDSSync = &SDSSyncSummary{
		Status:          "completed",
		MockupImageURLs: rendered,
	}
	task.Result.Shein.Images = &PlatformImageSet{
		MainImage:    rendered[0],
		Gallery:      append([]string(nil), rendered[1:]...),
		SourceImages: []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: rendered[0],
		Gallery:   append([]string(nil), rendered[1:]...),
		Source:    []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].ImageInfo = &SheinImageDraft{
		MainImage: rendered[0],
		Gallery:   append([]string(nil), rendered[1:]...),
		Source:    []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].MainImage = rendered[0]
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo(rendered[:1])
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	uploaded := make([]string, 0, len(rendered))
	uploadMap := map[string]string{}
	for index, url := range rendered {
		uploadedURL := fmt.Sprintf("https://img.shein.com/uploaded/rendered-%d.jpg", index)
		uploaded = append(uploaded, uploadedURL)
		uploadMap[url] = uploadedURL
	}
	imageAPI := &stubSheinImageAPI{uploaded: uploadMap}
	var submitted *sheinproduct.Product
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{
						Success: true,
						SPUName: "SPU-123",
						Version: "v1",
					},
				},
			},
		}),
		withTestSheinImageAPIBuilder(stubSheinImageAPIBuilder{api: imageAPI}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	expectedSPUImages := append([]string(nil), uploaded...)
	expectedSPUImages = append(expectedSPUImages, uploaded[0])
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) != len(expectedSPUImages) {
		t.Fatalf("submitted SPU image info = %+v, want normalized uploaded SPU images", submitted.ImageInfo)
	}
	for index, image := range submitted.ImageInfo.ImageInfoList {
		if image.ImageURL != expectedSPUImages[index] {
			t.Fatalf("submitted SPU image %d = %q, want uploaded %q", index, image.ImageURL, expectedSPUImages[index])
		}
		wantType := 2
		if index == 0 {
			wantType = 1
		}
		if index == len(expectedSPUImages)-1 {
			wantType = 5
		}
		if image.ImageType != wantType {
			t.Fatalf("submitted SPU image %d type = %d, want %d", index, image.ImageType, wantType)
		}
	}
	expectedSKCImages := append([]string(nil), uploaded...)
	expectedSKCImages = append(expectedSKCImages, uploaded[0])
	expectedSKCImages = append(expectedSKCImages, uploaded[0])
	if len(submitted.SKCList) != 1 || len(submitted.SKCList[0].ImageInfo.ImageInfoList) != len(expectedSKCImages) {
		t.Fatalf("submitted SKC image info = %+v", submitted.SKCList)
	}
	for index, image := range submitted.SKCList[0].ImageInfo.ImageInfoList {
		if image.ImageURL != expectedSKCImages[index] {
			t.Fatalf("submitted SKC image %d = %q, want uploaded %q", index, image.ImageURL, expectedSKCImages[index])
		}
		wantType := 2
		if index == 0 {
			wantType = 1
		}
		if index == len(expectedSKCImages)-1 {
			wantType = 6
		} else if index == len(expectedSKCImages)-2 {
			wantType = 5
		}
		if image.ImageType != wantType {
			t.Fatalf("submitted SKC image %d type = %d, want %d", index, image.ImageType, wantType)
		}
		if image.ImageURL == sourceImage {
			t.Fatalf("submitted SKC image still uses source image: %q", sourceImage)
		}
	}
	if len(submitted.SKCList[0].SKUS) != 1 || submitted.SKCList[0].SKUS[0].ImageInfo == nil || len(submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList) != 1 {
		t.Fatalf("submitted SKU image info = %+v, want preserved uploaded SKU image", submitted.SKCList[0].SKUS)
	}
	if got := submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList[0].ImageURL; got != uploaded[0] {
		t.Fatalf("submitted SKU image url = %q, want uploaded %q", got, uploaded[0])
	}
	if len(imageAPI.calls) != len(rendered) {
		t.Fatalf("upload calls = %+v, want %d unique uploads", imageAPI.calls, len(rendered))
	}
	for _, url := range rendered {
		if imageAPI.calls[url] != 1 {
			t.Fatalf("upload call count for %q = %d, want 1", url, imageAPI.calls[url])
		}
	}
}

func TestBuildSheinImageUploadPreflightCountsUniqueSDSImages(t *testing.T) {
	t.Parallel()

	rendered := []string{
		"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-1.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-2.jpg",
	}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo(rendered[:1])

	report := buildSheinImageUploadPreflight(task.Result.Shein)
	if report == nil {
		t.Fatal("expected image upload preflight")
	}
	if report.TotalImageReferences != 7 {
		t.Fatalf("total references = %d, want 7", report.TotalImageReferences)
	}
	if report.UniqueImageURLs != len(rendered) {
		t.Fatalf("unique urls = %d, want %d", report.UniqueImageURLs, len(rendered))
	}
	if report.PendingUploadURLs != len(rendered) {
		t.Fatalf("pending upload urls = %d, want %d", report.PendingUploadURLs, len(rendered))
	}
	if !report.UsesSDSMockups || report.SDSMockupURLs != len(rendered) {
		t.Fatalf("sds mockup report = %+v", report)
	}
}

func TestSubmitTaskBlocksPublishWhenSheinImageUploadFails(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	rendered := []string{"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg"}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo(rendered)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo(rendered)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalled := false
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalled = true
				},
			},
		}),
		withTestSheinImageAPIBuilder(stubSheinImageAPIBuilder{
			api: &stubSheinImageAPI{err: errors.New("upload rejected")},
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !strings.Contains(err.Error(), "upload rejected") {
		t.Fatalf("submit err = %v, want upload rejected", err)
	}
	if publishCalled {
		t.Fatal("publish should not be called when image upload fails")
	}
	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("submission was not persisted: %+v", saved.Result)
	}
	if saved.Result.Shein.Submission.LastStatus != "failed" || !strings.Contains(saved.Result.Shein.Submission.LastError, "upload rejected") {
		t.Fatalf("submission failure = %+v", saved.Result.Shein.Submission)
	}
}

func TestSubmitTaskReusesSheinImageUploadCache(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].ImageInfo = sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	imageAPI := &stubSheinImageAPI{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "OK"},
			},
		}),
		withTestSheinImageAPIBuilder(stubSheinImageAPIBuilder{api: imageAPI}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft", ConfirmedFinal: true})
		if err != nil {
			t.Fatalf("submit task %d: %v", i+1, err)
		}
	}
	if imageAPI.calls[sourceImage] != 1 {
		t.Fatalf("upload calls for source image = %d, want 1", imageAPI.calls[sourceImage])
	}
	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	cache := saved.Result.Shein.FinalDraft.SheinImageUploadCache
	if cache[sourceImage] == "" || !isSheinUploadedImageURL(cache[sourceImage]) {
		t.Fatalf("upload cache = %+v, want shein uploaded url for source", cache)
	}
}

func TestSubmitTaskSaveDraftAllowsMissingStrictPublishImageRoles(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "OK"},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"})
	if err != nil {
		t.Fatalf("save draft should allow missing strict publish image roles: %v", err)
	}
}

func TestSubmitTaskSaveDraftDoesNotRequireFinalConfirmation(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       false,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "OK"},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"}); err != nil {
		t.Fatalf("save draft should not require final confirmation: %v", err)
	}
}

func TestSubmitTaskPublishAllowsMissingSizeMapRole(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage},
		ImageRoleOverrides: map[string]string{
			sourceImage: "swatch",
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{sourceImage})
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalled := false
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalled = true
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("publish err = %v, want missing size map to be non-blocking", err)
	}
	readiness := buildSheinSubmitReadinessForAction(task.Result.Shein, "publish")
	foundSizeMapBlocker := false
	if readiness != nil {
		for _, item := range readiness.BlockingItems {
			if strings.Contains(item.Message, "尺寸图") {
				foundSizeMapBlocker = true
				break
			}
		}
	}
	if foundSizeMapBlocker {
		t.Fatalf("publish readiness = %+v, want no size map blocker", readiness)
	}
	if !publishCalled {
		t.Fatal("publish should be called when only size map is missing")
	}
}

func TestSubmitTaskPublishRepairsMissingSKCImagesFromFinalDraft(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	sourceImage := "https://oss.shuomiai.com/listingkit/source-main.png"
	sizeImage := "https://oss.shuomiai.com/listingkit/source-size.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    sourceImage,
		FinalImageOrder: []string{sourceImage, sizeImage},
		ImageRoleOverrides: map[string]string{
			sizeImage: "size_map",
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: sourceImage,
		Gallery:   []string{sourceImage, sizeImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].ImageInfo = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].MainImage = sourceImage
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{sourceImage, sizeImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = sheinproduct.ImageInfo{}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var submitted *sheinproduct.Product
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true},
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if len(submitted.SKCList) == 0 || len(submitted.SKCList[0].ImageInfo.ImageInfoList) == 0 {
		t.Fatalf("submitted skc images = %+v, want repaired images", submitted.SKCList)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result.Shein.RequestDraft.SKCList[0].ImageInfo == nil || strings.TrimSpace(saved.Result.Shein.RequestDraft.SKCList[0].ImageInfo.MainImage) == "" {
		t.Fatalf("saved request skc image = %+v, want repaired main image", saved.Result.Shein.RequestDraft.SKCList[0].ImageInfo)
	}
	if len(saved.Result.Shein.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList) == 0 {
		t.Fatalf("saved preview skc images = %+v, want repaired images", saved.Result.Shein.PreviewProduct.SKCList[0].ImageInfo)
	}
}

func TestSubmitTaskBlocksSharedSingleImageAcrossMultipleSKCs(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{},
	}
	mainImage := "https://oss.shuomiai.com/listingkit/shared-main.png"
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: mainImage,
		Gallery:   []string{mainImage, "https://oss.shuomiai.com/listingkit/size-map.png"},
	}
	task.Result.Shein.RequestDraft.SKCList = []sheinpub.SKCRequestDraft{
		{
			SkcName:      "black",
			SaleName:     "black",
			SupplierCode: "BLACK",
			ImageInfo:    &SheinImageDraft{MainImage: mainImage},
			SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "BLACK-20OZ", MainImage: mainImage, Attributes: map[string]string{"Color": "black"}}},
		},
		{
			SkcName:      "gray",
			SaleName:     "gray",
			SupplierCode: "GRAY",
			ImageInfo:    &SheinImageDraft{MainImage: mainImage},
			SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "GRAY-20OZ", MainImage: mainImage, Attributes: map[string]string{"Color": "gray"}}},
		},
		{
			SkcName:      "Pale pink",
			SaleName:     "Pale pink",
			SupplierCode: "PALE-PINK",
			ImageInfo:    &SheinImageDraft{MainImage: mainImage},
			SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "PALE-PINK-20OZ", MainImage: mainImage, Attributes: map[string]string{"Color": "Pale pink"}}},
		},
	}
	task.Result.Shein.SkcList = []sheinpub.SKCPackage{
		{SkcName: "black", SaleName: "black", SupplierCode: "BLACK", MainImageURL: mainImage},
		{SkcName: "gray", SaleName: "gray", SupplierCode: "GRAY", MainImageURL: mainImage},
		{SkcName: "Pale pink", SaleName: "Pale pink", SupplierCode: "PALE-PINK", MainImageURL: mainImage},
	}
	task.Result.Shein.PreviewProduct = sheinpub.BuildPreviewProduct(task.Result.Shein)
	task.Result.SDSSync = &SDSSyncSummary{
		Status: "failed",
		Error:  "SDS render failed for selected color variants: gray, Pale pink",
		VariantResults: []SDSSyncSummary{
			{VariantColor: "black", Status: "completed", MockupImageURLs: []string{mainImage}},
			{VariantColor: "gray", Status: "failed"},
			{VariantColor: "Pale pink", Status: "failed"},
		},
	}
	task.Result.Summary = &GenerationSummary{}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	publishCalled := false
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					publishCalled = true
				},
			},
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submit err = %v, want readiness block", err)
	}
	if publishCalled {
		t.Fatal("publish should not be called when variant image coverage is incomplete")
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Summary == nil || !saved.Result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review", saved.Result.Summary)
	}
	if len(saved.Result.ReviewReasons) == 0 || !strings.Contains(saved.Result.ReviewReasons[0], "gray, Pale pink") {
		t.Fatalf("review reasons = %#v, want failed variant reason", saved.Result.ReviewReasons)
	}
	for _, skc := range saved.Result.Shein.RequestDraft.SKCList {
		if skc.ImageInfo == nil || strings.TrimSpace(skc.ImageInfo.MainImage) == "" {
			t.Fatalf("skc image info = %+v, want preserved shared images", skc.ImageInfo)
		}
	}
	if saved.Result.Shein.Metadata[sheinVariantImageCoverageStatusKey] != "blocked" {
		t.Fatalf("metadata = %#v, want blocked variant image coverage status", saved.Result.Shein.Metadata)
	}
}

func TestSubmitReadinessDerivesSwatchFromSKCImage(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	mainImage := "https://oss.shuomiai.com/listingkit/main.png"
	sizeImage := "https://oss.shuomiai.com/listingkit/size.png"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    mainImage,
		FinalImageOrder: []string{mainImage, sizeImage},
		ImageRoleOverrides: map[string]string{
			sizeImage: "size_map",
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: mainImage,
		Gallery:   []string{mainImage, sizeImage},
	}
	task.Result.Shein.RequestDraft.SKCList[0].ImageInfo = &SheinImageDraft{
		MainImage: mainImage,
		Gallery:   []string{mainImage, sizeImage},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{mainImage, sizeImage})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{mainImage, sizeImage})

	readiness := buildSheinSubmitReadinessForAction(task.Result.Shein, "publish")
	if readiness == nil || !readiness.Ready {
		t.Fatalf("readiness = %+v, want ready because submit derives swatch from SKC image", readiness)
	}
}

func sheinImageInfo(urls []string) *sheinproduct.ImageInfo {
	info := &sheinproduct.ImageInfo{
		ImageInfoList: make([]sheinproduct.ImageDetail, 0, len(urls)),
	}
	for index, url := range urls {
		imageType := 2
		if index == 0 {
			imageType = 1
		}
		info.ImageInfoList = append(info.ImageInfoList, sheinproduct.ImageDetail{
			ImageType:          imageType,
			ImageSort:          index + 1,
			ImageURL:           url,
			MarketingMainImage: index == 0,
		})
	}
	return info
}
