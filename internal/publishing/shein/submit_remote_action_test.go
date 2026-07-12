package shein

import (
	"errors"
	"testing"

	listingsubmission "task-processor/internal/listing/submission"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestExecuteSubmitRemoteDispatchesSaveDraftAndBuildsSummary(t *testing.T) {
	t.Parallel()

	api := submitRemoteActionTestProductAPI{
		saveDraftFunc: func(product *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
			if product == nil || product.SupplierCode != "SUP-1" {
				t.Fatalf("product = %+v, want supplier code SUP-1", product)
			}
			return &sheinproduct.SheinResponse{Code: "0", Msg: "OK", Info: sheinproduct.ResponseInfo{SPUName: "SPU-1"}}, "", nil
		},
	}

	result, err := ExecuteSubmitRemote(&api, listingsubmission.SubmitActionSaveDraft, &sheinproduct.Product{SupplierCode: "SUP-1"})
	if err != nil {
		t.Fatalf("ExecuteSubmitRemote() error = %v", err)
	}
	if result == nil || result.Raw == nil || result.Response == nil {
		t.Fatalf("result = %+v, want raw and summary", result)
	}
	if result.Response.Code != "0" || result.Response.Message != "OK" || result.Response.SPUName != "SPU-1" {
		t.Fatalf("summary = %+v, want normalized SHEIN response", result.Response)
	}
	if api.called != listingsubmission.SubmitActionSaveDraft {
		t.Fatalf("called = %q, want save_draft", api.called)
	}
}

func TestExecuteSubmitRemoteDispatchesPublishAndReturnsErrorWithSummary(t *testing.T) {
	t.Parallel()

	remoteErr := errors.New("remote failed")
	api := submitRemoteActionTestProductAPI{
		publishFunc: func(product *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
			return &sheinproduct.SheinResponse{Code: "500", Msg: "bad"}, "", remoteErr
		},
	}

	result, err := ExecuteSubmitRemote(&api, listingsubmission.SubmitActionPublish, &sheinproduct.Product{})
	if !errors.Is(err, remoteErr) {
		t.Fatalf("ExecuteSubmitRemote() error = %v, want %v", err, remoteErr)
	}
	if result == nil || result.Response == nil || result.Response.Code != "500" {
		t.Fatalf("result = %+v, want response summary even on remote error", result)
	}
	if api.called != listingsubmission.SubmitActionPublish {
		t.Fatalf("called = %q, want publish", api.called)
	}
}

func TestExecuteSubmitRemoteRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	result, err := ExecuteSubmitRemote(&submitRemoteActionTestProductAPI{}, "delete", &sheinproduct.Product{})
	if err == nil || err.Error() != "unsupported submit action: delete" {
		t.Fatalf("ExecuteSubmitRemote() error = %v, want unsupported action", err)
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil", result)
	}
}

type submitRemoteActionTestProductAPI struct {
	called        string
	saveDraftFunc func(product *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error)
	publishFunc   func(product *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error)
}

func (p *submitRemoteActionTestProductAPI) GetProduct(productID string) (*sheinproduct.Product, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) UpdateProduct(product *sheinproduct.Product) error {
	return nil
}
func (p *submitRemoteActionTestProductAPI) DeleteProduct(productID string) error {
	return nil
}
func (p *submitRemoteActionTestProductAPI) GetPartInfo(categoryID int) (*sheinproduct.PartInfoResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) SaveDraftProduct(product *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	p.called = listingsubmission.SubmitActionSaveDraft
	if p.saveDraftFunc == nil {
		return nil, "", nil
	}
	return p.saveDraftFunc(product)
}
func (p *submitRemoteActionTestProductAPI) PublishProduct(product *sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	p.called = listingsubmission.SubmitActionPublish
	if p.publishFunc == nil {
		return nil, "", nil
	}
	return p.publishFunc(product)
}
func (p *submitRemoteActionTestProductAPI) ConfirmPublish(product *sheinproduct.Product) (bool, string, error) {
	return false, "", nil
}
func (p *submitRemoteActionTestProductAPI) Record(request *sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) ListProducts(pageNum, pageSize int, request *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QueryStock(request *sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QueryInventory(spuName string) (*sheinproduct.InventoryQueryResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) UpdateInventory(request *sheinproduct.InventoryUpdateRequest) error {
	return nil
}
func (p *submitRemoteActionTestProductAPI) QueryPrice(spuName string) (*sheinproduct.PriceQueryResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QueryCostPrice(spuName string, skcNameList []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QueryProductNameLengthConfig(categoryID int) ([]sheinproduct.NameLengthConfigItem, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QueryLanguageList() ([]sheinproduct.LanguageListItem, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) QuerySiteList() ([]sheinproduct.SiteListGroup, error) {
	return nil, nil
}
func (p *submitRemoteActionTestProductAPI) OffShelf(request *sheinproduct.ShelfOperateRequest) error {
	return nil
}
func (p *submitRemoteActionTestProductAPI) OnShelf(request *sheinproduct.ShelfOperateRequest) error {
	return nil
}
