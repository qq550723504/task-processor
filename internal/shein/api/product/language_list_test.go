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

func TestClientQueryLanguageList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/spmp-api-prefix/spmp/basic/get_language_list" {
			t.Errorf("path = %s, want language list endpoint", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if len(body) != 0 {
			t.Errorf("body = %#v, want empty object", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"language_abbr":"en","language_name":"英语","input_mode":1},{"language_abbr":"fr","language_name":"法语","input_mode":1}],"meta":{"count":39,"customObj":null}}}`))
	}))
	defer server.Close()

	client := NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	got, err := client.QueryLanguageList()
	if err != nil {
		t.Fatalf("QueryLanguageList() error = %v", err)
	}
	want := []LanguageListItem{{LanguageAbbr: "en", LanguageName: "英语", InputMode: 1}, {LanguageAbbr: "fr", LanguageName: "法语", InputMode: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("QueryLanguageList() = %#v, want %#v", got, want)
	}
}
