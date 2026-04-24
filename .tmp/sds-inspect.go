package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"task-processor/internal/sds/client"
	"task-processor/internal/sds/design"
)

func main() {
	c, err := client.New(client.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	svc := design.NewService(c)
	page, err := svc.GetDesignProduct(context.Background(), 212097)
	if err != nil {
		log.Fatal(err)
	}
	raw, _ := json.MarshalIndent(map[string]any{
		"productPrototypeID":            page.Product.PrototypeID,
		"prototypeGroup":                page.PrototypeGroup,
		"layers":                        page.Layers,
		"psds":                          page.PSDs,
		"merchantProductParentID":       page.MerchantProductParentID,
		"merchantProductResultGroupID":  page.MerchantProductResultGroupID,
		"designStatus":                  page.DesignStatus,
		"productImgURL":                 page.Product.ImgURL,
		"productPSDImgURL":              page.Product.PSDImgURL,
		"merchantProduct":               page.MerchantProduct,
	}, "", "  ")
	fmt.Println(string(raw))
}
