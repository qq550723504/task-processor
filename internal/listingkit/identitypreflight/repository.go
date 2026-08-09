package identitypreflight

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
)

const postgresUndefinedTableSQLState = "42P01"

var sqlIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type PersistedOwner struct {
	Table    string
	TenantID string
	UserID   string
	RowCount int64
}

type OwnerRepository interface {
	List(ctx context.Context) ([]PersistedOwner, error)
}

type ownerRows interface {
	Next() bool
	Scan(destinations ...any) error
	Err() error
	Close() error
}

type ownerQueryer interface {
	QueryContext(ctx context.Context, query string) (ownerRows, error)
}

type sqlOwnerQueryer struct {
	db *sql.DB
}

func (queryer sqlOwnerQueryer) QueryContext(ctx context.Context, query string) (ownerRows, error) {
	if queryer.db == nil {
		return nil, errors.New("identity preflight database is unavailable")
	}
	return queryer.db.QueryContext(ctx, query)
}

type postgresOwnerRepository struct {
	database  ownerQueryer
	inventory []OwnerTable
}

// NewPostgresOwnerRepository constructs the read-only persisted-owner repository.
func NewPostgresOwnerRepository(db *sql.DB) OwnerRepository {
	return newPostgresOwnerRepository(sqlOwnerQueryer{db: db}, ownerTableInventory[:])
}

func newPostgresOwnerRepository(database ownerQueryer, inventory []OwnerTable) OwnerRepository {
	return &postgresOwnerRepository{
		database:  database,
		inventory: append([]OwnerTable(nil), inventory...),
	}
}

func (repository *postgresOwnerRepository) List(ctx context.Context) ([]PersistedOwner, error) {
	if err := validateOwnerTableInventory(repository.inventory); err != nil {
		return nil, err
	}
	owners := make([]PersistedOwner, 0)
	for _, table := range repository.inventory {
		rows, err := repository.database.QueryContext(ctx, ownerAggregateQuery(table))
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, fmt.Errorf("list persisted owners from %s: %w", table.Table, ctxErr)
			}
			if isPostgresUndefinedTable(err) {
				continue
			}
			return nil, fmt.Errorf("list persisted owners from %s: database query failed", table.Table)
		}
		tableOwners, err := scanOwnerRows(ctx, table.Table, rows)
		if err != nil {
			return nil, err
		}
		owners = append(owners, tableOwners...)
	}
	return owners, nil
}

func validateOwnerTableInventory(inventory []OwnerTable) error {
	for _, table := range inventory {
		if !sqlIdentifierPattern.MatchString(table.Table) ||
			!sqlIdentifierPattern.MatchString(table.TenantColumn) ||
			!sqlIdentifierPattern.MatchString(table.UserColumn) {
			return errors.New("identity preflight inventory contains an invalid SQL identifier")
		}
	}
	return nil
}

func ownerAggregateQuery(table OwnerTable) string {
	return fmt.Sprintf(`SELECT CAST(%s AS text) AS tenant_id,
       CAST(%s AS text) AS user_id,
       COUNT(*) AS row_count
FROM %s
WHERE NULLIF(BTRIM(CAST(%s AS text)), '') IS NOT NULL
  AND NULLIF(BTRIM(CAST(%s AS text)), '') IS NOT NULL
GROUP BY %s, %s`,
		table.TenantColumn,
		table.UserColumn,
		table.Table,
		table.TenantColumn,
		table.UserColumn,
		table.TenantColumn,
		table.UserColumn,
	)
}

func scanOwnerRows(ctx context.Context, table string, rows ownerRows) (owners []PersistedOwner, resultErr error) {
	defer func() {
		if err := rows.Close(); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("list persisted owners from %s: close aggregate rows failed", table)
		}
	}()

	for rows.Next() {
		owner := PersistedOwner{Table: table}
		if err := rows.Scan(&owner.TenantID, &owner.UserID, &owner.RowCount); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, fmt.Errorf("list persisted owners from %s: %w", table, ctxErr)
			}
			return nil, fmt.Errorf("list persisted owners from %s: scan aggregate row failed", table)
		}
		owners = append(owners, owner)
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("list persisted owners from %s: %w", table, ctxErr)
		}
		return nil, fmt.Errorf("list persisted owners from %s: iterate aggregate rows failed", table)
	}
	return owners, nil
}

type sqlStateError interface {
	SQLState() string
}

func isPostgresUndefinedTable(err error) bool {
	var stateError sqlStateError
	return errors.As(err, &stateError) && stateError.SQLState() == postgresUndefinedTableSQLState
}
