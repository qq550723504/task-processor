package warehouse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestGetWarehousesUsesLegacyEndpointWhenAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetWarehousesEndpoint() {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "OK",
			"info": map[string]any{
				"data": []map[string]any{
					{
						"warehouse_name":    "legacy-wh",
						"warehouse_code":    "LEGACY001",
						"sale_country_list": []string{"US"},
						"warehouse_type":    3,
					},
				},
				"meta": map[string]any{
					"count": 1,
				},
			},
		})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	resp, err := client.GetWarehouses()
	if err != nil {
		t.Fatalf("GetWarehouses() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("warehouse count = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].WarehouseCode != "LEGACY001" {
		t.Fatalf("warehouse code = %q, want LEGACY001", resp.Data[0].WarehouseCode)
	}
}

func TestGetWarehousesFallsBackToStoreAddressEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case sheinclient.GetWarehousesEndpoint():
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "0",
				"msg":  "OK",
				"info": map[string]any{
					"data": []any{},
					"meta": map[string]any{
						"count": 0,
					},
				},
			})
		case sheinclient.GetStoreAddressListEndpoint():
			var body StoreAddressListRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if body.AddressType != 2 {
				t.Fatalf("addressType = %d, want 2", body.AddressType)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "0",
				"msg":  "OK",
				"info": map[string]any{
					"addresses": []map[string]any{
						{
							"warehouseName": "美国仓-萨克拉门托海外工厂-95828",
							"warehouseCode": "WH2604273684342787",
							"warehouseType": 3,
							"storeSiteInfos": []map[string]any{
								{
									"site":          "shein-us",
									"saleCountries": []string{"US"},
								},
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	resp, err := client.GetWarehouses()
	if err != nil {
		t.Fatalf("GetWarehouses() error = %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("warehouse count = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].WarehouseCode != "WH2604273684342787" {
		t.Fatalf("warehouse code = %q, want WH2604273684342787", resp.Data[0].WarehouseCode)
	}
	if resp.Data[0].WarehouseName != "美国仓-萨克拉门托海外工厂-95828" {
		t.Fatalf("warehouse name = %q", resp.Data[0].WarehouseName)
	}
}

func TestAddStoreAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetStoreAddressAddEndpoint() {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		var body StoreAddressAddRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.WarehouseName != "test" {
			t.Fatalf("warehouseName = %q, want test", body.WarehouseName)
		}
		if len(body.BindSites) != 1 || body.BindSites[0] != "shein-us" {
			t.Fatalf("bindSites = %#v", body.BindSites)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "OK",
			"info": map[string]any{},
		})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	err := client.AddStoreAddress(&StoreAddressAddRequest{
		Address1:              "17950 Ajax Cir",
		AddressLeafID:         "54105",
		FirstName:             "sds",
		LastName:              "sds",
		Phone:                 "1235856545",
		AddressType:           2,
		PostCode:              "91748",
		CollectionPatternType: 2,
		BindSites:             []string{"shein-us"},
		SellerEmail:           "zone@shuomiai.com",
		Lat:                   "33.9997094",
		Lng:                   "-117.9129391",
		WarehouseName:         "test",
		WarehouseType:         3,
		IsRefundAddress:       "2",
		CollectionJudgeRecord: &CollectionJudgeRecord{
			TriggerReason:     1,
			InCollectionRange: 1,
			CollectionType:    []int{2},
			Operator:          "seller",
			OperateTime:       "2026-04-27 16:21:12",
			MaxAdo:            0,
			MultipleShopStore: 2,
		},
		CollectionMark: 1,
		ProviderInfoList: []ProviderInfo{
			{ProviderID: 1021, ProviderName: "GOFO", IsMatchAdo: 1},
			{ProviderID: 680, ProviderName: "UniUni", IsMatchAdo: 1},
		},
		CheckResultUUID: "1f841524c2aa417dba7f16fda64a325a",
	})
	if err != nil {
		t.Fatalf("AddStoreAddress() error = %v", err)
	}
}

func TestListStoreAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetStoreAddressListEndpoint() {
			http.NotFound(w, r)
			return
		}

		var body StoreAddressListRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.AddressType != 2 {
			t.Fatalf("addressType = %d, want 2", body.AddressType)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "OK",
			"info": map[string]any{
				"addresses": []map[string]any{
					{
						"address1":              "17950 Ajax Cir",
						"cityId":                54105,
						"postCode":              "91748",
						"firstName":             "sds",
						"lastName":              "sds",
						"phone":                 "+1 1235856545",
						"addressType":           2,
						"warehouseName":         "美国仓-加州工业城四号海外工厂-91748",
						"warehouseCode":         "WH2604273679490051",
						"warehouseType":         3,
						"collectionPatternType": 2,
						"collectionMark":        1,
						"sellerEmail":           "zone@shuomiai.com",
						"storeSiteInfos": []map[string]any{
							{"site": "shein-us", "siteStatus": 1, "defaultWarehouse": 2},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	resp, err := client.ListStoreAddresses(2)
	if err != nil {
		t.Fatalf("ListStoreAddresses() error = %v", err)
	}
	if len(resp.Addresses) != 1 {
		t.Fatalf("address count = %d, want 1", len(resp.Addresses))
	}
	if resp.Addresses[0].CityID != 54105 {
		t.Fatalf("cityId = %d", resp.Addresses[0].CityID)
	}
}

func TestCheckStoreAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != sheinclient.GetStoreAddressCheckEndpoint() {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		var body StoreAddressCheckRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.AddressLeafID != "54105" {
			t.Fatalf("addressLeafId = %q, want 54105", body.AddressLeafID)
		}
		if body.QueryLatLngAddress != 2 {
			t.Fatalf("queryLatLngAddress = %d, want 2", body.QueryLatLngAddress)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": "0",
			"msg":  "OK",
			"info": map[string]any{
				"collectionList": []map[string]any{
					{"collection": 2, "collectionName": "上门揽收"},
				},
				"collectionJudgeRecord": map[string]any{
					"triggerReason":     1,
					"inCollectionRange": 1,
					"collectionType":    []int{2},
					"operator":          "seller",
					"operateTime":       "2026-04-27 16:21:12",
					"maxAdo":            0,
					"multipleShopStore": 2,
				},
				"latLng": map[string]any{
					"lat": "33.9997094",
					"lng": "-117.9129391",
				},
				"collectionMark": 1,
				"providerInfoList": []map[string]any{
					{"providerId": 1021, "providerName": "GOFO", "isMatchAdo": 1},
					{"providerId": 680, "providerName": "UniUni", "isMatchAdo": 1},
				},
				"checkResultUUid": "1f841524c2aa417dba7f16fda64a325a",
			},
		})
	}))
	defer server.Close()

	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	client := NewClient(baseClient)

	resp, err := client.CheckStoreAddress(&StoreAddressCheckRequest{
		AddressLeafID:      "54105",
		Address1:           "17950 Ajax Cir",
		PostCode:           "91748",
		QueryLatLngAddress: 2,
	})
	if err != nil {
		t.Fatalf("CheckStoreAddress() error = %v", err)
	}
	if resp == nil {
		t.Fatal("CheckStoreAddress() returned nil response")
	}
	if resp.LatLng == nil || resp.LatLng.Lat != "33.9997094" || resp.LatLng.Lng != "-117.9129391" {
		t.Fatalf("latLng = %#v", resp.LatLng)
	}
	if len(resp.CollectionList) != 1 || resp.CollectionList[0].Collection != 2 {
		t.Fatalf("collectionList = %#v", resp.CollectionList)
	}
	if resp.CollectionJudgeRecord == nil || resp.CollectionJudgeRecord.InCollectionRange != 1 {
		t.Fatalf("collectionJudgeRecord = %#v", resp.CollectionJudgeRecord)
	}
	if resp.CheckResultUUID != "1f841524c2aa417dba7f16fda64a325a" {
		t.Fatalf("checkResultUUid = %q", resp.CheckResultUUID)
	}
}
