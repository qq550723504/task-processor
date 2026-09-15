package referral

import (
	"context"
	domain "task-processor/internal/referral"
	"time"
)

func (r *Repository) AllowIP(ctx context.Context, key string, now time.Time) error {
	if key == "" || len(key) > 200 || ctx == nil {
		return domain.ErrInvalid
	}
	cleanup, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	err := r.db.WithContext(cleanup).Exec(`DELETE FROM public.registration_admission_buckets WHERE ctid IN
 (SELECT ctid FROM public.registration_admission_buckets WHERE window_start < ? ORDER BY window_start LIMIT 20 FOR UPDATE SKIP LOCKED)`, now.UTC().Truncate(time.Minute)).Error
	cancel()
	if err != nil {
		return domain.ErrUnavailable
	}
	ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result := r.db.WithContext(ctx).Exec(`INSERT INTO public.registration_admission_buckets(kind,key,window_start,hits) VALUES ('ip',?,?,1)
 ON CONFLICT(kind,key,window_start) DO UPDATE SET hits=registration_admission_buckets.hits+1
 WHERE registration_admission_buckets.hits<5`, key, now.UTC().Truncate(time.Minute))
	if result.Error != nil {
		return domain.ErrUnknown
	}
	if result.RowsAffected != 1 {
		return domain.ErrLimited
	}
	return nil
}
