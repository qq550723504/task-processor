package publishing

import (
	"errors"
	"testing"
	"time"

	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestClassifyRemoteRecordHandlesMissingRecord(t *testing.T) {
	t.Parallel()

	outcome := ClassifyRemoteRecord("publish", nil, false)
	if outcome.Status != RemoteRecordStatusPending {
		t.Fatalf("status = %q, want pending", outcome.Status)
	}
	if outcome.Detail != "record not found" {
		t.Fatalf("detail = %q, want record not found", outcome.Detail)
	}
	if outcome.Err != nil {
		t.Fatalf("err = %v, want nil", outcome.Err)
	}
}

func TestBuildRemoteConfirmationPolicyForAcceptedPublish(t *testing.T) {
	t.Parallel()

	policy := BuildRemoteConfirmationPolicy("publish", true)
	if !policy.DefaultConfirmed {
		t.Fatal("DefaultConfirmed = false, want true")
	}
	if policy.RefreshFallbackMessage != "SHEIN accepted publish request; remote record not yet visible" {
		t.Fatalf("RefreshFallbackMessage = %q", policy.RefreshFallbackMessage)
	}
	if policy.ResolveFallbackMessage != "SHEIN accepted publish request; remote confirmation pending" {
		t.Fatalf("ResolveFallbackMessage = %q", policy.ResolveFallbackMessage)
	}
	if policy.MissingSupplierCodeStatus != RemoteRecordStatusConfirmed {
		t.Fatalf("MissingSupplierCodeStatus = %q, want confirmed", policy.MissingSupplierCodeStatus)
	}
	if policy.MissingSupplierCodeDetail != "SHEIN accepted publish request, but supplier code is unavailable for remote confirmation" {
		t.Fatalf("MissingSupplierCodeDetail = %q", policy.MissingSupplierCodeDetail)
	}
}

func TestBuildRemoteConfirmationPolicyForPendingRefresh(t *testing.T) {
	t.Parallel()

	policy := BuildRemoteConfirmationPolicy("publish", false)
	if policy.DefaultConfirmed {
		t.Fatal("DefaultConfirmed = true, want false")
	}
	if policy.RefreshFallbackMessage != "refreshing SHEIN remote record" {
		t.Fatalf("RefreshFallbackMessage = %q", policy.RefreshFallbackMessage)
	}
	if policy.ResolveFallbackMessage != "refreshing SHEIN remote record" {
		t.Fatalf("ResolveFallbackMessage = %q", policy.ResolveFallbackMessage)
	}
	if policy.MissingSupplierCodeStatus != RemoteRecordStatusPending {
		t.Fatalf("MissingSupplierCodeStatus = %q, want pending", policy.MissingSupplierCodeStatus)
	}
	if policy.MissingSupplierCodeDetail != "SHEIN submit succeeded, but supplier code is unavailable for remote confirmation" {
		t.Fatalf("MissingSupplierCodeDetail = %q", policy.MissingSupplierCodeDetail)
	}
}

func TestResolveRemoteConfirmationFallbackMessagePrefersExplicitFallback(t *testing.T) {
	t.Parallel()

	if got := ResolveRemoteConfirmationFallbackMessage("save_draft", false, " custom fallback "); got != "custom fallback" {
		t.Fatalf("fallback message = %q, want custom fallback", got)
	}
	if got := ResolveRemoteConfirmationFallbackMessage("publish", true, ""); got != "SHEIN accepted publish request; remote confirmation pending" {
		t.Fatalf("default fallback message = %q", got)
	}
}

func TestResolveRemoteConfirmationDecisionPrefersOnWayDocument(t *testing.T) {
	t.Parallel()

	decision := ResolveRemoteConfirmationDecision("publish", RemoteConfirmationResolution{
		DefaultConfirmed: true,
		OnWayDocument: &OnWayDocument{
			SpuName:    "SPU-1",
			DocumentSn: "DOC-1",
		},
	})
	if decision.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", decision.Status)
	}
	if decision.Detail != "SHEIN on-way document confirmed for spu_name=SPU-1 document_sn=DOC-1" {
		t.Fatalf("detail = %q", decision.Detail)
	}
	if decision.Err != nil {
		t.Fatalf("err = %v, want nil", decision.Err)
	}
}

func TestResolveRemoteConfirmationDecisionUsesRecordOutcome(t *testing.T) {
	t.Parallel()

	decision := ResolveRemoteConfirmationDecision("publish", RemoteConfirmationResolution{
		Record: &sheinproduct.RecordItem{State: 4, AuditState: 5},
	})
	if decision.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", decision.Status)
	}
	if decision.Detail != "SHEIN remote record confirmed" {
		t.Fatalf("detail = %q", decision.Detail)
	}
	if decision.Err != nil {
		t.Fatalf("err = %v, want nil", decision.Err)
	}
}

func TestResolveRemoteConfirmationDecisionUsesInventoryFallback(t *testing.T) {
	t.Parallel()

	decision := ResolveRemoteConfirmationDecision("publish", RemoteConfirmationResolution{
		InventoryConfirmed: true,
		SPUName:            "SPU-INV",
	})
	if decision.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", decision.Status)
	}
	if decision.Detail != "SHEIN remote inventory confirmed for spu_name=SPU-INV" {
		t.Fatalf("detail = %q", decision.Detail)
	}
}

