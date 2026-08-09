package productimage

import (
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestOpenAICompatibleSceneGeneratorAppliesRouteModel(t *testing.T) {
	sourcePath := t.TempDir() + "/source.png"
	file, err := os.Create(sourcePath)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			source.Set(x, y, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		t.Fatalf("encode source: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}

	responseImage, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source response: %v", err)
	}
	client := &recordingSceneImageClient{defaultModel: "legacy-model", responseImage: responseImage}
	generator, err := NewOpenAICompatibleSceneGenerator(t.TempDir(), client)
	if err != nil {
		t.Fatalf("NewOpenAICompatibleSceneGenerator: %v", err)
	}
	routed, ok := generator.(SceneGeneratorWithRoute)
	if !ok {
		t.Fatalf("generator does not implement SceneGeneratorWithRoute")
	}

	_, err = routed.GenerateSceneWithRoute(context.Background(), &SceneGenerationRequest{
		SourceAsset: &ImageAsset{URL: sourcePath, Metadata: map[string]string{"local_path": sourcePath}},
	}, SceneGenerationRoute{RoutingKey: "productimage-image", ModelID: "routed-model"})
	if err != nil {
		t.Fatalf("GenerateSceneWithRoute: %v", err)
	}
	if client.lastRequest == nil || client.lastRequest.Model != "routed-model" {
		t.Fatalf("request = %+v, want routed model", client.lastRequest)
	}
}

type recordingSceneImageClient struct {
	defaultModel  string
	responseImage []byte
	lastRequest   *openaiclient.ImageEditRequest
}

func (c *recordingSceneImageClient) EditImage(_ context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	c.lastRequest = req
	return &openaiclient.ImageResponse{Data: []openaiclient.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(c.responseImage)}}}, nil
}

func (c *recordingSceneImageClient) GetDefaultModel() string { return c.defaultModel }
