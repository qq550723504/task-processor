package listingadmin

import "testing"

func TestApplySensitiveWordDefaultsSetsLanguageAndLevel(t *testing.T) {
	t.Parallel()

	row := listingSensitiveWord{}
	applySensitiveWordDefaults(&row)

	if row.Language != "en" {
		t.Fatalf("language = %q, want en", row.Language)
	}
	if row.Level != 1 {
		t.Fatalf("level = %d, want 1", row.Level)
	}
}

func TestApplySensitiveWordAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingSensitiveWord{}
	applySensitiveWordAuditFields(&row, "user-1", true)

	if row.OwnerUserID != "user-1" {
		t.Fatalf("ownerUserID = %q, want user-1", row.OwnerUserID)
	}
	if row.Creator != "user-1" || row.CreatedBy != "user-1" {
		t.Fatalf("creator/createdBy = %q/%q, want user-1/user-1", row.Creator, row.CreatedBy)
	}
	if row.Updater != "user-1" || row.UpdatedBy != "user-1" {
		t.Fatalf("updater/updatedBy = %q/%q, want user-1/user-1", row.Updater, row.UpdatedBy)
	}
}

func TestListingSensitiveWordConversionPreservesFields(t *testing.T) {
	t.Parallel()

	word := SensitiveWord{
		ID:          1,
		TenantID:    101,
		Word:        " restricted ",
		Language:    " en ",
		Tags:        " policy ",
		Level:       3,
		ReplaceWord: " safe ",
		Remark:      " note ",
		Status:      2,
	}

	row := listingSensitiveWordFromSensitiveWord(&word)
	if row.Word != "restricted" || row.Language != "en" || row.Tags != "policy" {
		t.Fatalf("trimmed row = %+v, want trimmed strings", row)
	}

	converted := row.toSensitiveWord()
	if converted.Level != 3 || converted.Status != 2 {
		t.Fatalf("converted = %+v, want numeric fields preserved", converted)
	}
	if converted.ReplaceWord != "safe" || converted.Remark != "note" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
}
