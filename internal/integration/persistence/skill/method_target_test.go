package skillpersistence

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	configstore "task-processor/internal/integration/persistence/agentconfig"
	d "task-processor/internal/skill"
)

func TestImageMethodsRequireTheirOwnActivationTargetAndSharedOwner(t *testing.T) {
	db, s := fixture(t)
	ctx := context.Background()
	admin := d.Principal{UserID: "maintainer", PlatformMaintainer: true}
	installed, err := s.InstallOfficial(ctx, admin, func(context.Context) (d.Principal, error) { return admin, nil })
	require.NoError(t, err)
	p := d.Principal{UserID: "alice", OrganizationID: "org-a", MemberID: "member-a", UseOrganization: true, ReadOrganization: true, ConfigureOrganization: true}
	cfg, err := configstore.New(db)
	require.NoError(t, err)
	for _, id := range []string{agentconfig.ContentAgentID, agentconfig.ImageAgentID} {
		_, err = cfg.Execute(ctx, agentconfig.Command{Scope: agent.Scope{OrganizationID: p.OrganizationID, ActorID: p.UserID}, Key: uuid.NewString(), AgentID: id, Operation: "enable", Absent: true})
		require.NoError(t, err)
	}
	for _, target := range []string{d.TitleTarget, d.ImageTarget} {
		items, err := s.CandidatesForTarget(ctx, p, target)
		require.NoError(t, err)
		require.Len(t, items, 3)
		for _, v := range items {
			require.True(t, v.Package.Supports(target))
			require.Empty(t, v.Package.Files)
		}
	}
	var image agent.SkillVersionRef
	for _, ref := range installed.Installed {
		v, e := s.Read(ctx, admin, d.Owner{Kind: d.PlatformOwner, ID: d.PlatformID}, ref.ID, 1)
		require.NoError(t, e)
		if v.Package.Target == d.ImageTarget {
			image = ref
			break
		}
	}
	_, err = s.ReadForUse(ctx, p, image)
	require.ErrorIs(t, err, d.ErrForbidden)
	pkg, release, err := s.AcquireUseForTarget(ctx, p, image, d.ImageTarget)
	require.NoError(t, err)
	require.Equal(t, d.ImageTarget, pkg.Target)
	release()
	resolve := func(context.Context) (d.Principal, error) { return p, nil }
	for _, owner := range []d.Owner{{Kind: d.UserOwner, ID: p.UserID}, {Kind: d.OrganizationOwner, ID: p.OrganizationID}} {
		r, e := s.Execute(ctx, d.Command{Principal: p, Owner: owner, Key: uuid.NewString(), Operation: "copy", Source: &image}, resolve)
		require.NoError(t, e)
		for _, action := range []string{"publish", "enable"} {
			r, e = s.Execute(ctx, d.Command{Principal: p, Owner: owner, Key: uuid.NewString(), ID: r.ID, Version: 1, Expected: r.Revision, Operation: action}, resolve)
			require.NoError(t, e)
		}
		v, e := s.Read(ctx, p, owner, r.ID, 1)
		require.NoError(t, e)
		_, release, e := s.AcquireUseForTarget(ctx, p, v.Ref(), d.ImageTarget)
		if owner.Kind == d.UserOwner {
			require.ErrorIs(t, e, d.ErrForbidden)
			require.Nil(t, release)
		} else {
			require.NoError(t, e)
			release()
			other := p
			other.OrganizationID = "org-b"
			_, _, e = s.AcquireUseForTarget(ctx, other, v.Ref(), d.ImageTarget)
			require.Error(t, e)
		}
	}
	_, err = s.CandidatesForTarget(ctx, p, "arbitrary-target")
	require.ErrorIs(t, err, d.ErrInvalid)
}
