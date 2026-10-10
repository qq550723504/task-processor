package imageagentapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
	"testing"
)

type routedSourceFixture struct {
	kind   imageagent.ImageSourceContextKind
	calls  int
	denied bool
}

func (f *routedSourceFixture) ReadImageSetSource(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	f.calls++
	if f.denied {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	return imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ContextKind: f.kind}}, nil
}
func TestImageSetSourceRouterNeverTriesAnotherOwnerAfterDenial(t *testing.T) {
	a := &routedSourceFixture{kind: imageagent.ImageSourceAcquisition, denied: true}
	s := &routedSourceFixture{kind: imageagent.ImageSourceSupply}
	r := ImageSetSourceRouter{Acquisition: a, Supply: s}
	_, err := r.ReadImageSetSource(context.Background(), imageagent.ExecutionIdentity{}, imageagent.PrepareImageSetInput{ContextKind: imageagent.ImageSourceAcquisition})
	require.Error(t, err)
	require.Equal(t, 1, a.calls)
	require.Zero(t, s.calls)
	_, err = r.ReadImageSetSource(context.Background(), imageagent.ExecutionIdentity{}, imageagent.PrepareImageSetInput{ContextKind: "unknown"})
	require.Error(t, err)
	require.Zero(t, s.calls)
	_, err = r.ReadImageSetSource(context.Background(), imageagent.ExecutionIdentity{}, imageagent.PrepareImageSetInput{ContextKind: imageagent.ImageSourceSupply})
	require.NoError(t, err)
	require.Equal(t, 1, s.calls)
	s.kind = imageagent.ImageSourceAcquisition
	_, err = r.ReadImageSetSource(context.Background(), imageagent.ExecutionIdentity{}, imageagent.PrepareImageSetInput{ContextKind: imageagent.ImageSourceSupply})
	require.Error(t, err, "the selected owner must attest the same explicit context")
}
