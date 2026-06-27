package preview

// ResultProjectionInput captures a built preview shell and exposes only the
// generic result-driven fields that facade adapters should project onward.
type ResultProjectionInput struct {
	Preview *Preview
}

type ResultProjection struct {
	NeedsReview         bool
	Attachment          *Attachment
	Overview            *Header
	RevisionHistoryMeta *RevisionHistoryMeta
}

func BuildResultProjection(input ResultProjectionInput) ResultProjection {
	if input.Preview == nil {
		return ResultProjection{}
	}
	return ResultProjection{
		NeedsReview:         input.Preview.NeedsReview,
		Attachment:          input.Preview.Attachment,
		Overview:            cloneHeader(input.Preview.Overview),
		RevisionHistoryMeta: cloneRevisionHistoryMeta(input.Preview.RevisionHistoryMeta),
	}
}

func cloneHeader(input *Header) *Header {
	if input == nil {
		return nil
	}
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

func cloneRevisionHistoryMeta(input *RevisionHistoryMeta) *RevisionHistoryMeta {
	if input == nil {
		return nil
	}
	return &RevisionHistoryMeta{
		TotalRecords:    input.TotalRecords,
		ReturnedRecords: input.ReturnedRecords,
		HasMore:         input.HasMore,
		IsTruncated:     input.IsTruncated,
		MaxRecords:      input.MaxRecords,
	}
}
