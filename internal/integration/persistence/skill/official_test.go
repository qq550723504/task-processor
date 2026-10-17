package skillpersistence

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	d "task-processor/internal/skill"
)

func TestOfficialInstallKeepsExistingVersionsAndStatesAcrossMaintainers(t *testing.T) {
	db, s := fixture(t)
	ctx := context.Background()
	p := d.Principal{UserID: "admin-a", PlatformMaintainer: true}
	resolve := func(context.Context) (d.Principal, error) { return p, nil }
	first, err := s.InstallOfficial(ctx, p, resolve)
	require.NoError(t, err)
	require.Len(t, first.Installed, 6)
	require.Empty(t, first.Existing)
	v, err := s.Read(ctx, p, d.Owner{Kind: d.PlatformOwner, ID: d.PlatformID}, first.Installed[0].ID, 1)
	require.NoError(t, err)
	require.Equal(t, d.Enabled, v.State)
	require.True(t, v.AllowCopy && v.AllowExport)
	_, err = s.Execute(ctx, d.Command{Principal: p, Owner: v.Metadata.Owner, Key: uuid.NewString(), ID: v.Metadata.ID, Expected: v.Metadata.Revision, Version: 1, Operation: "disable"}, resolve)
	require.NoError(t, err)
	secondAdmin := d.Principal{UserID: "admin-b", PlatformMaintainer: true}
	second, err := s.InstallOfficial(ctx, secondAdmin, func(context.Context) (d.Principal, error) { return secondAdmin, nil })
	require.NoError(t, err)
	require.Empty(t, second.Installed)
	require.Len(t, second.Existing, 6)
	unchanged, err := s.Read(ctx, p, v.Metadata.Owner, v.Metadata.ID, 1)
	require.NoError(t, err)
	require.Equal(t, d.Disabled, unchanged.State)
	require.Equal(t, v.Package, unchanged.Package)
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM skill_capability.versions").Scan(&count).Error)
	require.EqualValues(t, 6, count)
	_, err = s.InstallOfficial(ctx, d.Principal{UserID: "ordinary-user"}, resolve)
	require.ErrorIs(t, err, d.ErrForbidden)
}

func TestOfficialInstallConcurrentAndConflictRollsBackEntireRelease(t *testing.T) {
	db, s := fixture(t)
	ctx := context.Background()
	p := d.Principal{UserID: "admin", PlatformMaintainer: true}
	resolve := func(context.Context) (d.Principal, error) { return p, nil }
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.InstallOfficial(ctx, p, resolve); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	var rows []skillRow
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 6)
	// Leave a conflicting release identity and remove the others, only inside
	// this fixture DB. The installer must roll back every otherwise-new row.
	conflict := rows[len(rows)-1].ID
	require.NoError(t, db.Exec("DELETE FROM skill_capability.versions WHERE skill_id<>?", conflict).Error)
	require.NoError(t, db.Exec("DELETE FROM skill_capability.skills WHERE id<>?", conflict).Error)
	require.NoError(t, db.Exec("UPDATE skill_capability.skills SET owner_kind='USER',owner_id='other-user' WHERE id=?", conflict).Error)
	_, err := s.InstallOfficial(ctx, p, resolve)
	require.ErrorIs(t, err, d.ErrConflict)
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM skill_capability.skills").Scan(&count).Error)
	require.EqualValues(t, 1, count)
}
