package dub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := NewClient(Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewClient() error = %v, want ErrInvalidConfig", err)
	}
}

func TestNewClientRejectsInsecureRemoteBaseURL(t *testing.T) {
	_, err := NewClient(Config{BaseURL: "http://example.com", APIKey: "dub_test_secret"})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewClient() error = %v, want ErrInvalidConfig", err)
	}
}

func TestUpsertPartnerUsesStableExternalIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/partners" {
			t.Fatalf("request = %s %s, want POST /partners", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer dub_test_secret" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["tenantId"] != "user-42" || body["email"] != "partner@example.com" {
			t.Fatalf("body = %#v", body)
		}
		if body["country"] != "CN" {
			t.Fatalf("country = %#v, want CN", body["country"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"pn_1","name":"Partner","email":"partner@example.com","country":"CN"}`))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	partner, err := client.UpsertPartner(context.Background(), PartnerInput{
		ExternalID: " user-42 ", Email: "partner@example.com", Name: "Partner", Country: "cn",
	})
	if err != nil {
		t.Fatal(err)
	}
	if partner.ID != "pn_1" {
		t.Fatalf("partner.ID = %q", partner.ID)
	}
}

func TestPartnerNameLengthCountsCharactersNotUTF8Bytes(t *testing.T) {
	valid := normalizePartnerInput(PartnerInput{
		ExternalID: "partner-1",
		Email:      "partner@example.com",
		Name:       strings.Repeat("硕", 100),
	})
	if err := validatePartnerInput(valid); err != nil {
		t.Fatalf("100-character Unicode name rejected: %v", err)
	}

	invalid := valid
	invalid.Name = strings.Repeat("硕", 101)
	if err := validatePartnerInput(invalid); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("101-character Unicode name error = %v, want ErrInvalidInput", err)
	}
}

func TestLeadEventNameLengthCountsCharactersNotUTF8Bytes(t *testing.T) {
	valid := normalizeLeadInput(LeadInput{
		EventName:          strings.Repeat("转", 255),
		CustomerExternalID: "customer-1",
	})
	if err := validateLeadInput(valid); err != nil {
		t.Fatalf("255-character Unicode event rejected: %v", err)
	}

	invalid := valid
	invalid.EventName = strings.Repeat("转", 256)
	if err := validateLeadInput(invalid); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("256-character Unicode event error = %v, want ErrInvalidInput", err)
	}
}

func TestCreatePartnerLinkUsesDocumentedPartnerLinkFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/partners/links" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["tenantId"] != "affiliate-7" || body["url"] != "https://shuomi.example/pricing" || body["key"] != "alice" {
			t.Fatalf("top-level partner-link fields = %#v", body)
		}
		props, ok := body["linkProps"].(map[string]any)
		if !ok {
			t.Fatalf("linkProps = %#v", body["linkProps"])
		}
		if props["externalId"] != "ref-link-7" || props["tenantId"] != "affiliate-7" {
			t.Fatalf("linkProps = %#v", props)
		}
		if _, exists := props["trackConversion"]; exists {
			t.Fatalf("linkProps contains undocumented trackConversion: %#v", props)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"link_1","domain":"s.example","key":"alice","url":"https://shuomi.example/pricing","shortLink":"https://s.example/alice","externalId":"ref-link-7","tenantId":"affiliate-7","partnerId":"pn_7"}`))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	link, err := client.CreatePartnerLink(context.Background(), PartnerLinkInput{
		ExternalPartnerID: "affiliate-7",
		DestinationURL:    "https://shuomi.example/pricing",
		Key:               "alice",
		ExternalLinkID:    "ref-link-7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if link.ShortLink != "https://s.example/alice" || link.PartnerID != "pn_7" {
		t.Fatalf("link = %#v", link)
	}
}

