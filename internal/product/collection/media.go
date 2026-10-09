package collection

import "context"

const MaxMediaBytes = 3 << 20

type MediaIdentity struct {
	Hash  string `json:"hash"`
	Bytes int64  `json:"bytes"`
}
type MediaImage struct {
	MediaIdentity
	URL       string `json:"url"`
	MediaType string `json:"mediaType"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}
type SourceMedia interface {
	Upload(context.Context, MediaIdentity, []byte) (MediaImage, error)
	Read(context.Context, MediaIdentity) (MediaImage, error)
}

func (s *Service) WithMedia(media SourceMedia) *Service { s.media = media; return s }
func (s *Service) UploadMedia(ctx context.Context, identity MediaIdentity, raw []byte) (MediaImage, error) {
	if s.media == nil {
		return MediaImage{}, ErrUnavailable
	}
	return s.media.Upload(ctx, identity, raw)
}
func (s *Service) ReadMedia(ctx context.Context, identity MediaIdentity) (MediaImage, error) {
	if s.media == nil {
		return MediaImage{}, ErrUnavailable
	}
	return s.media.Read(ctx, identity)
}
