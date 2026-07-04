package productsync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/shein/api/marketing"
	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestMarketingAPISaveConfigSendsPromotionType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mrs-api-prefix/mbrs/activity/auto_partake/save_config_v2" {
			http.NotFound(w, r)
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["type"]; got != float64(1) {
			t.Fatalf("type = %v, want 1", got)
		}
		configList := body["config_list"].([]any)
		config := configList[0].(map[string]any)
		if _, ok := config["act_stock"]; ok {
			t.Fatalf("regular activity config includes act_stock: %#v", config)
		}
		if _, ok := config["reserved_act_stock"]; ok {
			t.Fatalf("regular activity config includes reserved_act_stock: %#v", config)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "0", "msg": "OK"})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	api := NewMarketingAPI(baseClient)

	_, err := api.SaveConfig(&marketing.SaveConfigRequest{
		ConfigList: []marketing.ActivityConfig{
			{
				Skc:               "sg260618173737193036297",
				DropRate:          63,
				SitePriceInfoList: []marketing.ActivitySitePriceInfo{},
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}
