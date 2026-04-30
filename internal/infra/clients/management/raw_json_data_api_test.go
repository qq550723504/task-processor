package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/infra/clients/management/api"
)

func TestRawJsonDataAPIClientCreateRawJsonData_AcceptsStringID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data":    "12345",
		})
	}))
	defer server.Close()

	client := &RawJsonDataAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL(server.URL),
	}
	client.SetUserToken("token", "1")

	id, err := client.CreateRawJsonData(&api.RawJsonDataCreateReqDTO{
		TenantID:  1,
		StoreID:   838,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B0TEST",
		Creator:   "test",
	})
	if err != nil {
		t.Fatalf("CreateRawJsonData returned error: %v", err)
	}
	if id != 12345 {
		t.Fatalf("CreateRawJsonData id=%d, want 12345", id)
	}
}

func TestParseInt64Result_AcceptsNumericJSON(t *testing.T) {
	id, err := parseInt64Result(json.RawMessage(`123`))
	if err != nil {
		t.Fatalf("parseInt64Result returned error: %v", err)
	}
	if id != 123 {
		t.Fatalf("parseInt64Result id=%d, want 123", id)
	}
}
