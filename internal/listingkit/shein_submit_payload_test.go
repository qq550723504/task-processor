package listingkit

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPrepareSheinProductForNewSubmitDefaultsShelfWay(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{
					ImageType: 1,
					ImageSort: 1,
					ImageURL:  "https://img.example.com/a.jpg",
				}},
			},
			SKUS: []sheinproduct.SKU{{
				StockInfoList: []sheinproduct.StockInfo{{
					MerchantWarehouseCode: "DEFAULT",
					InventoryNum:          10,
				}},
			}},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	if got := product.SKCList[0].ShelfWay; got != defaultSheinSKCShelfWay {
		t.Fatalf("shelf_way = %d, want %d", got, defaultSheinSKCShelfWay)
	}
}
