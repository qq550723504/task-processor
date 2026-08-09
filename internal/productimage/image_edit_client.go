package productimage

import "context"

type imageEditClient interface {
	EditImage(ctx context.Context, req imageEditRequest) (*imageEditResponse, error)
	GetDefaultModel() string
}

type routeBoundImageEditClient interface {
	EditImageWithRoute(ctx context.Context, req imageEditRequest, route SceneGenerationRoute) (*imageEditResponse, error)
}

type imageEditRequest struct {
	Model          string
	Prompt         string
	Image          []byte
	ImageURL       string
	ResponseFormat string
	N              int
	Size           string
}

type imageEditResponse struct {
	Data []imageEditData
}

type imageEditData struct {
	B64JSON       string
	RevisedPrompt string
}