func TestTrackLeadSendsClickAndStableCustomerIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track/lead" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["clickId"] != "click_123" || body["eventName"] != "Sign up" || body["customerExternalId"] != "customer-9" {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"click":{"id":"click_123"},"link":{"id":"link_1","partnerId":"pn_1","programId":"prog_1","tenantId":"affiliate-1","externalId":"ref-1","shortLink":"https://s.example/a"},"customer":{"externalId":"customer-9"}}`))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	result, err := client.TrackLead(context.Background(), LeadInput{
		ClickID: "click_123", EventName: "Sign up", CustomerExternalID: "customer-9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Link.PartnerID != "pn_1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestTrackLeadTreatsDuplicateNullAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, exists := body["clickId"]; exists {
			t.Fatalf("deferred lead payload contains clickId: %#v", body)
		}
		if body["customerExternalId"] != "customer-9" || body["eventName"] != "Sign up" {
			t.Fatalf("deferred lead body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(" null\n"))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	result, err := client.TrackLead(context.Background(), LeadInput{
		EventName: "Sign up", CustomerExternalID: "customer-9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil duplicate", result)
	}
}

func TestTrackSaleRequiresInvoiceId(t *testing.T) {
	client := mustTestClient(t, "https://api.dub.invalid")
	_, err := client.TrackSale(context.Background(), SaleInput{CustomerExternalID: "customer-1", Amount: 29900})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("TrackSale() error = %v, want ErrInvalidInput", err)
	}
}

func TestTrackSaleUsesInvoiceIdAndSafeDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track/sale" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["customerExternalId"] != "customer-1" || body["invoiceId"] != "invoice-1001" {
			t.Fatalf("body = %#v", body)
		}
		if body["currency"] != "usd" || body["eventName"] != "Invoice paid" || body["paymentProcessor"] != "custom" {
			t.Fatalf("defaults = %#v", body)
		}
		if body["amount"] != float64(29900) {
			t.Fatalf("amount = %#v", body["amount"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"eventName":"Invoice paid","customer":{"id":"cus_1","externalId":"customer-1"},"sale":{"amount":29900,"currency":"usd","paymentProcessor":"custom","invoiceId":"invoice-1001","metadata":{}}}`))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	result, err := client.TrackSale(context.Background(), SaleInput{
		CustomerExternalID: "customer-1", Amount: 29900, InvoiceID: "invoice-1001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Sale == nil || result.Customer == nil {
		t.Fatalf("result = %#v, want non-null customer and sale", result)
	}
	if result.Sale.InvoiceID != "invoice-1001" || result.Sale.Amount != 29900 {
		t.Fatalf("sale = %#v", result.Sale)
	}
}

func TestTrackSaleTreatsDuplicateNullAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("null\n"))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	result, err := client.TrackSale(context.Background(), SaleInput{
		CustomerExternalID: "customer-1", Amount: 29900, InvoiceID: "invoice-1001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil duplicate", result)
	}
}

func TestTrackSalePreservesNullableNestedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"eventName":"Invoice paid","customer":null,"sale":null}`))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	result, err := client.TrackSale(context.Background(), SaleInput{
		CustomerExternalID: "customer-1", Amount: 29900, InvoiceID: "invoice-1001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Customer != nil || result.Sale != nil {
		t.Fatalf("result = %#v, want documented nullable nested fields", result)
	}
}

func TestAPIErrorIsStructuredAndDoesNotLeakAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limit_exceeded","message":"try again later"}}`))
	}))
	defer server.Close()

	client := mustTestClient(t, server.URL)
	_, err := client.UpsertPartner(context.Background(), PartnerInput{ExternalID: "user-1", Email: "partner@example.com"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Code != "rate_limit_exceeded" {
		t.Fatalf("apiErr = %#v", apiErr)
	}
	if strings.Contains(err.Error(), "dub_test_secret") {
		t.Fatalf("error leaked api key: %v", err)
	}
}

func mustTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := NewClient(Config{BaseURL: baseURL, APIKey: "dub_test_secret"})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
