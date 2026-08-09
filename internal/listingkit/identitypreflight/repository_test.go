package identitypreflight

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const (
	listingTasksAggregateQuery = `SELECT CAST(tenant_id AS text) AS tenant_id,
       CAST(user_id AS text) AS user_id,
       COUNT(*) AS row_count
FROM listing_kit_tasks
WHERE NULLIF(BTRIM(CAST(tenant_id AS text)), '') IS NOT NULL
  AND NULLIF(BTRIM(CAST(user_id AS text)), '') IS NOT NULL
GROUP BY tenant_id, user_id`
	listingStoreAggregateQuery = `SELECT CAST(tenant_id AS text) AS tenant_id,
       CAST(owner_user_id AS text) AS user_id,
       COUNT(*) AS row_count
FROM listing_store
WHERE NULLIF(BTRIM(CAST(tenant_id AS text)), '') IS NOT NULL
  AND NULLIF(BTRIM(CAST(owner_user_id AS text)), '') IS NOT NULL
GROUP BY tenant_id, owner_user_id`
)

func TestPostgresOwnerRepositoryAggregatesConfiguredTablesWithReadOnlyQueries(t *testing.T) {
	t.Parallel()

	database, mock := openOwnerRepositorySQLMock(t, sqlmock.QueryMatcherEqual)
	mock.ExpectQuery(listingTasksAggregateQuery).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "user_id", "row_count"}).AddRow("tenant-a", "subject-a", int64(2))).
		RowsWillBeClosed()
	mock.ExpectQuery(listingStoreAggregateQuery).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "user_id", "row_count"}).AddRow("tenant-b", "subject-b", int64(3))).
		RowsWillBeClosed()
	repository := newPostgresOwnerRepository(sqlOwnerQueryer{db: database}, []OwnerTable{
		{Table: "listing_kit_tasks", TenantColumn: "tenant_id", UserColumn: "user_id"},
		{Table: "listing_store", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	})

	owners, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []PersistedOwner{
		{Table: "listing_kit_tasks", TenantID: "tenant-a", UserID: "subject-a", RowCount: 2},
		{Table: "listing_store", TenantID: "tenant-b", UserID: "subject-b", RowCount: 3},
	}
	if !reflect.DeepEqual(owners, want) {
		t.Fatalf("owners = %#v, want %#v", owners, want)
	}
}

func TestNewPostgresOwnerRepositoryUsesHardCodedReadOnlyInventory(t *testing.T) {
	t.Parallel()

	database, mock := openOwnerRepositorySQLMock(t, sqlmock.QueryMatcherFunc(matchReadOnlyInventoryQuery))
	for _, table := range ownerTableInventory {
		mock.ExpectQuery(table.Table).
			WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "user_id", "row_count"})).
			RowsWillBeClosed()
	}

	owners, err := NewPostgresOwnerRepository(database).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(owners) != 0 {
		t.Fatalf("owners = %#v, want none", owners)
	}
}

func TestPostgresOwnerRepositorySkipsOnlyPostgresUndefinedTables(t *testing.T) {
	t.Parallel()

	database, mock := openOwnerRepositorySQLMock(t, sqlmock.QueryMatcherEqual)
	optionalQuery := `SELECT CAST(tenant_id AS text) AS tenant_id,
       CAST(user_id AS text) AS user_id,
       COUNT(*) AS row_count
FROM optional_table
WHERE NULLIF(BTRIM(CAST(tenant_id AS text)), '') IS NOT NULL
  AND NULLIF(BTRIM(CAST(user_id AS text)), '') IS NOT NULL
GROUP BY tenant_id, user_id`
	mock.ExpectQuery(optionalQuery).
		WillReturnError(postgresStateError{state: "42P01", message: "relation optional_table does not exist"})
	mock.ExpectQuery(listingTasksAggregateQuery).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "user_id", "row_count"}).AddRow("tenant-a", "subject-a", int64(1))).
		RowsWillBeClosed()
	repository := newPostgresOwnerRepository(sqlOwnerQueryer{db: database}, []OwnerTable{
		{Table: "optional_table", TenantColumn: "tenant_id", UserColumn: "user_id"},
		{Table: "listing_kit_tasks", TenantColumn: "tenant_id", UserColumn: "user_id"},
	})

	owners, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []PersistedOwner{{Table: "listing_kit_tasks", TenantID: "tenant-a", UserID: "subject-a", RowCount: 1}}
	if !reflect.DeepEqual(owners, want) {
		t.Fatalf("owners = %#v, want %#v", owners, want)
	}
}

