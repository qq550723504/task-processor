package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubSheinPODImageLookupHandlerService struct {
	ctx   context.Context
	query *listingkit.SheinPODImageLookupQuery
	items []listingkit.SheinPODImageLookupRecord
	total int64
	err   error
}

func (s *stubSheinPODImageLookupHandlerService) LookupSheinPODImages(ctx context.Context, query *listingkit.SheinPODImageLookupQuery) ([]listingkit.SheinPODImageLookupRecord, int64, error) {
	s.ctx = ctx
	s.query = query
	return s.items, s.total, s.err
}

func TestLookupSheinPODImagesReturnsMatchedItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubSheinPODImageLookupHandlerService{
		items: []listingkit.SheinPODImageLookupRecord{{
			TaskID:             "000a11f9-b41e-4e7f-bd9d-b3cefd739012",
			StoreID:            869,
			SellerSKU:          "XB0606012001-V49720-T000A11F9-R4012C1-14624330",
			SheinSPUName:       "g2605302354951131",
			SheinVersion:       "SPMP260530352497648",
			AIOriginalImageURL: "https://oss.shuomiai.com/listingkit-assets/20260530/d669b6d0-833c-4567-a39f-480e03a58fc3.png",
			SDSMainImageURL:    "https://cdn.sdspod.com/out/0/202605/f95d77f558fa121c28ba51b1f1926f5d.jpg",
		}},
		total: 1,
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSheinPODImageLookupService(service))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-pod-image-lookup/stores/:store_id", h.LookupSheinPODImages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-pod-image-lookup/stores/869?query=XB0606012001V49720-T000A11F9-R4012C1-14624330&limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if service.query == nil {
		t.Fatal("service query was not captured")
	}
	if service.query.StoreID != 869 {
		t.Fatalf("store id = %d, want 869", service.query.StoreID)
	}
	if service.query.Query != "XB0606012001V49720-T000A11F9-R4012C1-14624330" {
		t.Fatalf("query = %q", service.query.Query)
	}
	if service.query.Limit != 10 {
		t.Fatalf("limit = %d, want 10", service.query.Limit)
	}

	var body struct {
		Items []listingkit.SheinPODImageLookupRecord `json:"items"`
		Total int64                                  `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("body total/items = %d/%d, want 1/1", body.Total, len(body.Items))
	}
	if body.Items[0].SheinSPUName != "g2605302354951131" {
		t.Fatalf("spu name = %q", body.Items[0].SheinSPUName)
	}
}

func TestLookupSheinPODImagesRequiresQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithSheinPODImageLookupService(&stubSheinPODImageLookupHandlerService{}))
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-pod-image-lookup/stores/:store_id", h.LookupSheinPODImages)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-pod-image-lookup/stores/869", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