func TestResolveRemoteConfirmationDecisionFallsBackForRecordErrorsAndDefaultConfirmed(t *testing.T) {
	t.Parallel()

	recordErrDecision := ResolveRemoteConfirmationDecision("publish", RemoteConfirmationResolution{
		DefaultConfirmed: true,
		FallbackMessage:  "fallback pending",
		RecordErr:        errors.New("remote query failed"),
	})
	if recordErrDecision.Status != RemoteRecordStatusPending {
		t.Fatalf("status = %q, want pending", recordErrDecision.Status)
	}
	if recordErrDecision.Detail != "fallback pending" {
		t.Fatalf("detail = %q, want fallback pending", recordErrDecision.Detail)
	}

	defaultConfirmedDecision := ResolveRemoteConfirmationDecision("publish", RemoteConfirmationResolution{
		DefaultConfirmed: true,
	})
	if defaultConfirmedDecision.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", defaultConfirmedDecision.Status)
	}
	if defaultConfirmedDecision.Detail != "SHEIN accepted publish request; remote confirmation pending" {
		t.Fatalf("detail = %q", defaultConfirmedDecision.Detail)
	}
}

func TestClassifyRemoteRecordConfirmsSaveDraft(t *testing.T) {
	t.Parallel()

	outcome := ClassifyRemoteRecord("save_draft", &sheinproduct.RecordItem{State: 1, AuditState: 2}, false)
	if outcome.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", outcome.Status)
	}
	if outcome.Detail != "SHEIN draft record confirmed" {
		t.Fatalf("detail = %q, want save draft confirmation", outcome.Detail)
	}
	if outcome.Err != nil {
		t.Fatalf("err = %v, want nil", outcome.Err)
	}
}

func TestClassifyRemoteRecordConfirmsAcceptedPublish(t *testing.T) {
	t.Parallel()

	outcome := ClassifyRemoteRecord("publish", &sheinproduct.RecordItem{State: 0, AuditState: 0}, true)
	if outcome.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", outcome.Status)
	}
	want := "SHEIN publish API reported success (state=0 audit_state=0)"
	if outcome.Detail != want {
		t.Fatalf("detail = %q, want %q", outcome.Detail, want)
	}
	if outcome.Err != nil {
		t.Fatalf("err = %v, want nil", outcome.Err)
	}
}

func TestClassifyRemoteRecordFailsDraftPublishState(t *testing.T) {
	t.Parallel()

	outcome := ClassifyRemoteRecord("publish", &sheinproduct.RecordItem{State: 1, AuditState: 2}, false)
	if outcome.Status != RemoteRecordStatusFailed {
		t.Fatalf("status = %q, want failed", outcome.Status)
	}
	want := "SHEIN publish landed in draft state (state=1 audit_state=2)"
	if outcome.Detail != want {
		t.Fatalf("detail = %q, want %q", outcome.Detail, want)
	}
	if outcome.Err == nil || outcome.Err.Error() != want {
		t.Fatalf("err = %v, want error %q", outcome.Err, want)
	}
}

func TestClassifyRemoteRecordConfirmsRemoteRecord(t *testing.T) {
	t.Parallel()

	outcome := ClassifyRemoteRecord("publish", &sheinproduct.RecordItem{State: 4, AuditState: 5}, false)
	if outcome.Status != RemoteRecordStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", outcome.Status)
	}
	if outcome.Detail != "SHEIN remote record confirmed" {
		t.Fatalf("detail = %q, want remote confirmation", outcome.Detail)
	}
	if outcome.Err != nil {
		t.Fatalf("err = %v, want nil", outcome.Err)
	}
}

func TestClassifyRemoteRecordLeavesPendingForIntermediateState(t *testing.T) {
	t.Parallel()

	outcome := ClassifyRemoteRecord("publish", &sheinproduct.RecordItem{State: 0, AuditState: 4}, false)
	if outcome.Status != RemoteRecordStatusPending {
		t.Fatalf("status = %q, want pending", outcome.Status)
	}
	want := "SHEIN remote record is not yet publish-confirmed (state=0 audit_state=4)"
	if outcome.Detail != want {
		t.Fatalf("detail = %q, want %q", outcome.Detail, want)
	}
	if outcome.Err != nil {
		t.Fatalf("err = %v, want nil", outcome.Err)
	}
}

func TestSelectRemoteRecordPrefersExactSPUName(t *testing.T) {
	t.Parallel()

	record := SelectRemoteRecord([]sheinproduct.RecordItem{
		{RecordID: "record-old", SpuName: "SPU-OLD", CreateTime: "2026-05-12 13:19:24"},
		{RecordID: "record-match", SpuName: "SPU-PUBLISH", CreateTime: "2026-05-12 13:10:00"},
		{RecordID: "record-new", SpuName: "SPU-NEW", CreateTime: "2026-05-12 13:45:13"},
	}, " SPU-PUBLISH ")

	if record == nil || record.RecordID != "record-match" {
		t.Fatalf("record = %+v, want exact spu match", record)
	}
}

