package marketing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestClientAutoPartakeUsesV2Endpoints(t *testing.T) {
	requestedPaths := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths[r.URL.Path] = true
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mrs-api-prefix/mbrs/activity/auto_partake/get_available_skc_list_v2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "0",
				"msg":  "OK",
				"info": map[string]any{
					"total":         0,
					"skc_info_list": []any{},
				},
			})
		case "/mrs-api-prefix/mbrs/activity/auto_partake/get_config_list_v2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "0",
				"msg":  "OK",
				"info": map[string]any{
					"total":       0,
					"config_list": []any{},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	if _, err := client.GetAvailableSkcList(&GetAvailableSkcListRequest{PageNum: 1, PageSize: 20}); err != nil {
		t.Fatalf("GetAvailableSkcList() error = %v", err)
	}
	if _, err := client.GetConfigList(&GetConfigListRequest{PageNum: 1, PageSize: 20}); err != nil {
		t.Fatalf("GetConfigList() error = %v", err)
	}

	if !requestedPaths["/mrs-api-prefix/mbrs/activity/auto_partake/get_available_skc_list_v2"] {
		t.Fatal("GetAvailableSkcList did not request v2 endpoint")
	}
	if !requestedPaths["/mrs-api-prefix/mbrs/activity/auto_partake/get_config_list_v2"] {
		t.Fatal("GetConfigList did not request v2 endpoint")
	}
}

func TestClientSaveConfigSendsPromotionType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mrs-api-prefix/mbrs/activity/auto_partake/save_config_v2" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
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
	client := NewClient(baseClient)

	_, err := client.SaveConfig(&SaveConfigRequest{
		ConfigList: []ActivityConfig{
			{
				Skc:               "sg260618173737193036297",
				DropRate:          63,
				SitePriceInfoList: []ActivitySitePriceInfo{},
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func TestClientSaveConfigSendsStockForLimitedPromotionType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mrs-api-prefix/mbrs/activity/auto_partake/save_config_v2" {
			http.NotFound(w, r)
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["type"]; got != float64(2) {
			t.Fatalf("type = %v, want 2", got)
		}
		configList := body["config_list"].([]any)
		config := configList[0].(map[string]any)
		if got := config["act_stock"]; got != float64(499) {
			t.Fatalf("act_stock = %v, want 499", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "0", "msg": "OK"})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	_, err := client.SaveConfig(&SaveConfigRequest{
		Type: AutoPartakeActivityTypeLimited,
		ConfigList: []ActivityConfig{
			{
				Skc:               "sg260618173737193036297",
				ActStock:          499,
				DropRate:          63,
				SitePriceInfoList: []ActivitySitePriceInfo{},
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func TestClientUpdateConfigStateUsesV2Endpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mrs-api-prefix/mbrs/activity/auto_partake/update_config_state_v2" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["state"]; got != float64(1) {
			t.Fatalf("state = %v, want 1", got)
		}
		ids := body["ids"].([]any)
		if len(ids) != 2 || ids[0] != float64(13042096) || ids[1] != float64(13042097) {
			t.Fatalf("ids = %#v, want [13042096 13042097]", ids)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "0", "msg": "OK"})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	_, err := client.UpdateConfigState(&UpdateConfigStateRequest{
		IDs:   []int64{13042096, 13042097},
		State: AutoPartakeConfigStateOpen,
	})
	if err != nil {
		t.Fatalf("UpdateConfigState() error = %v", err)
	}
}
