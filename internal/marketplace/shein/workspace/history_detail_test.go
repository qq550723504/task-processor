package workspace

import "testing"

func TestBuildHistoryDetailProjectsMetadata(t *testing.T) {
	record := "record"
	payload := "payload"
	navigation := &HistoryNavigation{PrevRevisionID: "prev", NextRevisionID: "next"}

	detail := BuildHistoryDetail("task-1", &record, navigation, &payload, 2, 5, true, 10)

	if detail == nil {
		t.Fatal("detail = nil")
	}
	if detail.TaskID != "task-1" || detail.Record != &record || detail.RestorePayload != &payload {
		t.Fatalf("detail payload = %#v", detail)
	}
	if detail.Navigation != navigation || detail.HistoryIndex != 2 || detail.TotalRecords != 5 || !detail.IsTruncated || detail.MaxRecords != 10 {
		t.Fatalf("detail metadata = %#v", detail)
	}
}