func TestSelectRemoteRecordFallsBackToNewestCreateTime(t *testing.T) {
	t.Parallel()

	record := SelectRemoteRecord([]sheinproduct.RecordItem{
		{RecordID: "record-old", CreateTime: "2026-05-12 13:19:24"},
		{RecordID: "record-new", CreateTime: "2026-05-12T14:45:13Z"},
		{RecordID: "record-rfc3339", CreateTime: "2026-05-12T13:30:00Z"},
	}, "missing")

	if record == nil || record.RecordID != "record-new" {
		t.Fatalf("record = %+v, want newest record", record)
	}
}

func TestParseRemoteRecordTimeSupportsKnownLayouts(t *testing.T) {
	t.Parallel()

	gotLocal := ParseRemoteRecordTime("2026-05-12 13:45:13")
	if gotLocal.IsZero() {
		t.Fatal("ParseRemoteRecordTime(local) = zero, want parsed time")
	}
	gotRFC3339 := ParseRemoteRecordTime("2026-05-12T13:30:00Z")
	if gotRFC3339.IsZero() {
		t.Fatal("ParseRemoteRecordTime(rfc3339) = zero, want parsed time")
	}
	if !gotLocal.After(ParseRemoteRecordTime("2026-05-12 13:19:24")) {
		t.Fatalf("gotLocal = %v, want after older timestamp", gotLocal)
	}
	if ParseRemoteRecordTime("not-a-time") != (time.Time{}) {
		t.Fatalf("ParseRemoteRecordTime(invalid) = %v, want zero time", ParseRemoteRecordTime("not-a-time"))
	}
}

func TestSelectOnWayDocumentPrefersExactSPUNameWithDocumentSN(t *testing.T) {
	t.Parallel()

	doc := SelectOnWayDocument([]struct {
		SpuName    string `json:"spu_name"`
		SkcName    string `json:"skc_name"`
		DocumentSn string `json:"document_sn"`
	}{
		{SpuName: "SPU-OTHER", DocumentSn: "doc-other"},
		{SpuName: " SPU-PUBLISH ", SkcName: "SKC-1", DocumentSn: " doc-123 "},
	}, "spu-publish")

	if doc == nil || doc.SpuName != "SPU-PUBLISH" || doc.DocumentSn != "doc-123" {
		t.Fatalf("doc = %+v, want trimmed exact match", doc)
	}
}

func TestSelectOnWayDocumentFromResponseRejectsNonSuccess(t *testing.T) {
	t.Parallel()

	doc := SelectOnWayDocumentFromResponse(&sheinother.BatchCheckOnWayResponse{Code: "500"}, "SPU-PUBLISH")
	if doc != nil {
		t.Fatalf("doc = %+v, want nil", doc)
	}
}

func TestSelectRemoteRecordFromResponseUsesSelectionPolicy(t *testing.T) {
	t.Parallel()

	record, err := SelectRemoteRecordFromResponse(&sheinproduct.RecordResponse{
		Code: "0",
		Info: struct {
			Data []sheinproduct.RecordItem `json:"data"`
			Meta struct {
				Count     int `json:"count"`
				CustomObj struct {
					ScrollID string `json:"scroll_id"`
				} `json:"customObj"`
			} `json:"meta"`
		}{
			Data: []sheinproduct.RecordItem{
				{RecordID: "record-old", SpuName: "SPU-OLD", CreateTime: "2026-05-12 13:19:24"},
				{RecordID: "record-match", SpuName: "SPU-PUBLISH", CreateTime: "2026-05-12 13:10:00"},
			},
		},
	}, "SPU-PUBLISH")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if record == nil || record.RecordID != "record-match" {
		t.Fatalf("record = %+v, want selected match", record)
	}
}

func TestSelectRemoteRecordFromResponseUsesMessageForFailure(t *testing.T) {
	t.Parallel()

	record, err := SelectRemoteRecordFromResponse(&sheinproduct.RecordResponse{Code: "500", Msg: "bad query"}, "SPU-PUBLISH")
	if record != nil {
		t.Fatalf("record = %+v, want nil", record)
	}
	if err == nil || err.Error() != "bad query" {
		t.Fatalf("err = %v, want bad query", err)
	}
}

func TestInventoryConfirmedRequiresSuccessCodeAndSPUName(t *testing.T) {
	t.Parallel()

	if !InventoryConfirmed(&sheinproduct.InventoryQueryResponse{
		Code: "0",
		Info: sheinproduct.InventoryInfo{SpuName: "SPU-PUBLISH"},
	}) {
		t.Fatal("InventoryConfirmed(success) = false, want true")
	}
	if InventoryConfirmed(&sheinproduct.InventoryQueryResponse{Code: "500"}) {
		t.Fatal("InventoryConfirmed(non-success) = true, want false")
	}
	if InventoryConfirmed(&sheinproduct.InventoryQueryResponse{Code: "0"}) {
		t.Fatal("InventoryConfirmed(blank spu) = true, want false")
	}
}
