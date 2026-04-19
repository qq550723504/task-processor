package productimage

import (
	"fmt"

	productenrich "task-processor/internal/productenrich"
)

type DefaultModelProviderConfig struct {
	LLMManager      productenrich.LLMManager
	WorkDir         string
	Segmenter       SegmentationClient
	WhiteBackground WhiteBackgroundClient
	SceneGenerator  SceneGenerationClient
}

type defaultModelProvider struct {
	faithfulEditor FaithfulEditor
	sceneGenerator SceneGenerator
	reviewModel    ImageReviewModel
}

func NewModelProvider(faithfulEditor FaithfulEditor, sceneGenerator SceneGenerator, reviewModel ImageReviewModel) ProductImageModelProvider {
	return &defaultModelProvider{
		faithfulEditor: faithfulEditor,
		sceneGenerator: sceneGenerator,
		reviewModel:    reviewModel,
	}
}

func NewDefaultModelProvider(config DefaultModelProviderConfig) (ProductImageModelProvider, error) {
	if config.LLMManager == nil && config.Segmenter == nil && config.WhiteBackground == nil {
		return nil, fmt.Errorf("default model provider requires at least one configured capability")
	}

	var faithfulEditor FaithfulEditor
	if config.Segmenter != nil || config.WhiteBackground != nil {
		editor, err := NewRemoteFaithfulEditor(RemoteFaithfulEditorConfig{
			WorkDir:         config.WorkDir,
			Segmenter:       config.Segmenter,
			WhiteBackground: config.WhiteBackground,
		})
		if err != nil {
			return nil, err
		}
		faithfulEditor = editor
	}

	var reviewModel ImageReviewModel
	if config.LLMManager != nil {
		model, err := NewLLMReviewModel(config.LLMManager)
		if err != nil {
			return nil, err
		}
		reviewModel = model
	}

	var sceneGenerator SceneGenerator
	if config.SceneGenerator != nil {
		generator, err := NewRemoteSceneGenerator(RemoteSceneGeneratorConfig{
			WorkDir: config.WorkDir,
			Client:  config.SceneGenerator,
		})
		if err != nil {
			return nil, err
		}
		sceneGenerator = generator
	}

	return &defaultModelProvider{
		faithfulEditor: faithfulEditor,
		sceneGenerator: sceneGenerator,
		reviewModel:    reviewModel,
	}, nil
}

func (p *defaultModelProvider) FaithfulEditor() FaithfulEditor {
	return p.faithfulEditor
}

func (p *defaultModelProvider) SceneGenerator() SceneGenerator {
	return p.sceneGenerator
}

func (p *defaultModelProvider) ReviewModel() ImageReviewModel {
	return p.reviewModel
}
