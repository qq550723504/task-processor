//go:build integration

package referral

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCleanupPlansBoundRetainedHistory(t *testing.T) {
	owner, runtime := ownedDatabase(t)
	r, err := New(runtime)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	seed := admitted(t, r, "plan-seed", now.Add(-25*time.Hour))
	// Permanent erased history dominates; only forty payloads are due. Future
	// payloads and admission buckets exercise the actual partial/range predicates.
	err = owner.Exec(`INSERT INTO public.registration_intents
 (id,issuer,instance,organization,subject,referrer,key_hash,email_hash,fingerprint,secret_hash,key_id,ciphertext,created_at,create_expires_at,completion_expires_at,lease_until,state)
 SELECT 'plan-'||g,issuer,instance,organization,'plan-sub-'||g,referrer,'plan-key-'||g,'plan-email-'||g,fingerprint,secret_hash,key_id,
 CASE WHEN g<=100000 THEN NULL ELSE ciphertext END,
 created_at+CASE WHEN g>100040 THEN interval '48 hours' ELSE interval '0 hours' END,
 create_expires_at+CASE WHEN g>100040 THEN interval '48 hours' ELSE interval '0 hours' END,
 completion_expires_at+CASE WHEN g>100040 THEN interval '48 hours' ELSE interval '0 hours' END,lease_until,state
 FROM public.registration_intents CROSS JOIN generate_series(1,102040) g WHERE id=?`, seed.ID).Error
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Exec(`INSERT INTO public.registration_admission_buckets(kind,key,window_start,hits)
 SELECT 'ip','plan-'||g,CASE WHEN g<=100040 THEN ?::timestamptz-interval '1 hour' ELSE ?::timestamptz END,1 FROM generate_series(1,102040) g`, now, now.UTC().Truncate(time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err = owner.Exec("ANALYZE public.registration_intents; ANALYZE public.registration_admission_buckets").Error; err != nil {
		t.Fatal(err)
	}
	queries := []struct {
		name, index, sql string
		cutoff           time.Time
	}{
		{"intent", "registration_intents_payload_expiry_idx", `UPDATE public.registration_intents SET ciphertext=NULL WHERE id IN (SELECT id FROM public.registration_intents WHERE completion_expires_at<=? AND ciphertext IS NOT NULL ORDER BY completion_expires_at LIMIT 20 FOR UPDATE SKIP LOCKED)`, now},
		{"bucket", "registration_admission_buckets_expiry_idx", `DELETE FROM public.registration_admission_buckets WHERE ctid IN (SELECT ctid FROM public.registration_admission_buckets WHERE window_start < ? ORDER BY window_start LIMIT 20 FOR UPDATE SKIP LOCKED)`, now.UTC().Truncate(time.Minute)},
	}
	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			// ANALYZE really executes under the existing least-privilege runtime role;
			// rollback keeps identical rows for the production operation below.
			tx := runtime.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			var plan string
			err := tx.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+q.sql, q.cutoff).Scan(&plan).Error
			rollback := tx.Rollback().Error
			if err != nil || rollback != nil {
				t.Fatalf("plan %v rollback %v", err, rollback)
			}
			t.Logf("RAW_EXPLAIN_%s=%s", q.name, plan)
			if !strings.Contains(plan, q.index) {
				t.Errorf("cleanup scans retained history without supporting %s", q.index)
			}
			if strings.Contains(plan, `"Node Type": "Seq Scan"`) {
				t.Error("cleanup uses full sequential scan")
			}
			var before, after int64
			count := func() int64 {
				var n int64
				sql := "SELECT count(*) FROM public.registration_intents WHERE ciphertext IS NOT NULL AND completion_expires_at<=?"
				if q.name == "bucket" {
					sql = "SELECT count(*) FROM public.registration_admission_buckets WHERE window_start<?"
				}
				if e := owner.Raw(sql, q.cutoff).Scan(&n).Error; e != nil {
					t.Fatal(e)
				}
				return n
			}
			before = count()
			started := time.Now()
			if q.name == "intent" {
				err = r.Cleanup(context.Background(), q.cutoff)
			} else {
				err = r.AllowIP(context.Background(), fmt.Sprint("fresh-", time.Now().UnixNano()), q.cutoff)
			}
			elapsed := time.Since(started)
			after = count()
			t.Logf("cleanup=%s before=%d after=%d removed=%d elapsed=%s (bucket includes bounded counter admission)", q.name, before, after, before-after, elapsed)
			if err != nil || before-after != 20 {
				t.Fatalf("cleanup err=%v removed=%d", err, before-after)
			}
			if elapsed > 100*time.Millisecond {
				t.Errorf("representative fixture exceeded 100ms: %s", elapsed)
			}
		})
	}
	if err = VerifyPermissions(context.Background(), runtime); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaInstallRejectsExistingSchemaWithoutChanges(t *testing.T) {
	owner, _ := ownedDatabase(t)
	if err := Install(context.Background(), owner); err == nil {
		t.Fatal("must not migrate existing schema")
	}
	var count int64
	if err := owner.Raw("SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&count).Error; err != nil || count != 12 {
		t.Fatalf("schema mutated count=%d err=%v", count, err)
	}
}
