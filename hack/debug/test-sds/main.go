package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
	"task-processor/internal/sds/adapter"
	"task-processor/internal/sds/client"
	"task-processor/internal/sds/design"
	"task-processor/internal/sds/template"
	"task-processor/internal/sds/workflow"
)

func main() {
	var (
		mode          = flag.String("mode", "option-groups", "login|option-groups|list|detail|cycle|recommend|sync-url|sync-file|sync-result-file|process-and-sync")
		token         = flag.String("token", "", "SDS access-token")
		merchantID    = flag.Int64("merchant-id", 0, "SDS merchant id")
		username      = flag.String("username", "", "SDS username")
		password      = flag.String("password", "", "SDS password")
		merchantName  = flag.String("merchant-name", "", "SDS merchant name")
		domainName    = flag.String("domain-name", "www.sdsdiy.com", "SDS login domain name")
		extraInfo     = flag.String("extra-info", "", "SDS login extraInfo payload")
		verifyCaptcha = flag.String("verify-captcha-param", "", "SDS login verifyCaptchaParam payload")
		productID     = flag.String("product-id", "", "SDS product id")
		variantID     = flag.Int64("variant-id", 0, "SDS variant id")
		parentID      = flag.Int64("parent-id", 0, "SDS parent product id")
		imageURL      = flag.String("image-url", "", "remote image url for design sync")
		imageFile     = flag.String("image-file", "", "local image file for design sync")
		resultFile    = flag.String("result-file", "", "productimage result json file for design sync")
		prototypeID   = flag.Int64("prototype-group-id", 0, "SDS prototype group id")
		layerID       = flag.String("layer-id", "", "SDS layer id")
		fitLevel      = flag.Float64("fit-level", 1, "fabric fit level")
		resizeMode    = flag.Int("resize-mode", 0, "fabric resize mode")
		page          = flag.Int("page", 1, "page")
		size          = flag.Int("size", 20, "size")
		keyword       = flag.String("keyword", "", "keyword")
		sortField     = flag.String("sort-field", "", "sort field")
		sortType      = flag.String("sort-type", "", "sort type")
		shipmentArea  = flag.String("shipment-area", "overseas", "shipment area")
		overseasArea  = flag.String("overseas-area", "overseas", "overseas area")
		sideActiveID  = flag.String("side-active-id", "overseas", "side active id")
		preciseSearch = flag.Int("precise-search", 0, "precise search")
	)
	flag.Parse()

	cfg := client.DefaultConfig()
	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("create sds client: %v", err)
	}

	if *token != "" {
		c.SetAuthState(&client.AuthState{
			AccessToken: *token,
			MerchantID:  *merchantID,
		})
		if err := c.SaveAuthState(); err != nil {
			log.Fatalf("save auth state: %v", err)
		}
	}

	svc := template.NewService(c)
	ctx := context.Background()

	var out any

	switch *mode {
	case "login":
		requireUsername(*username)
		requirePassword(*password)
		out, err = c.Login(ctx, client.LoginRequest{
			MerchantName:       *merchantName,
			Username:           *username,
			Password:           *password,
			DomainName:         *domainName,
			VerifyCaptchaParam: *verifyCaptcha,
			ExtraInfo:          *extraInfo,
		})
	case "option-groups":
		out, err = svc.GetOptionGroups(ctx, template.OptionGroupParams{
			Size:          *size,
			Page:          *page,
			PreciseSearch: *preciseSearch,
			ShipmentArea:  *shipmentArea,
			OverseasArea:  *overseasArea,
		})
	case "list":
		out, err = svc.ListProducts(ctx, template.ListParams{
			Page:          *page,
			Size:          *size,
			Keyword:       *keyword,
			SortField:     *sortField,
			SortType:      *sortType,
			SideActiveID:  *sideActiveID,
			PreciseSearch: fmt.Sprintf("%d", *preciseSearch),
			ShipmentArea:  *shipmentArea,
			OverseasArea:  *overseasArea,
			IsOverseas:    *overseasArea,
		})
	case "detail":
		requireProductID(*productID)
		out, err = svc.GetProduct(ctx, *productID)
	case "cycle":
		requireProductID(*productID)
		out, err = svc.GetCycle(ctx, *productID)
	case "recommend":
		requireProductID(*productID)
		out, err = svc.GetRecommendations(ctx, *productID)
	case "sync-url":
		requireVariantID(*variantID)
		requireImageURL(*imageURL)
		wf := workflow.NewService(design.NewService(c))
		out, err = wf.SyncDesignFromURL(ctx, workflow.SyncInput{
			VariantID:        *variantID,
			ParentProductID:  *parentID,
			PrototypeGroupID: *prototypeID,
			LayerID:          *layerID,
			FitLevel:         *fitLevel,
			ResizeMode:       *resizeMode,
		}, workflow.ImageSource{
			URL: *imageURL,
		})
	case "sync-file":
		requireVariantID(*variantID)
		requireImageFile(*imageFile)
		wf := workflow.NewService(design.NewService(c))
		out, err = wf.SyncDesignFromFile(ctx, workflow.SyncInput{
			VariantID:        *variantID,
			ParentProductID:  *parentID,
			PrototypeGroupID: *prototypeID,
			LayerID:          *layerID,
			FitLevel:         *fitLevel,
			ResizeMode:       *resizeMode,
		}, workflow.FileSource{
			Path: *imageFile,
		})
	case "sync-result-file":
		requireVariantID(*variantID)
		requireResultFile(*resultFile)
		payload, readErr := os.ReadFile(*resultFile)
		if readErr != nil {
			log.Fatalf("read result file: %v", readErr)
		}
		var imageResult productimage.ImageProcessResult
		if unmarshalErr := json.Unmarshal(payload, &imageResult); unmarshalErr != nil {
			log.Fatalf("unmarshal result file: %v", unmarshalErr)
		}
		sdsWorkflow := workflow.NewService(design.NewService(c))
		sdsAdapter := adapter.NewService(nil, sdsWorkflow)
		out, err = sdsAdapter.SyncFromImageResult(ctx, workflow.SyncInput{
			VariantID:        *variantID,
			ParentProductID:  *parentID,
			PrototypeGroupID: *prototypeID,
			LayerID:          *layerID,
			FitLevel:         *fitLevel,
			ResizeMode:       *resizeMode,
		}, &imageResult)
	case "process-and-sync":
		requireVariantID(*variantID)
		requireImageURL(*imageURL)
		imageSvc, imageErr := productimage.NewService(&productimage.ServiceConfig{
			TaskRepo: productimagestore.NewMemTaskRepository(),
		})
		if imageErr != nil {
			log.Fatalf("create productimage service: %v", imageErr)
		}
		sdsWorkflow := workflow.NewService(design.NewService(c))
		sdsAdapter := adapter.NewService(imageSvc, sdsWorkflow)
		out, err = sdsAdapter.SyncFromImageRequest(ctx, adapter.SyncFromImageRequestInput{
			SyncInput: workflow.SyncInput{
				VariantID:        *variantID,
				ParentProductID:  *parentID,
				PrototypeGroupID: *prototypeID,
				LayerID:          *layerID,
				FitLevel:         *fitLevel,
				ResizeMode:       *resizeMode,
			},
			ImageRequest: &productimage.ImageProcessRequest{
				ImageURLs:   []string{*imageURL},
				Marketplace: "amazon",
			},
		})
	default:
		log.Fatalf("unsupported mode: %s", *mode)
	}

	if err != nil {
		log.Fatalf("sds request failed: %v", err)
	}

	data, err := jsonx.MarshalPretty(out)
	if err != nil {
		log.Fatalf("marshal output: %v", err)
	}

	_, _ = os.Stdout.Write(data)
	_, _ = os.Stdout.Write([]byte("\n"))
}

func requireProductID(productID string) {
	if productID == "" {
		log.Fatal("-product-id is required")
	}
}

func requireVariantID(variantID int64) {
	if variantID <= 0 {
		log.Fatal("-variant-id is required")
	}
}

func requireImageURL(imageURL string) {
	if imageURL == "" {
		log.Fatal("-image-url is required")
	}
}

func requireResultFile(resultFile string) {
	if resultFile == "" {
		log.Fatal("-result-file is required")
	}
}

func requireImageFile(imageFile string) {
	if imageFile == "" {
		log.Fatal("-image-file is required")
	}
}

func requireUsername(username string) {
	if username == "" {
		log.Fatal("-username is required")
	}
}

func requirePassword(password string) {
	if password == "" {
		log.Fatal("-password is required")
	}
}
