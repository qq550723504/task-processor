package productsourcing

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/product/sourcing"
)

type acquisitionReadDeadlinePublisher struct{ sourcing.AcquisitionPublisher }

func (p acquisitionReadDeadlinePublisher) Read(context.Context, string) (sourcing.PersistedPublication, error) {
	return sourcing.PersistedPublication{}, context.DeadlineExceeded
}

func TestAcquisitionPostPublicationReadDeadlineRemainsUnknown(t *testing.T) {
	db := acquisitionDatabase(t)
	require.NoError(t, InstallAcquisitionSchema(db))
	provider, fetches, _ := acquisitionFixture(t)
	permissions, live := acquisitionPermissionDependencies(t)
	service, err := NewPublicAcquisition(context.Background(), db, live, permissions, provider)
	require.NoError(t, err)
	service.publisher = acquisitionReadDeadlinePublisher{service.publisher}
	ctx, key := acquisitionIdentity("org-read-deadline", "actor"), uuid.NewString()
	_, err = service.Acquire(ctx, key, "981645030344")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
	var versions int64
	require.NoError(t, db.Table("product_snapshot_versions").Count(&versions).Error)
	require.EqualValues(t, 1, versions)
	rebuilt, err := NewPublicAcquisition(context.Background(), db, live, permissions, provider)
	require.NoError(t, err)
	result, err := rebuilt.Verify(ctx, key, "981645030344")
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Publication.Receipt.CatalogVersion)
	require.EqualValues(t, 1, fetches.Load())
}

// The real PostgreSQL COMMIT succeeds; only its acknowledgement is lost at the
// database/sql boundary. Unlike a seeded receipt, this exercises actual writes.
type acquisitionAckDriver struct {
	target string
	lost   atomic.Bool
}

func (d *acquisitionAckDriver) Open(dsn string) (driver.Conn, error) {
	c, e := stdlib.GetDefaultDriver().Open(dsn)
	if e != nil {
		return nil, e
	}
	return &acquisitionAckConn{Conn: c, owner: d}, nil
}

type acquisitionAckConn struct {
	driver.Conn
	owner *acquisitionAckDriver
	stage string
}

func (c *acquisitionAckConn) Prepare(q string) (driver.Stmt, error) {
	s, e := c.Conn.Prepare(q)
	if e != nil {
		return nil, e
	}
	return &acquisitionAckStmt{Stmt: s, conn: c, query: q}, nil
}
func (c *acquisitionAckConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		s, e := p.PrepareContext(ctx, q)
		if e != nil {
			return nil, e
		}
		return &acquisitionAckStmt{Stmt: s, conn: c, query: q}, nil
	}
	return c.Prepare(q)
}
func (c *acquisitionAckConn) Begin() (driver.Tx, error) {
	c.stage = ""
	tx, e := c.Conn.Begin()
	if e != nil {
		return nil, e
	}
	return &acquisitionAckTx{Tx: tx, conn: c}, nil
}
func (c *acquisitionAckConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	c.stage = ""
	tx, e := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
	if e != nil {
		return nil, e
	}
	return &acquisitionAckTx{Tx: tx, conn: c}, nil
}

type acquisitionAckTx struct {
	driver.Tx
	conn *acquisitionAckConn
}

func (t *acquisitionAckTx) Commit() error {
	if e := t.Tx.Commit(); e != nil {
		return e
	}
	if t.conn.stage == t.conn.owner.target && t.conn.owner.lost.CompareAndSwap(false, true) {
		return errors.New("task-injected lost COMMIT acknowledgement")
	}
	return nil
}

type acquisitionAckStmt struct {
	driver.Stmt
	conn  *acquisitionAckConn
	query string
}

func (s *acquisitionAckStmt) mark() {
	q := strings.ToLower(strings.TrimSpace(s.query))
	if strings.Contains(q, "product_acquisition_operations") {
		if strings.HasPrefix(q, "insert") {
			s.conn.stage = "start"
		}
		if strings.HasPrefix(q, "update") {
			switch {
			case strings.Contains(q, "set command="):
				s.conn.stage = "prepare"
			case strings.Contains(q, "state='publishing'"):
				s.conn.stage = "claim"
			case strings.Contains(q, "failure_code="):
				s.conn.stage = "finish"
			}
		}
	} else if strings.HasPrefix(q, "insert") && strings.Contains(q, "product_source_publications") {
		s.conn.stage = "publication"
	}
}
func (s *acquisitionAckStmt) Exec(v []driver.Value) (driver.Result, error) {
	s.mark()
	return s.Stmt.Exec(v)
}
func (s *acquisitionAckStmt) Query(v []driver.Value) (driver.Rows, error) {
	s.mark()
	return s.Stmt.Query(v)
}
func (s *acquisitionAckStmt) ExecContext(ctx context.Context, v []driver.NamedValue) (driver.Result, error) {
	s.mark()
	return s.Stmt.(driver.StmtExecContext).ExecContext(ctx, v)
}
func (s *acquisitionAckStmt) QueryContext(ctx context.Context, v []driver.NamedValue) (driver.Rows, error) {
	s.mark()
	return s.Stmt.(driver.StmtQueryContext).QueryContext(ctx, v)
}

