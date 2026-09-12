package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/product/sourcing"
)

func browserStagingDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ISSUE399_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: ISSUE399_TEST_DSN must target task-owned PostgreSQL")
	}
	require.Contains(t, dsn, "host=127.0.0.1")
	require.Contains(t, dsn, "user=issue399_owner")
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := root.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(2)
	name := "issue399_staging_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE DATABASE "+name).Error)
	db, err := gorm.Open(postgres.Open(dsn+" dbname="+name), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connections, err := db.DB()
	require.NoError(t, err)
	connections.SetMaxOpenConns(8)
	t.Cleanup(func() {
		require.NoError(t, connections.Close())
		require.NoError(t, root.Exec("DROP DATABASE "+name+" WITH (FORCE)").Error)
		require.NoError(t, pool.Close())
	})
	require.NoError(t, InstallSchema(db))
	return db
}

func TestBrowserCapturePostgresDurableIntentAndCrossActionConflict(t *testing.T) {
	db := browserStagingDatabase(t)
	ctx := context.Background()
	repo, err := NewRepository(ctx, db)
	require.NoError(t, err)
	request, command := browserStagingOperation(t)
	op, claimed, err := repo.Start(ctx, request)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, request.CaptureSHA256, op.CaptureSHA256)
	restarted, err := NewRepository(ctx, db)
	require.NoError(t, err)
	read, err := restarted.ByKey(ctx, op.Scope, op.Key)
	require.NoError(t, err)
	require.Equal(t, request.CaptureSHA256, read.CaptureSHA256)
	require.Nil(t, read.Command)
	changed := request
	changed.CaptureSHA256 = strings.Repeat("a", 64)
	changed.Fingerprint, err = sourcing.BrowserAcquisitionFingerprint(changed.Source, changed.CaptureSHA256)
	require.NoError(t, err)
	_, claim, err := restarted.Start(ctx, changed)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	require.False(t, claim)
	public := request
	public.CaptureSHA256 = ""
	input, _ := json.Marshal([]string{sourcing.AcquisitionContractVersion, "acquire", public.Source.URL})
	sum := sha256.Sum256(input)
	public.Fingerprint = hex.EncodeToString(sum[:])
	_, claim, err = restarted.Start(ctx, public)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	require.False(t, claim)
	prepared, err := repo.Prepare(ctx, op, command)
	require.NoError(t, err)
	require.Equal(t, request.CaptureSHA256, prepared.CaptureSHA256)
	require.Equal(t, sourcing.AcquisitionPrepared, prepared.State)
	var count int64
	require.NoError(t, db.Table("product_acquisition_operations").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestBrowserCapturePostgresPrepareChecksLockedIntent(t *testing.T) {
	db := browserStagingDatabase(t)
	ctx := context.Background()
	repo, err := NewRepository(ctx, db)
	require.NoError(t, err)
	request, command := browserStagingOperation(t)
	op, claimed, err := repo.Start(ctx, request)
	require.NoError(t, err)
	require.True(t, claimed)
	forged := op
	forged.CaptureSHA256 = strings.Repeat("b", 64)
	forged.Fingerprint, err = sourcing.BrowserAcquisitionFingerprint(forged.Source, forged.CaptureSHA256)
	require.NoError(t, err)
	command.Envelope.RawReference.Metadata["capture_sha256"] = forged.CaptureSHA256
	require.True(t, validCommand(forged, command), "request alone is internally consistent")
	_, err = repo.Prepare(ctx, forged, command)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	current, err := repo.ByKey(ctx, op.Scope, op.Key)
	require.NoError(t, err)
	require.Nil(t, current.Command)
	require.Equal(t, sourcing.AcquisitionAcquiring, current.State)
	require.Equal(t, op.CaptureSHA256, current.CaptureSHA256)
}

func TestBrowserCapturePostgresRejectsCorruption(t *testing.T) {
	for _, mutation := range []string{
		"UPDATE product_acquisition_operations SET capture_sha256=''",
		"UPDATE product_acquisition_operations SET capture_sha256=repeat('a',64)",
		"UPDATE product_acquisition_operations SET fingerprint=repeat('a',64)",
		"UPDATE product_acquisition_operations SET command_hash=repeat('a',64)",
	} {
		t.Run(mutation, func(t *testing.T) {
			db := browserStagingDatabase(t)
			ctx := context.Background()
			repo, err := NewRepository(ctx, db)
			require.NoError(t, err)
			request, command := browserStagingOperation(t)
			op, _, err := repo.Start(ctx, request)
			require.NoError(t, err)
			_, err = repo.Prepare(ctx, op, command)
			require.NoError(t, err)
			require.NoError(t, db.Exec(mutation).Error)
			_, err = repo.ByKey(ctx, op.Scope, op.Key)
			require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
		})
	}
}

func TestBrowserCapturePostgresRejectsWeakSchema(t *testing.T) {
	for _, ddl := range []string{
		"ALTER TABLE product_acquisition_operations DROP CONSTRAINT acq_capture_digest; ALTER TABLE product_acquisition_operations ADD CONSTRAINT acq_capture_digest CHECK(length(capture_sha256)<=64)",
		"ALTER TABLE product_acquisition_operations ALTER COLUMN capture_sha256 DROP NOT NULL",
		"ALTER TABLE product_acquisition_operations ALTER COLUMN capture_sha256 TYPE varchar(128)",
		"ALTER TABLE product_acquisition_operations ALTER COLUMN capture_sha256 DROP DEFAULT",
		"ALTER TABLE product_acquisition_operations ALTER COLUMN capture_sha256 SET DEFAULT 'unexpected'",
	} {
		t.Run(ddl, func(t *testing.T) {
			db := browserStagingDatabase(t)
			require.NoError(t, db.Exec(ddl).Error) // Fault injection in this test's empty, owned database only.
			_, err := NewRepository(context.Background(), db)
			require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
		})
	}
}
