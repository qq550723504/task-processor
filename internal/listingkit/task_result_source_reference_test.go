package listingkit

import "testing"

func TestBuildTaskResultIncludesPersistedSourceReference(t *testing.T) {
	task := &Task{ID: "task-detail-source", TenantID: "tenant-a", Request: &GenerateRequest{
		Source: &SourceReference{
			Key: "crawler:1688:888", Type: "crawler", Platform: "1688", ID: "888",
			URL: "https://detail.1688.com/offer/888.html",
		},
	}}
	result := buildTaskResult(task, nil)
	if result.SourceReference == nil || result.SourceReference.ID != "888" {
		t.Fatalf("source_reference = %+v, want persisted source identity", result.SourceReference)
	}
	if result.SourceReference == task.Request.Source {
		t.Fatal("source_reference shares request pointer, want defensive copy")
	}
}

func TestBuildTaskResultOmitsLegacySourceReference(t *testing.T) {
	result := buildTaskResult(&Task{ID: "legacy-task"}, nil)
	if result.SourceReference != nil {
		t.Fatalf("source_reference = %+v, want nil for legacy task", result.SourceReference)
	}
}
