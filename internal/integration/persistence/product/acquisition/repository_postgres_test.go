package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/product/sourcing"
)

func isolatedDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ISSUE398_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: ISSUE398_TEST_DSN must target task-owned PostgreSQL")
	}
	require.Contains(t, dsn, "host=127.0.0.1")
	require.Contains(t, dsn, "user=issue398_owner")
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	rootSQL, err := root.DB()
	require.NoError(t, err)
	rootSQL.SetMaxOpenConns(2)
	name := "issue398_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.Exec("CREATE DATABASE "+name).Error)
	db, err := gorm.Open(postgres.Open(dsn+" dbname="+name), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() {
		require.NoError(t, pool.Close())
		require.NoError(t, root.Exec("DROP DATABASE "+name+" WITH (FORCE)").Error)
		require.NoError(t, rootSQL.Close())
	})
	return db
}

func TestAcquisitionSchemaDriftFailsClosed(t *testing.T) {
	for _, change := range []string{
		"ALTER TABLE product_acquisition_operations DROP CONSTRAINT product_acquisition_operations_pkey",
		"ALTER TABLE product_acquisition_operations DROP CONSTRAINT acq_command_bound; ALTER TABLE product_acquisition_operations ADD CONSTRAINT acq_command_bound CHECK(true)",
		"ALTER TABLE product_acquisition_operations ALTER COLUMN fingerprint TYPE text",
	} {
		t.Run(change, func(t *testing.T) {
			db := isolatedDatabase(t)
			require.NoError(t, InstallSchema(db))
			require.NoError(t, db.Exec(change).Error)
			_, err := NewRepository(context.Background(), db)
			require.Error(t, err, "drifted schema must not admit a writer")
		})
	}
}

func requestedOperation(t *testing.T, org, actor, key string) sourcing.AcquisitionOperation {
	t.Helper()
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	identity, _ := json.Marshal([]string{org, actor, key})
	input, _ := json.Marshal([]string{sourcing.AcquisitionContractVersion, "acquire", source.URL})
	sum := sha256.Sum256(input)
	return sourcing.AcquisitionOperation{Scope: sourcing.PublicationScope{OrganizationID: org, ActorID: actor}, Key: key, ID: uuid.NewSHA1(uuid.NameSpaceURL, identity).String(), Source: source, Fingerprint: hex.EncodeToString(sum[:])}
}

func originalCommand(t *testing.T, op sourcing.AcquisitionOperation) sourcing.PublicationCommand {
	t.Helper()
	title := "actual fixture title"
	envelope, err := sourcing.MapAcquisitionEvidence(op.Source, sourcing.AcquisitionEvidence{SchemaVersion: 1, SourceURL: op.Source.URL, OfferID: op.Source.OfferID, Title: &title, CapturedAt: time.Now().UTC(), ContentSHA256: strings.Repeat("a", 64), ParserVersion: "fixture/v1"}, "public_http", op.ID)
	require.NoError(t, err)
	key, id, err := sourcing.PublicationIdentity(envelope)
	require.NoError(t, err)
	base := uint64(0)
	return sourcing.PublicationCommand{PublicationID: id, Producer: sourcing.ProducerDescriptor{Kind: sourcing.AcquisitionProducerKind, Version: "v1"}, ProductKey: key, ExpectedBaseVersion: &base, Envelope: envelope}
}

