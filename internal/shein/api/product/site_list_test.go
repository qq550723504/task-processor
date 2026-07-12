package product

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/imroc/req/v3"
	sheinclient "task-processor/internal/shein/client"
)

func TestClientQuerySiteList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/spmp-api-prefix/spmp/supplier/query_site_list" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 0 {
			t.Errorf("body = %#v, err = %v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"main_site":"shein","main_site_name":"SHEIN","sub_site_list":[{"site_name":"SHEIN美国站","site_abbr":"shein-us","site_status":1,"store_type":1,"currency":"USD"}]}]}}`))
	}))
	defer server.Close()
	client := NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	got, err := client.QuerySiteList()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MainSite != "shein" || len(got[0].SubSiteList) != 1 || got[0].SubSiteList[0].Currency != "USD" {
		t.Fatalf("got %#v", got)
	}
}
