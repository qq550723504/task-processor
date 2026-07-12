package store

import (
	"context"
	"github.com/imroc/req/v3"
	"net/http"
	"net/http/httptest"
	"reflect"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/client"
	"testing"
)

func TestSiteInfoHandlerUsesDynamicSites(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"main_site":"shein","sub_site_list":[{"site_abbr":"shein-fr","site_status":1},{"site_abbr":"off","site_status":0}]}]}}`))
	}))
	defer server.Close()
	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "US"})
	ctx.ProductData = &product.Product{}
	ctx.ProductAPI = product.NewClient(client.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	if err := NewSiteInfoHandler().Handle(ctx); err != nil {
		t.Fatal(err)
	}
	want := []product.SiteInfo{{MainSite: "shein", SubSiteList: []string{"shein-fr"}}}
	if !reflect.DeepEqual(ctx.SiteList, want) || !reflect.DeepEqual(ctx.ProductData.SiteList, want) {
		t.Fatalf("sites ctx=%#v product=%#v", ctx.SiteList, ctx.ProductData.SiteList)
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
}

func TestSiteInfoHandlerFallsBackOnAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"1","msg":"bad"}`))
	}))
	defer server.Close()
	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "FR"})
	ctx.ProductData = &product.Product{}
	ctx.ProductAPI = product.NewClient(client.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	if err := NewSiteInfoHandler().Handle(ctx); err != nil {
		t.Fatal(err)
	}
	if got := ctx.SiteList[0].SubSiteList[0]; got != "shein-fr" {
		t.Fatalf("got %s", got)
	}
}
