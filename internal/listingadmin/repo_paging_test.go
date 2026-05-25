package listingadmin

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type repoPagingTestRow struct {
	ID   int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Name string `gorm:"column:name"`
}

func (repoPagingTestRow) TableName() string {
	return "repo_paging_test_rows"
}

func TestFindPagedRowsNormalizesPagingAndOrdersByLatestID(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repoPagingTestRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, row := range []repoPagingTestRow{{Name: "first"}, {Name: "second"}, {Name: "third"}} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	var rows []repoPagingTestRow
	total, page, pageSize, err := findPagedRows(db.Table("repo_paging_test_rows"), 0, 999, &rows)
	if err != nil {
		t.Fatalf("findPagedRows: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if page != 1 || pageSize != 200 {
		t.Fatalf("page/pageSize = %d/%d, want 1/200", page, pageSize)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0].Name != "third" || rows[1].Name != "second" || rows[2].Name != "first" {
		t.Fatalf("rows = %+v, want descending id order", rows)
	}
}