func TestAcquisitionActualCommitAcknowledgementLossAtEveryPersistenceBoundary(t *testing.T) {
	for _, stage := range []string{"start", "prepare", "claim", "publication", "finish"} {
		t.Run(stage, func(t *testing.T) {
			owner := acquisitionDatabase(t)
			require.NoError(t, InstallAcquisitionSchema(owner))
			var name string
			require.NoError(t, owner.Raw("SELECT current_database()").Scan(&name).Error)
			fault := &acquisitionAckDriver{target: stage}
			driverName := "issue398_" + uuid.NewString()
			sql.Register(driverName, fault)
			pool, err := sql.Open(driverName, os.Getenv("ISSUE398_TEST_DSN")+" dbname="+name)
			require.NoError(t, err)
			pool.SetMaxOpenConns(8)
			t.Cleanup(func() { require.NoError(t, pool.Close()) })
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			provider, fetches, _ := acquisitionFixture(t)
			permissions, live := acquisitionPermissionDependencies(t)
			service, err := NewPublicAcquisition(context.Background(), db, live, permissions, provider)
			require.NoError(t, err)
			ctx := acquisitionIdentity("org-ack", "actor")
			key := uuid.NewString()
			_, err = service.Acquire(ctx, key, "981645030344")
			require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
			require.True(t, fault.lost.Load(), "test must cross and lose the targeted real COMMIT")
			// Restart without fault injection: persisted state alone drives recovery.
			rebuilt, err := NewPublicAcquisition(context.Background(), owner, live, permissions, provider)
			require.NoError(t, err)
			op, err := rebuilt.operations.ByKey(ctx, sourcing.PublicationScope{OrganizationID: "org-ack", ActorID: "actor"}, key)
			require.NoError(t, err)
			before := fetches.Load()
			var versions int64
			require.NoError(t, owner.Table("product_snapshot_versions").Count(&versions).Error)
			switch stage {
			case "start":
				require.Equal(t, sourcing.AcquisitionAcquiring, op.State)
				require.Zero(t, before)
				require.Zero(t, versions)
				_, err = rebuilt.Verify(ctx, key, "981645030344")
				require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
				// Only this pre-prepare state may acquire a new fenced GET lease.
				require.NoError(t, owner.Exec("UPDATE product_acquisition_operations SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=?", op.ID).Error)
				result, e := rebuilt.Acquire(ctx, key, "981645030344")
				require.NoError(t, e)
				require.EqualValues(t, 1, result.Publication.Receipt.CatalogVersion)
			case "prepare":
				require.Equal(t, sourcing.AcquisitionPrepared, op.State)
				require.Zero(t, versions)
				_, err = rebuilt.Verify(ctx, key, "981645030344")
				require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
				result, e := rebuilt.Acquire(ctx, key, "981645030344")
				require.NoError(t, e)
				require.Equal(t, op.CommandHash, result.Operation.CommandHash)
				require.Equal(t, before, fetches.Load())
			case "claim":
				require.Equal(t, sourcing.AcquisitionPublishing, op.State)
				require.Zero(t, versions)
				_, err = rebuilt.Verify(ctx, key, "981645030344")
				require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
				_, err = rebuilt.Acquire(ctx, key, "981645030344")
				require.ErrorIs(t, err, sourcing.ErrAcquisitionUnknown)
				require.NoError(t, owner.Table("product_snapshot_versions").Count(&versions).Error)
				require.Zero(t, versions)
				require.Equal(t, before, fetches.Load())
			case "publication", "finish":
				require.EqualValues(t, 1, versions)
				result, e := rebuilt.Verify(ctx, key, "981645030344")
				require.NoError(t, e)
				require.EqualValues(t, 1, result.Publication.Receipt.CatalogVersion)
				require.Equal(t, op.CommandHash, result.Operation.CommandHash)
				require.Equal(t, before, fetches.Load())
			}
		})
	}
}
