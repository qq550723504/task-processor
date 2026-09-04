package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/sds/client"
	"task-processor/internal/sds/template"
)

func main() {
	var (
		mode          = flag.String("mode", "option-groups", "login|option-groups|list|detail|cycle|recommend")
		token         = flag.String("token", "", "SDS access-token")
		merchantID    = flag.Int64("merchant-id", 0, "SDS merchant id")
		username      = flag.String("username", "", "SDS username")
		password      = flag.String("password", "", "SDS password")
		merchantName  = flag.String("merchant-name", "", "SDS merchant name")
		domainName    = flag.String("domain-name", "www.sdsdiy.com", "SDS login domain name")
		extraInfo     = flag.String("extra-info", "", "SDS login extraInfo payload")
		verifyCaptcha = flag.String("verify-captcha-param", "", "SDS login verifyCaptchaParam payload")
		productID     = flag.String("product-id", "", "SDS product id")
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

	c, err := client.New(client.DefaultConfig())
	if err != nil {
		log.Fatalf("create sds client: %v", err)
	}
	if *token != "" {
		c.SetAuthState(&client.AuthState{AccessToken: *token, MerchantID: *merchantID})
		if err := c.SaveAuthState(); err != nil {
			log.Fatalf("save auth state: %v", err)
		}
	}

	svc := template.NewService(c)
	ctx := context.Background()
	var out any
	switch *mode {
	case "login":
		requireValue("-username", *username)
		requireValue("-password", *password)
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
			Size: *size, Page: *page, PreciseSearch: *preciseSearch,
			ShipmentArea: *shipmentArea, OverseasArea: *overseasArea,
		})
	case "list":
		out, err = svc.ListProducts(ctx, template.ListParams{
			Page: *page, Size: *size, Keyword: *keyword, SortField: *sortField, SortType: *sortType,
			SideActiveID: *sideActiveID, PreciseSearch: fmt.Sprintf("%d", *preciseSearch),
			ShipmentArea: *shipmentArea, OverseasArea: *overseasArea, IsOverseas: *overseasArea,
		})
	case "detail":
		requireValue("-product-id", *productID)
		out, err = svc.GetProduct(ctx, *productID)
	case "cycle":
		requireValue("-product-id", *productID)
		out, err = svc.GetCycle(ctx, *productID)
	case "recommend":
		requireValue("-product-id", *productID)
		out, err = svc.GetRecommendations(ctx, *productID)
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

func requireValue(name, value string) {
	if value == "" {
		log.Fatalf("%s is required", name)
	}
}
