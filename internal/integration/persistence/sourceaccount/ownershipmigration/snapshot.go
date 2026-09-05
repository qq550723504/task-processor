package ownershipmigration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ReadSnapshot reads the two explicitly selected databases independently. It
// never discovers a metadata database, resolves a request or mutates a table.
func ReadSnapshot(ctx context.Context, source, metadata *sql.DB, sourceID, metadataID string) (Snapshot, error) {
	if source == nil || metadata == nil || strings.TrimSpace(sourceID) == "" || strings.TrimSpace(metadataID) == "" {
		return Snapshot{}, fmt.Errorf("two databases and explicit source identities required")
	}
	var s Snapshot
	obs, err := readTransaction(ctx, source, sourceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT id, tenant_id, platform, profile_ref, status, deleted FROM source_account WHERE LOWER(platform) = '1688' ORDER BY id LIMIT $1", MaxRows+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a LegacyAccount
			if err := rows.Scan(&a.ID, &a.TenantID, &a.Platform, &a.ProfileRef, &a.Status, &a.Deleted); err != nil {
				return err
			}
			s.Accounts = append(s.Accounts, a)
			if len(s.Accounts) > MaxRows {
				return fmt.Errorf("account row limit exceeded")
			}
		}
		return rows.Err()
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("source account snapshot failed; check connection, schema, permissions and row limit")
	}
	s.AccountObservation = obs
	obs, err = readTransaction(ctx, metadata, metadataID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT org_id, value, sequence, owner_removed FROM projections.org_metadata2 WHERE key = 'yudao_tenant_id' ORDER BY org_id, sequence LIMIT $1", MaxRows+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m OrganizationMetadata
			if err := rows.Scan(&m.OrganizationID, &m.Value, &m.Sequence, &m.OwnerRemoved); err != nil {
				return err
			}
			s.Metadata = append(s.Metadata, m)
			if len(s.Metadata) > MaxRows {
				return fmt.Errorf("metadata row limit exceeded")
			}
		}
		return rows.Err()
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("Organization metadata snapshot failed; check explicit authority database, schema, permissions and row limit")
	}
	s.MetadataObservation = obs
	return s, nil
}

func readTransaction(ctx context.Context, db *sql.DB, sourceID string, read func(*sql.Tx) error) (Observation, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Observation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// Set before any read; explicit SQL is also testable with the repository's
	// existing sqlmock version, which does not expose BeginTx option matching.
	if _, err = tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
		return Observation{}, err
	}
	obs := Observation{SourceID: sourceID}
	if err = tx.QueryRowContext(ctx, "SELECT current_database(), transaction_timestamp()").Scan(&obs.Database, &obs.At); err != nil {
		return Observation{}, err
	}
	if err = read(tx); err != nil {
		return Observation{}, err
	}
	if err = tx.Commit(); err != nil {
		return Observation{}, err
	}
	return obs, nil
}
