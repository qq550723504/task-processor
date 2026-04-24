package template

import (
	"encoding/json"
	"testing"
)

func TestListRequestQueryDefaults(t *testing.T) {
	t.Parallel()

	params := ListParams{
		Page:         1,
		Size:         20,
		SideActiveID: "overseas",
		IsOverseas:   "overseas",
	}

	if params.Page != 1 || params.Size != 20 {
		t.Fatalf("unexpected params: %+v", params)
	}
}

func TestProductDetailUnmarshalRealFields(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id": 89763,
		"name": "美码成人烫画短袖T恤",
		"sku": "NS6001064",
		"blankDesignUrl": "https://cdn.sdspod.com/blank.jpg",
		"product_details": {
			"production_process": "烫画",
			"picture_request": "850 px * 1049 px"
		},
		"subproducts": {
			"attributers": [
				{
					"size": "S",
					"sizeId": 2058,
					"colors": [
						{
							"colorSort": 1,
							"color": "#000300",
							"opacity": 100,
							"color_name": "black",
							"colorId": 1001
						}
					]
				}
			],
			"items": [
				{
					"id": 89764,
					"parent_id": 89763,
					"sku": "NS6001064001",
					"size": "S",
					"sizeId": 2058,
					"colorId": 1001,
					"designPrototype": {
						"prototypeId": "698744758228934657",
						"prototypeGroupId": 14555,
						"productId": 89764,
						"productParentId": 89763,
						"buildType": "quick",
						"prototypeResultGroups": [
							{
								"id": "782092292317859840",
								"resultImage": "https://cdn.sdspod.com/result.jpg",
								"sort": 1,
								"prototypeId": "698744758228934657",
								"faceSheetState": true
							}
						],
						"prototypeLayerList": [
							{
								"id": "698744758333792256",
								"name": "素材",
								"type": 1,
								"height": 1049,
								"width": 850,
								"printHeight": 1049,
								"printWidth": 850,
								"maskShowUrl": "https://cdn.sdspod.com/mask.png",
								"imageUrl": "https://cdn.sdspod.com/layer.jpg"
							}
						]
					}
				}
			]
		}
	}`)

	var detail ProductDetail
	if err := json.Unmarshal(payload, &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}

	if detail.ID != 89763 {
		t.Fatalf("unexpected detail id: %d", detail.ID)
	}
	if detail.ProductDetails.PictureRequest != "850 px * 1049 px" {
		t.Fatalf("unexpected picture request: %q", detail.ProductDetails.PictureRequest)
	}
	if detail.Subproducts == nil || len(detail.Subproducts.Items) != 1 {
		t.Fatalf("unexpected subproducts: %+v", detail.Subproducts)
	}

	item := detail.Subproducts.Items[0]
	if item.ParentID != 89763 {
		t.Fatalf("unexpected parent id: %d", item.ParentID)
	}
	if item.DesignPrototype == nil {
		t.Fatal("expected design prototype")
	}
	if item.DesignPrototype.PrototypeID != "698744758228934657" {
		t.Fatalf("unexpected prototype id: %s", item.DesignPrototype.PrototypeID)
	}
	if len(item.DesignPrototype.PrototypeLayerList) != 1 {
		t.Fatalf("unexpected prototype layers: %+v", item.DesignPrototype.PrototypeLayerList)
	}
	if item.DesignPrototype.PrototypeLayerList[0].PrintWidth != 850 {
		t.Fatalf("unexpected print width: %.0f", item.DesignPrototype.PrototypeLayerList[0].PrintWidth)
	}
}
