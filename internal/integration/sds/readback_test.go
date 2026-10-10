package sds

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"task-processor/internal/product/pod"
)

func TestReadbackQualifiesCurrentSavedDesignProtocol(t *testing.T) {
	fabric := `{"version":"5.2.1","objects":[{"type":"image","sds":{"originUrl":"https://cdn.sdspod.com/images1000Thumbs/test/art.png?material_id=480953643&thumbsZoom=1"},"src":"https://cdn.sdspod.com/images1000Thumbs/test/art.png?material_id=480953643&thumbsZoom=1"}]}`
	intent := pod.DesignIntent{OperationID: "12752596-6056-4316-9f2f-380c97df9675", MerchantID: "123", ParentID: "95146", VariantID: "95147", PrototypeID: "730897612975054849", GroupID: "15261", Materials: []pod.MaterialReference{{ID: "480953643", FileCode: "art.png"}}, Layers: []pod.DesignLayer{{ID: "730897620537384960", Width: 567, Height: 850, PrintWidth: 1334, PrintHeight: 2000, FabricJSON: fabric}}, RenderFileIDs: []string{"753068348962332672"}}
	finished := `{"id":"962657110282047488","key_id":"test-finished","merchant_id":123,"task_id":"962657107283120129","design_task_id":"962657107283120129","product_parent_id":95146,"product_id":95147,"prototype_id":"730897612975054849","status":2,"buildFinish":true,"img_urls":["https://cdn.sdspod.com/out/123/test/finished.jpg"]}`
	saved := savedDesignDTO{FinishedID: "962657110282047488", PrototypeID: remoteID(intent.PrototypeID)}
	saved.Product.ID = remoteID(intent.VariantID)
	saved.Product.ParentID = remoteID(intent.ParentID)
	saved.Group.ID = remoteID(intent.GroupID)
	savedJSON := `{"designProductId":"962657110282047488","product":{"id":95147,"parent_id":95146},"prototypeGroup":{"id":15261},"prototype_id":"730897612975054849","layers":[{"id":"730897620537384960","width":567,"height":850,"print_width":1334,"print_height":2000,"fabric_json":FABRIC}],"designFiles":[{"id":"753068348962332672"}]}`
	encodedFabric, _ := json.Marshal(fabric)
	savedJSON = strings.Replace(savedJSON, "FABRIC", string(encodedFabric), 1)
	calls := []string{}
	client, _ := NewReadbackClient(&http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.Method+" "+r.URL.Host+r.URL.Path)
		if r.Method != "GET" {
			t.Fatal("readback mutated provider")
		}
		var body string
		switch r.URL.Path {
		case "/design_products":
			if r.URL.Query().Get("search") != intent.OperationID || r.URL.Query().Get("size") != "10" {
				t.Fatal("unbounded candidate query")
			}
			body = `{"total_count":1,"items":[` + finished + `]}`
		case "/design_products/962657110282047488":
			body = strings.Replace(finished, `"buildFinish":true,`, "", 1)
		case "/ps/endproducts/962657110282047488/design":
			body = savedJSON
		case "/ps/task/result":
			body = `{"total":1,"items":[{"id":"962657107283120129","status":5,"qty_complete":1,"qty_success":1,"qty_failed":0,"qty_wait":0}]}`
		case "/ps/task/962657107283120129/children":
			body = `{"taskId":"962657107283120129","items":[{"productId":95147,"productParentId":95146,"results":[{"file":"http://cdn.sdspod.com/out/123/test/finished.jpg"}]}]}`
		default:
			t.Fatalf("unqualified endpoint: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}, readbackCredentials{Credentials{BindingID: "platform-sds", Revision: "revision-1", MerchantID: "123", AccessToken: "test-token"}})
	proof, err := client.Observe(context.Background(), Binding{ID: "platform-sds", Revision: "revision-1", MerchantID: "123"}, intent)
	if err != nil || proof.Reference().ID != "962657110282047488" || len(calls) != 5 {
		t.Fatalf("real protocol shape not qualified: %v %#v", err, calls)
	}
}

type readbackRoundTripper func(*http.Request) (*http.Response, error)

func (f readbackRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type readbackCredentials struct{ value Credentials }

func (c readbackCredentials) Current(context.Context) (Credentials, error) { return c.value, nil }

func TestReadbackOnlyGetsFixedProviderPathsAndDoesNotRetry(t *testing.T) {
	calls := 0
	client, err := NewReadbackClient(&http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Scheme != "https" || r.URL.Host != "mapi2.sdspod.com" || r.URL.Path != "/design_products" || r.Header.Get("Access-Token") != "test-token" {
			t.Fatalf("unsafe request: %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("temporarily unavailable")), Request: r}, nil
	})}, readbackCredentials{Credentials{BindingID: "platform-sds", Revision: "revision-1", MerchantID: "123", AccessToken: "test-token"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Observe(context.Background(), Binding{ID: "platform-sds", Revision: "revision-1", MerchantID: "123"}, pod.DesignIntent{OperationID: "12752596-6056-4316-9f2f-380c97df9675", MerchantID: "123"})
	if !errors.Is(err, pod.ErrUnknown) || calls != 1 {
		t.Fatalf("readback did not fail closed: calls=%d err=%v", calls, err)
	}
}

func TestReadbackRefusesBindingRotationBeforeNetwork(t *testing.T) {
	calls := 0
	client, _ := NewReadbackClient(&http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) { calls++; return nil, errors.New("must not send") })}, readbackCredentials{Credentials{BindingID: "platform-sds", Revision: "new-revision", MerchantID: "123", AccessToken: "test-token"}})
	_, err := client.Observe(context.Background(), Binding{ID: "platform-sds", Revision: "old-revision", MerchantID: "123"}, pod.DesignIntent{OperationID: "12752596-6056-4316-9f2f-380c97df9675", MerchantID: "123"})
	if !errors.Is(err, pod.ErrUnknown) || calls != 0 {
		t.Fatalf("cross-binding read: calls=%d err=%v", calls, err)
	}
}