func TestAcquisitionPostgresFrozenCommandFenceAndUniquePublish(t *testing.T) {
	db := isolatedDatabase(t)
	ctx := context.Background()
	_, err := NewRepository(ctx, db)
	require.Error(t, err, "construction must not install schema")
	require.NoError(t, InstallSchema(db))
	r, err := NewRepository(ctx, db)
	require.NoError(t, err)
	req := requestedOperation(t, "org-a", "actor-a", uuid.NewString())
	old, fetch, err := r.Start(ctx, req)
	require.NoError(t, err)
	require.True(t, fetch)
	require.EqualValues(t, 1, old.Fence)
	replay, fetch, err := r.Start(ctx, req)
	require.NoError(t, err)
	require.False(t, fetch)
	require.Equal(t, old.ID, replay.ID)
	// Only an unprepared expired acquisition may be re-fetched.
	require.NoError(t, db.Exec("UPDATE public.product_acquisition_operations SET lease_until=clock_timestamp()-interval '1 second' WHERE operation_id=?", old.ID).Error)
	current, fetch, err := r.Start(ctx, req)
	require.NoError(t, err)
	require.True(t, fetch)
	require.EqualValues(t, 2, current.Fence)
	_, err = r.Prepare(ctx, old, originalCommand(t, old))
	require.ErrorIs(t, err, sourcing.ErrAcquisitionFence)
	command := originalCommand(t, current)
	prepared, err := r.Prepare(ctx, current, command)
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionPrepared, prepared.State)
	var claims atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, claimed, e := r.Claim(ctx, prepared)
			if e != nil {
				t.Error(e)
			}
			if claimed {
				claims.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, claims.Load())
	rebuilt, err := NewRepository(ctx, db)
	require.NoError(t, err)
	stored, err := rebuilt.ByKey(ctx, req.Scope, req.Key)
	require.NoError(t, err)
	require.Equal(t, sourcing.AcquisitionPublishing, stored.State)
	require.Equal(t, prepared.CommandHash, stored.CommandHash)
	require.Equal(t, command, *stored.Command)
	_, fetch, err = rebuilt.Start(ctx, req)
	require.NoError(t, err)
	require.False(t, fetch)
	_, claimed, err := rebuilt.Claim(ctx, stored)
	require.NoError(t, err)
	require.False(t, claimed)
	_, err = rebuilt.ByID(ctx, sourcing.PublicationScope{OrganizationID: "org-a", ActorID: "actor-b"}, req.ID)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionNotFound)
	_, err = rebuilt.ByKey(ctx, sourcing.PublicationScope{OrganizationID: "org-b", ActorID: "actor-a"}, req.Key)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionNotFound)
	require.NoError(t, rebuilt.Finish(ctx, stored, sourcing.AcquisitionPublished, ""))
	_, err = rebuilt.Prepare(ctx, current, command)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionFence)
	// Read detects command tampering; staging is not accepted as published evidence.
	require.NoError(t, db.Exec("UPDATE public.product_acquisition_operations SET command_hash=? WHERE operation_id=?", strings.Repeat("b", 64), req.ID).Error)
	_, err = rebuilt.ByID(ctx, req.Scope, req.ID)
	require.ErrorIs(t, err, sourcing.ErrAcquisitionUnavailable)
}

func TestAcquisitionPostgresCapacityIncludesPreparedAndPublishing(t *testing.T) {
	db := isolatedDatabase(t)
	ctx := context.Background()
	require.NoError(t, InstallSchema(db))
	r, err := NewRepository(ctx, db)
	require.NoError(t, err)
	var first sourcing.AcquisitionOperation
	for i := 0; i < sourcing.MaxActiveAcquisitionOperations; i++ {
		op, claim, e := r.Start(ctx, requestedOperation(t, "org-cap", "actor", uuid.NewString()))
		require.NoError(t, e)
		require.True(t, claim)
		if i == 0 {
			first = op
			prepared, e := r.Prepare(ctx, op, originalCommand(t, op))
			require.NoError(t, e)
			_, _, e = r.Claim(ctx, prepared)
			require.NoError(t, e)
		}
	}
	_, _, err = r.Start(ctx, requestedOperation(t, "org-cap", "actor", uuid.NewString()))
	require.ErrorIs(t, err, sourcing.ErrAcquisitionCapacity)
	replay, claim, err := r.Start(ctx, first)
	require.NoError(t, err)
	require.False(t, claim)
	require.Equal(t, first.ID, replay.ID)
	// All terminal keys remain reserved, independently of active capacity.
	require.NoError(t, db.Exec("UPDATE public.product_acquisition_operations SET state='failed',failure_code='SOURCE_UNAVAILABLE' WHERE state='acquiring'").Error)
	for i := sourcing.MaxActiveAcquisitionOperations; i < sourcing.MaxAcquisitionOperations; i++ {
		op, _, e := r.Start(ctx, requestedOperation(t, "org-cap", "actor", uuid.NewString()))
		require.NoError(t, e)
		require.NoError(t, r.Finish(ctx, op, sourcing.AcquisitionFailed, "SOURCE_UNAVAILABLE"))
	}
	_, _, err = r.Start(ctx, requestedOperation(t, "org-cap", "actor", uuid.NewString()))
	require.ErrorIs(t, err, sourcing.ErrAcquisitionCapacity)
	_, claim, err = r.Start(ctx, first)
	require.NoError(t, err)
	require.False(t, claim)
}
