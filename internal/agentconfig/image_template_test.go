package agentconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	strict "sigs.k8s.io/json"
)

func TestImageTemplateHasItsOwnTypedParameters(t *testing.T) {
	raw := []byte(`{"name":"整套商品图片","targetPlatform":"product","image":{"schema":"image-config-v1","mode":"standard","shareOriginals":true,"background":"白色","language":"zh","carousel":[{"id":"identity","purpose":"product_identity"}],"detail":[{"id":"overview","purpose":"product_overview"}]}}`)
	var input TemplateInput
	violations, err := strict.UnmarshalStrict(raw, &input)
	require.NoError(t, err)
	require.Empty(t, violations, "image parameters must be typed rather than silently discarded")
	require.True(t, input.Valid())
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	var before, after map[string]any
	require.NoError(t, json.Unmarshal(raw, &before))
	require.NoError(t, json.Unmarshal(encoded, &after))
	require.Equal(t, before, after)
}

func TestImageTemplateRejectsAuthorityFieldsWrongAgentAndByteOverflow(t *testing.T) {
	parameters := SetTemplate{Schema: ImageParameterSchema, Mode: "standard", ShareOriginals: true, Background: "白色", Language: "zh", Carousel: []ContentTask{{ID: "identity", Purpose: "product_identity"}}}
	input := TemplateInput{Name: "图片", TargetPlatform: "product", Image: &parameters}
	require.True(t, input.ValidForAgent(ImageAgentID))
	require.False(t, input.ValidForAgent("product.title.agent"))
	input.DefaultKnowledgeBaseID = "9e7afaa9-a9f9-48ba-a11a-b5bb377f08e9"
	require.False(t, input.Valid())
	parameters.Background = strings.Repeat("中", MaxSetBackgroundBytes/3+1)
	require.Error(t, parameters.Validate(), "the bound is UTF-8 bytes, not a character count")
	parameters.Background = "白色"
	parameters.Carousel = append(parameters.Carousel, ContentTask{ID: "second", Purpose: "product_identity"})
	require.Error(t, parameters.Validate(), "standard tasks are selectable content intents, not duplicate paid outputs")
	var decoded TemplateInput
	violations, err := strict.UnmarshalStrict([]byte(`{"name":"图片","targetPlatform":"product","image":{"schema":"image-config-v1","mode":"custom","model":"other","credential":"secret"}}`), &decoded)
	require.NoError(t, err)
	require.NotEmpty(t, violations, "templates cannot inject provider/model/credential authority")
}
