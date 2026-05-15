package api

import (
	"encoding/json"
	"testing"
)

func TestListingDTOsAcceptJavaListingModuleFields(t *testing.T) {
	t.Run("store response metadata", func(t *testing.T) {
		var dto StoreRespDTO
		if err := json.Unmarshal([]byte(`{"id":1,"createTime":"2026-05-15T01:02:03","creator":"admin"}`), &dto); err != nil {
			t.Fatalf("unmarshal StoreRespDTO: %v", err)
		}
		if dto.Creator != "admin" || dto.CreateTime == nil {
			t.Fatalf("StoreRespDTO metadata = creator %q createTime %#v", dto.Creator, dto.CreateTime)
		}
	})

	t.Run("rule and mapping fields", func(t *testing.T) {
		var filter FilterRuleRespDTO
		if err := json.Unmarshal([]byte(`{"priceType":"special"}`), &filter); err != nil {
			t.Fatalf("unmarshal FilterRuleRespDTO: %v", err)
		}
		if filter.PriceType != "special" {
			t.Fatalf("FilterRuleRespDTO.PriceType = %q", filter.PriceType)
		}

		var profit ProfitRuleRespDTO
		if err := json.Unmarshal([]byte(`{"tenantId":88}`), &profit); err != nil {
			t.Fatalf("unmarshal ProfitRuleRespDTO: %v", err)
		}
		if profit.TenantID != 88 {
			t.Fatalf("ProfitRuleRespDTO.TenantID = %d", profit.TenantID)
		}

		var mapping ProductImportMappingRespDTO
		if err := json.Unmarshal([]byte(`{"createTime":"2026-05-15T01:02:03"}`), &mapping); err != nil {
			t.Fatalf("unmarshal ProductImportMappingRespDTO: %v", err)
		}
		if mapping.CreateTime == nil {
			t.Fatal("ProductImportMappingRespDTO.CreateTime is nil")
		}
	})

	t.Run("task request compatibility fields", func(t *testing.T) {
		submit := TaskSubmitReqDTO{
			SourcePlatform: "amazon",
			TargetPlatform: "shein",
		}
		payload, err := json.Marshal(submit)
		if err != nil {
			t.Fatalf("marshal TaskSubmitReqDTO: %v", err)
		}
		if !jsonContains(payload, "sourcePlatform") || !jsonContains(payload, "targetPlatform") {
			t.Fatalf("TaskSubmitReqDTO json = %s", payload)
		}

		status := TaskStatusReqDTO{TaskID: 7, TenantID: 9, IncludeDetails: true, IncludeLogs: true}
		payload, err = json.Marshal(status)
		if err != nil {
			t.Fatalf("marshal TaskStatusReqDTO: %v", err)
		}
		for _, key := range []string{"tenantId", "includeDetails", "includeLogs"} {
			if !jsonContains(payload, key) {
				t.Fatalf("TaskStatusReqDTO json missing %s: %s", key, payload)
			}
		}
	})

	t.Run("update requests include remark", func(t *testing.T) {
		store := StoreStatusUpdateReqDTO{ID: 1, Status: 0, Remark: "expired"}
		payload, err := json.Marshal(store)
		if err != nil {
			t.Fatalf("marshal StoreStatusUpdateReqDTO: %v", err)
		}
		if !jsonContains(payload, "remark") {
			t.Fatalf("StoreStatusUpdateReqDTO json = %s", payload)
		}

		task := ProductImportTaskUpdateReqDTO{ID: 2, Status: 3, Remark: "retry later"}
		payload, err = json.Marshal(task)
		if err != nil {
			t.Fatalf("marshal ProductImportTaskUpdateReqDTO: %v", err)
		}
		if !jsonContains(payload, "remark") {
			t.Fatalf("ProductImportTaskUpdateReqDTO json = %s", payload)
		}
	})
}

func TestRawJsonDataResponseIncludesTaskID(t *testing.T) {
	var dto RawJsonDataRespDTO
	if err := json.Unmarshal([]byte(`{"id":1,"taskId":99}`), &dto); err != nil {
		t.Fatalf("unmarshal RawJsonDataRespDTO: %v", err)
	}
	if dto.TaskID != 99 {
		t.Fatalf("RawJsonDataRespDTO.TaskID = %d", dto.TaskID)
	}
}

func jsonContains(payload []byte, key string) bool {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return false
	}
	_, ok := raw[key]
	return ok
}