func TestPostgresOwnerRepositoryFailsOnOtherDatabaseErrorsWithoutLeakingDetails(t *testing.T) {
	t.Parallel()

	const databaseSecret = "raw-database-secret"
	database, mock := openOwnerRepositorySQLMock(t, sqlmock.QueryMatcherEqual)
	mock.ExpectQuery(listingStoreAggregateQuery).
		WillReturnError(postgresStateError{state: "42501", message: databaseSecret})
	repository := newPostgresOwnerRepository(sqlOwnerQueryer{db: database}, []OwnerTable{
		{Table: "listing_store", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
		{Table: "listing_category", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	})

	_, err := repository.List(context.Background())
	if err == nil {
		t.Fatal("List error = nil, want database failure")
	}
	if strings.Contains(err.Error(), databaseSecret) {
		t.Fatalf("error leaks database details: %q", err)
	}
}

func TestPostgresOwnerRepositoryPropagatesQueryCancellation(t *testing.T) {
	t.Parallel()

	database, mock := openOwnerRepositorySQLMock(t, sqlmock.QueryMatcherEqual)
	mock.ExpectQuery(listingTasksAggregateQuery).
		WillDelayFor(200 * time.Millisecond).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "user_id", "row_count"}).AddRow("tenant-a", "subject-a", int64(1)))
	repository := newPostgresOwnerRepository(sqlOwnerQueryer{db: database}, []OwnerTable{
		{Table: "listing_kit_tasks", TenantColumn: "tenant_id", UserColumn: "user_id"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := repository.List(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("List error = %v, want context deadline exceeded", err)
	}
}

func TestPostgresOwnerRepositoryCancellationWinsOverUndefinedTable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	repository := newPostgresOwnerRepository(cancelingErrorOwnerQueryer{
		cancel: cancel,
		err:    fmt.Errorf("wrapped database error: %w", postgresStateError{state: "42P01", message: "relation does not exist"}),
	}, []OwnerTable{
		{Table: "optional_table", TenantColumn: "tenant_id", UserColumn: "user_id"},
	})

	owners, err := repository.List(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("List owners, error = %#v, %v; want context cancellation", owners, err)
	}
}

func TestPostgresOwnerRepositoryRejectsUnsafeInventoryBeforeQuerying(t *testing.T) {
	t.Parallel()

	const unsafeIdentifier = "listing_store; DELETE FROM listing_store"
	database, _ := openOwnerRepositorySQLMock(t, sqlmock.QueryMatcherEqual)
	repository := newPostgresOwnerRepository(sqlOwnerQueryer{db: database}, []OwnerTable{
		{Table: unsafeIdentifier, TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	})

	_, err := repository.List(context.Background())
	if err == nil {
		t.Fatal("List error = nil, want invalid inventory rejection")
	}
	if strings.Contains(err.Error(), unsafeIdentifier) {
		t.Fatalf("error repeats unsafe identifier: %q", err)
	}
}

func openOwnerRepositorySQLMock(t *testing.T, matcher sqlmock.QueryMatcher) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() {
		mock.ExpectClose()
		if err := database.Close(); err != nil {
			t.Errorf("close SQL mock: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
	})
	return database, mock
}

type cancelingErrorOwnerQueryer struct {
	cancel context.CancelFunc
	err    error
}

func (queryer cancelingErrorOwnerQueryer) QueryContext(context.Context, string) (ownerRows, error) {
	queryer.cancel()
	return nil, queryer.err
}

func matchReadOnlyInventoryQuery(expectedTable, actualQuery string) error {
	if !strings.HasPrefix(actualQuery, "SELECT ") {
		return errors.New("identity preflight repository issued a non-SELECT statement")
	}
	if !strings.Contains(actualQuery, "\nFROM "+expectedTable+"\n") {
		return errors.New("identity preflight repository queried the wrong inventory table")
	}
	upper := strings.ToUpper(actualQuery)
	for _, forbidden := range []string{"INSERT ", "UPDATE ", "DELETE ", "ALTER ", "CREATE ", "DROP ", "BEGIN", "COMMIT", "ROLLBACK"} {
		if strings.Contains(upper, forbidden) {
			return errors.New("identity preflight repository issued a write, migration, or transaction statement")
		}
	}
	return nil
}

type postgresStateError struct {
	state   string
	message string
}

func (err postgresStateError) Error() string {
	return err.message
}

func (err postgresStateError) SQLState() string {
	return err.state
}
