package ownershipmigration

import (
	"context"

	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReadSnapshotUsesSeparateReadOnlyTransactions(t *testing.T) {
	source, sm, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	metadata, mm, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	at := time.Now().UTC()
	for _, m := range []sqlmock.Sqlmock{sm, mm} {
		m.ExpectBegin()
		m.ExpectExec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY").WillReturnResult(sqlmock.NewResult(0, 0))
		m.ExpectQuery("SELECT current_database").WillReturnRows(sqlmock.NewRows([]string{"db", "at"}).AddRow("database", at))
	}
	sm.ExpectQuery("SELECT id, tenant_id, platform, profile_ref, status, deleted FROM source_account").WithArgs(MaxRows + 1).WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "platform", "profile_ref", "status", "deleted"}).AddRow(7, 101, "1688", "opaque", 1, 1))
	sm.ExpectCommit()
	mm.ExpectQuery("SELECT org_id, value, sequence, owner_removed FROM projections.org_metadata2").WithArgs(MaxRows + 1).WillReturnRows(sqlmock.NewRows([]string{"org_id", "value", "sequence", "owner_removed"}).AddRow("org-A", []byte("101"), 42, false))
	mm.ExpectCommit()
	s, err := ReadSnapshot(context.Background(), source, metadata, "business-prod", "identity-prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Accounts) != 1 || s.Accounts[0].Deleted != 1 || len(s.Metadata) != 1 || s.AccountObservation.SourceID != "business-prod" || s.MetadataObservation.SourceID != "identity-prod" || !s.AccountObservation.At.Equal(at) {
		t.Fatalf("wrong snapshot: %+v", s)
	}
	if err := sm.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := mm.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReadSnapshotFailureRollsBackWithoutPartialEvidence(t *testing.T) {
	for _, stage := range []string{"query", "scan", "commit"} {
		t.Run(stage, func(t *testing.T) {
			db, m, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m.ExpectBegin()
			m.ExpectExec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY").WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery("SELECT current_database").WillReturnRows(sqlmock.NewRows([]string{"db", "at"}).AddRow("business", time.Now()))
			q := m.ExpectQuery("SELECT id, tenant_id").WithArgs(MaxRows + 1)
			switch stage {
			case "query":
				q.WillReturnError(fmt.Errorf("sensitive connection detail"))
				m.ExpectRollback()
			case "scan":
				q.WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "platform", "profile_ref", "status", "deleted"}).AddRow("bad", 101, "1688", "ref", 0, 0))
				m.ExpectRollback()
			case "commit":
				q.WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "platform", "profile_ref", "status", "deleted"}))
				m.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))
			}
			s, err := ReadSnapshot(context.Background(), db, db, "business", "identity")
			if err == nil || len(s.Accounts) != 0 || err.Error() == "sensitive connection detail" {
				t.Fatalf("failure returned partial evidence: %+v %v", s, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReadSnapshotRequiresExplicitSources(t *testing.T) {
	if _, err := ReadSnapshot(context.Background(), nil, nil, "", ""); err == nil {
		t.Fatal("missing source accepted")
	}
}
