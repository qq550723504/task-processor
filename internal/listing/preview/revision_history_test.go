package preview

import "testing"

func TestBuildRevisionHistoryMeta(t *testing.T) {
	t.Parallel()

	meta := BuildRevisionHistoryMeta(RevisionHistoryMetaInput{
		TotalRecords:    23,
		ReturnedRecords: 5,
		MaxRecords:      20,
	})
	if meta == nil {
		t.Fatal("meta = nil")
	}
	if meta.TotalRecords != 23 || meta.ReturnedRecords != 5 {
		t.Fatalf("meta = %+v", meta)
	}
	if !meta.HasMore || !meta.IsTruncated {
		t.Fatalf("meta = %+v, want truncated", meta)
	}
	if meta.MaxRecords != 20 {
		t.Fatalf("max records = %d", meta.MaxRecords)
	}
}

func TestBuildRevisionHistoryMetaBackfillsTotalAndDefaultMax(t *testing.T) {
	t.Parallel()

	meta := BuildRevisionHistoryMeta(RevisionHistoryMetaInput{
		ReturnedRecords: 3,
	})
	if meta == nil {
		t.Fatal("meta = nil")
	}
	if meta.TotalRecords != 3 || meta.ReturnedRecords != 3 {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.HasMore || meta.IsTruncated {
		t.Fatalf("meta = %+v, want not truncated", meta)
	}
	if meta.MaxRecords != DefaultMaxRevisionHistoryRecords {
		t.Fatalf("max records = %d", meta.MaxRecords)
	}
}
