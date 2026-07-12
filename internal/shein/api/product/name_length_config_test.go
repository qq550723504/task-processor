package product

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestClientQueryProductNameLengthConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/spmp-api-prefix/spmp/product/publish/config/query_product_name_length_config" {
			t.Errorf("path = %s, want product name length config endpoint", r.URL.Path)
		}

		var body struct {
			CategoryID int `json:"category_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body.CategoryID != 1772 {
			t.Errorf("category_id = %d, want 1772", body.CategoryID)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "OK",
			"info": []map[string]any{
				{"language": "en", "max_length": 150},
				{"language": "zh-cn", "max_length": 100},
				{"language": "zh-tw", "max_length": 105},
			},
		})
	}))
	defer server.Close()

	client := NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	got, err := client.QueryProductNameLengthConfig(1772)
	if err != nil {
		t.Fatalf("QueryProductNameLengthConfig() error = %v", err)
	}
	want := []NameLengthConfigItem{
		{Language: "en", MaxLength: 150},
		{Language: "zh-cn", MaxLength: 100},
		{Language: "zh-tw", MaxLength: 105},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("QueryProductNameLengthConfig() = %#v, want %#v", got, want)
	}
}
