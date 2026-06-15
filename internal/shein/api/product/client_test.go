package product

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestQueryBrandList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetQueryBrandListEndpoint() {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "OK",
			"info": map[string]any{
				"data": []map[string]any{
					{
						"brand_code":    "2fd1n",
						"brand_name":    "Logitech罗技",
						"brand_name_en": "Logitech",
					},
				},
				"meta": map[string]any{
					"count":     1,
					"customObj": nil,
				},
			},
			"bbl": nil,
		})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	resp, err := client.QueryBrandList()
	if err != nil {
		t.Fatalf("QueryBrandList() error = %v", err)
	}
	if resp.Info.Meta.Count != 1 {
		t.Fatalf("count = %d, want 1", resp.Info.Meta.Count)
	}
	if len(resp.Info.Data) != 1 {
		t.Fatalf("brand count = %d, want 1", len(resp.Info.Data))
	}
	if resp.Info.Data[0].BrandCode != "2fd1n" {
		t.Fatalf("brand code = %q, want 2fd1n", resp.Info.Data[0].BrandCode)
	}
	if resp.Info.Data[0].BrandNameEn != "Logitech" {
		t.Fatalf("brand name en = %q, want Logitech", resp.Info.Data[0].BrandNameEn)
	}
}
