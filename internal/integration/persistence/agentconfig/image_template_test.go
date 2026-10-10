package agentconfigpersistence

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
)

func imageTemplateFixture() agentconfig.SetTemplate {
	return agentconfig.SetTemplate{Schema: agentconfig.ImageParameterSchema, Mode: "standard", ShareOriginals: true, Background: "白色", Language: "zh", Carousel: []agentconfig.ContentTask{{ID: "identity", Purpose: "product_identity"}}, Detail: []agentconfig.ContentTask{{ID: "overview", Purpose: "product_overview"}}}
}

func TestImageTemplateRevisionRoundTripAndAgentIsolation(t *testing.T) {
	_, store, _ := fixture(t)
	ctx := context.Background()
	scope := agent.Scope{OrganizationID: "image-org", ActorID: "image-actor"}
	enable := command(scope, "enable", 0)
	enable.AgentID, enable.Absent = agentconfig.ImageAgentID, true
	_, err := store.Execute(ctx, enable)
	require.NoError(t, err)
	parameters := imageTemplateFixture()
	create := command(scope, "create-template", 0)
	create.AgentID = agentconfig.ImageAgentID
	create.Input = agentconfig.TemplateInput{Name: "图片模板", TargetPlatform: "product", Image: &parameters}
	receipt, err := store.Execute(ctx, create)
	require.NoError(t, err)
	old, err := store.ReadTemplate(ctx, scope, create.AgentID, receipt.TemplateID, 1)
	require.NoError(t, err)
	require.Equal(t, agentconfig.ImageParameterSchema, old.SchemaVersion)
	require.Equal(t, parameters, *old.Image)
	update := command(scope, "update-template", 1)
	update.AgentID, update.TemplateID, update.Input = create.AgentID, receipt.TemplateID, create.Input
	changed := agentconfig.CloneSetTemplate(parameters)
	changed.Background = "自然光下的室内桌面"
	update.Input.Image = &changed
	_, err = store.Execute(ctx, update)
	require.NoError(t, err)
	old, err = store.ReadTemplate(ctx, scope, create.AgentID, receipt.TemplateID, 1)
	require.NoError(t, err)
	require.Equal(t, parameters, *old.Image, "updates must not rewrite old image parameters")
	newer, err := store.ReadTemplate(ctx, scope, create.AgentID, receipt.TemplateID, 2)
	require.NoError(t, err)
	require.Equal(t, changed, *newer.Image)
	create.AgentID = "product.title.agent"
	_, err = store.Execute(ctx, create)
	require.ErrorIs(t, err, agentconfig.ErrInvalid, "image parameters cannot enter a title template")
}
