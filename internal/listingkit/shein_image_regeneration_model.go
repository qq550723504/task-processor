package listingkit

type RegenerateSheinDataImageRequest struct {
	ImageURL string `json:"image_url,omitempty"`
	Label    string `json:"label,omitempty"`
	Role     string `json:"role,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

type RegenerateSheinDataImageResponse struct {
	Preview     *ListingKitPreview   `json:"preview,omitempty"`
	Image       StudioGeneratedImage `json:"image,omitempty"`
	ReplacedURL string               `json:"replaced_url,omitempty"`
}
