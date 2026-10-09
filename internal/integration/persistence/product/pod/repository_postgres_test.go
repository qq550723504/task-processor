package podpersistence

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"testing"
	"time"
)

func storedPlan() pod.Plan {
	return pod.Plan{OperationID: uuid.NewString(), Scope: collection.Scope{"org-a", "actor-a", "member-a"}, Binding: pod.AccountBinding{"binding", "revision", "36811", pod.ProtocolRevision}, Template: pod.TemplateManifest{ParentID: "95146", VariantID: "95147", PrototypeID: "730897612975054849", GroupID: "15261", Type: "FREE", Layers: []pod.EditableLayer{{ID: "730897620537384960", Width: 567, Height: 850, PrintWidth: 1334, PrintHeight: 2000}}, RenderFiles: []pod.RenderFile{{ID: "753068348962332672", Thumbnail: "http://e.sdspod.com/builds?content=test"}}}, TemplateItem: pod.InputReference{ItemID: uuid.NewString(), Revision: 1, Source: collection.Source{ProductKey: "template", PublicationID: uuid.NewString(), Version: 1, Kind: "sds_template"}}, Artwork: pod.ArtworkReference{Input: pod.InputReference{ItemID: uuid.NewString(), Revision: 1, Source: collection.Source{ProductKey: "artwork", PublicationID: uuid.NewString(), Version: 1, Kind: "own"}}, Version: 1, ActionID: uuid.NewString(), AssetID: "approved", Hash: collection.Digest("image"), Bytes: 100, Width: 100, Height: 100, MediaType: "image/png"}, Transforms: []pod.Transform{{LayerID: "730897620537384960", X: .5, Y: .5, Scale: 1}}, Name: "测试成品"}
}
func TestPostgresMerchantFenceSurvivesCredentialRotationAndScopedReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, e := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("pod622"), tcpostgres.WithUsername("pod_owner"), tcpostgres.WithPassword("isolated-pod-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, e)
	pool, e := db.DB()
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, InstallSchema(db))
	require.NoError(t, submissionstore.InstallSchema(db))
	r, e := NewRepository(ctx, db)
	require.NoError(t, e)
	guard := func(context.Context, *gorm.DB, pod.Plan) error { return nil }
	p := storedPlan()
	key := uuid.NewString()
	hash := collection.Digest("request")
	operation, e := r.Begin(ctx, key, hash, p, guard)
	require.NoError(t, e)
	require.Equal(t, p.OperationID, operation.Plan.OperationID)
	duplicate, e := r.Begin(ctx, key, hash, p, guard)
	require.NoError(t, e)
	require.Equal(t, p.OperationID, duplicate.Plan.OperationID)
	_, e = r.Begin(ctx, key, collection.Digest("different request"), p, guard)
	require.ErrorIs(t, e, pod.ErrConflict)
	other := storedPlan()
	other.Scope = collection.Scope{"org-b", "actor-b", "member-b"}
	other.Binding.ID = "different-binding"
	other.Binding.Revision = "rotated"
	_, e = r.Begin(ctx, uuid.NewString(), hash, other, guard)
	require.ErrorIs(t, e, pod.ErrConflict)
	_, e = r.Read(ctx, other.Scope, p.OperationID)
	require.ErrorIs(t, e, pod.ErrNotFound)
	_, e = r.Begin(ctx, uuid.NewString(), hash, storedPlan(), func(context.Context, *gorm.DB, pod.Plan) error { return pod.ErrForbidden })
	require.ErrorIs(t, e, pod.ErrConflict)
	var count int64
	require.NoError(t, db.Table("product_pod_operations").Count(&count).Error)
	require.EqualValues(t, 1, count)
	// Elapsed time is never a fence-release condition.
	require.NoError(t, db.Exec("UPDATE product_pod_operations SET created_at=created_at-interval '7 days'").Error)
	_, e = r.Begin(ctx, uuid.NewString(), hash, other, guard)
	require.ErrorIs(t, e, pod.ErrConflict)
	t.Run("step and finished references share kernel transaction", func(t *testing.T) {
		store, e := submissionstore.NewRepository(db)
		require.NoError(t, e)
		kernel, e := submission.NewExecutionKernel(store)
		require.NoError(t, e)
		acquire := func(o pod.Operation, step string) *submission.SendPermit {
			c, e := pod.StepCommand(o, step, "pod-worker")
			require.NoError(t, e)
			a, e := kernel.Acquire(ctx, c)
			require.NoError(t, e)
			require.NotNil(t, a.Permit)
			return a.Permit
		}
		attempt := func(step string) submission.ExecutionAttempt {
			a, e := kernel.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: p.Scope.OrganizationID}, pod.StepIntent(p.OperationID, step))
			require.NoError(t, e)
			return a
		}
		failingUpdate := func() {
			require.NoError(t, db.Exec(`CREATE OR REPLACE FUNCTION pod_test_update_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated POD transaction failure'; END $$`).Error)
			require.NoError(t, db.Exec(`CREATE TRIGGER pod_test_failure BEFORE UPDATE ON product_pod_operations FOR EACH ROW EXECUTE FUNCTION pod_test_update_failure()`).Error)
		}
		allowUpdate := func() { require.NoError(t, db.Exec(`DROP TRIGGER pod_test_failure ON product_pod_operations`).Error) }
		o, e := r.Read(ctx, p.Scope, p.OperationID)
		require.NoError(t, e)
		object := pod.ObjectReceipt{FileCode: "approved.png", Hash: p.Artwork.Hash}
		oss := acquire(o, pod.StepOSS)
		failingUpdate()
		require.Error(t, r.SaveStep(ctx, o, pod.StepOSS, oss, object, guard))
		allowUpdate()
		require.Equal(t, submission.ExecutionClaimed, attempt(pod.StepOSS).Status)
		o, e = r.Read(ctx, p.Scope, p.OperationID)
		require.NoError(t, e)
		require.Nil(t, o.Object)
		command, e := pod.StepCommand(o, pod.StepOSS, "pod-worker")
		require.NoError(t, e)
		replay, e := kernel.Acquire(ctx, command)
		require.NoError(t, e)
		require.Nil(t, replay.Permit)
		require.NoError(t, r.SaveStep(ctx, o, pod.StepOSS, oss, object, guard))
		require.Equal(t, submission.ExecutionSucceeded, attempt(pod.StepOSS).Status)
		o, e = r.Read(ctx, p.Scope, p.OperationID)
		require.NoError(t, e)
		material := pod.MaterialReceipt{ID: "480953643", FileCode: object.FileCode, Hash: object.Hash, Name: pod.MaterialName(p.OperationID), URL: "https://cdn.sdspod.com/images1000Thumbs/test/approved.png?material_id=480953643", Width: 100, Height: 100}
		permit := acquire(o, pod.StepMaterial)
		require.NoError(t, r.SaveStep(ctx, o, pod.StepMaterial, permit, material, guard))
		o, e = r.FreezeSync(ctx, o, guard)
		require.NoError(t, e)
		sync := acquire(o, pod.StepSync)
		_, e = kernel.MarkUnknown(ctx, pod.PermitClaim(p.Scope, sync), submission.UnknownResponseLost)
		require.NoError(t, e)
		qualify := func(i pod.DesignIntent) pod.QualifiedFinished {
			obs := pod.Observation{Finished: pod.FinishedRecord{ID: "962657110282047488", KeyID: "retained", MerchantID: p.Binding.MerchantID, TaskID: "962657107283120129", DesignTaskID: "962657107283120129", ParentID: p.Template.ParentID, VariantID: p.Template.VariantID, PrototypeID: p.Template.PrototypeID, BuildFinish: true, Status: 2, RenderURLs: []string{"https://cdn.sdspod.com/out/36811/test/result.jpg"}}, Design: pod.SavedDesign{FinishedID: "962657110282047488", ParentID: i.ParentID, VariantID: i.VariantID, PrototypeID: i.PrototypeID, GroupID: i.GroupID, Layers: i.Layers, RenderFileIDs: i.RenderFileIDs}, Task: pod.TaskRecord{ID: "962657107283120129", Status: 5, Complete: 1, Success: 1, RenderURLs: []string{"http://cdn.sdspod.com/out/36811/test/result.jpg"}}}
			q, e := pod.QualifyFinished(i, []pod.Observation{obs})
			require.NoError(t, e)
			return q
		}
		altered := *o.Intent
		altered.Layers = append([]pod.DesignLayer(nil), o.Intent.Layers...)
		altered.Layers[0].FabricJSON = strings.Replace(altered.Layers[0].FabricJSON, `"left":300`, `"left":301`, 1)
		require.ErrorIs(t, r.Finish(ctx, o, qualify(altered), guard), pod.ErrUnknown)
		q := qualify(*o.Intent)
		failingUpdate()
		require.Error(t, r.Finish(ctx, o, q, guard))
		allowUpdate()
		require.Equal(t, submission.ExecutionOutcomeUnknown, attempt(pod.StepSync).Status)
		o, e = r.Read(ctx, p.Scope, p.OperationID)
		require.NoError(t, e)
		require.Nil(t, o.Finished)
		require.NoError(t, db.Table("product_pod_fences").Count(&count).Error)
		require.EqualValues(t, 1, count)
		require.NoError(t, r.Finish(ctx, o, q, guard))
		require.NoError(t, r.Finish(ctx, o, q, guard))
		require.Equal(t, submission.ExecutionSucceeded, attempt(pod.StepSync).Status)
		o, e = r.Read(ctx, p.Scope, p.OperationID)
		require.NoError(t, e)
		require.Equal(t, q.Reference(), *o.Finished)
		require.NoError(t, db.Table("product_pod_fences").Count(&count).Error)
		require.Zero(t, count)
		_, e = r.Begin(ctx, uuid.NewString(), hash, other, guard)
		require.NoError(t, e)
	})
}
