package addresscopy

import (
	"reflect"
	"testing"

	"task-processor/internal/shein/api/warehouse"
)

func TestAddressLeafIDUsesMostSpecificLevel(t *testing.T) {
	addr := &warehouse.StoreAddress{DistrictID: 9, CityID: 8, StateID: 7}
	if got := addressLeafID(addr); got != "9" {
		t.Fatalf("addressLeafID() = %q, want 9", got)
	}

	addr = &warehouse.StoreAddress{CityID: 8, StateID: 7}
	if got := addressLeafID(addr); got != "8" {
		t.Fatalf("addressLeafID() = %q, want 8", got)
	}
}

func TestNormalizePhoneUS(t *testing.T) {
	addr := &warehouse.StoreAddress{CountryID: 226, Phone: "+1 1235856545"}
	if got := normalizePhone(addr); got != "1235856545" {
		t.Fatalf("normalizePhone() = %q", got)
	}
}

func TestBuildAddRequest(t *testing.T) {
	addr := &warehouse.StoreAddress{
		CountryID:             226,
		CityID:                54105,
		Address1:              "17950 Ajax Cir",
		FirstName:             "sds",
		LastName:              "sds",
		Phone:                 "+1 1235856545",
		AddressType:           2,
		PostCode:              "91748",
		CollectionPatternType: 2,
		SellerEmail:           "zone@shuomiai.com",
		WarehouseName:         "test",
		WarehouseType:         3,
		CollectionMark:        1,
		StoreSiteInfos: []warehouse.StoreSiteInfo{
			{Site: "shein-us"},
		},
	}
	checkInfo := &warehouse.StoreAddressCheckInfo{
		CollectionList: []warehouse.CollectionOption{
			{Collection: 2, CollectionName: "上门揽收"},
		},
		CollectionJudgeRecord: &warehouse.CollectionJudgeRecord{
			TriggerReason:     1,
			InCollectionRange: 1,
			CollectionType:    []int{2},
			Operator:          "seller",
			OperateTime:       "2026-04-27 16:21:12",
		},
		LatLng:         &warehouse.LatLng{Lat: "33.9997094", Lng: "-117.9129391"},
		CollectionMark: 1,
		ProviderInfoList: []warehouse.ProviderInfo{
			{ProviderID: 1021, ProviderName: "GOFO", IsMatchAdo: 1},
		},
		CheckResultUUID: "uuid-1",
	}

	got, err := buildAddRequest(addr, checkInfo)
	if err != nil {
		t.Fatalf("buildAddRequest() error = %v", err)
	}

	if got.AddressLeafID != "54105" {
		t.Fatalf("AddressLeafID = %q", got.AddressLeafID)
	}
	if got.Phone != "1235856545" {
		t.Fatalf("Phone = %q", got.Phone)
	}
	if !reflect.DeepEqual(got.BindSites, []string{"shein-us"}) {
		t.Fatalf("BindSites = %#v", got.BindSites)
	}
	if got.CheckResultUUID != "uuid-1" {
		t.Fatalf("CheckResultUUID = %q", got.CheckResultUUID)
	}
}
