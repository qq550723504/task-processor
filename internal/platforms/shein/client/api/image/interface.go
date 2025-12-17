package image

type ImageAPI interface {
	UploadOriginalImage(imageData []byte) (string, error)
	DownloadAndUploadImage(imageURL string) (string, error)
}
