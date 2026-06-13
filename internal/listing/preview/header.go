package preview

type HeaderInput struct {
	Country       string
	Language      string
	SourceType    string
	ImageCount    int
	VariantCount  int
	StatusMessage string
	Warnings      []string
	ReviewReasons []string
	PlatformCards []PlatformCard
}

func BuildHeader(input HeaderInput) *Header {
	return &Header{
		Country:       input.Country,
		Language:      input.Language,
		SourceType:    input.SourceType,
		ImageCount:    input.ImageCount,
		VariantCount:  input.VariantCount,
		StatusMessage: input.StatusMessage,
		Warnings:      append([]string(nil), input.Warnings...),
		ReviewReasons: append([]string(nil), input.ReviewReasons...),
		PlatformCards: append([]PlatformCard(nil), input.PlatformCards...),
	}
}
