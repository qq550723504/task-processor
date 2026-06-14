package workspace

func BuildRevisionInputFromEditorContext(ctx *EditorContext) *RevisionInput {
	if ctx == nil {
		return nil
	}
	input := &RevisionInput{}
	if ctx.Basics != nil {
		input.SpuName = stringPointerOrNil(ctx.Basics.SpuName)
		input.ProductNameEn = stringPointerOrNil(ctx.Basics.ProductNameEn)
		input.BrandName = stringPointerOrNil(ctx.Basics.BrandName)
		input.Description = stringPointerOrNil(ctx.Basics.Description)
		input.Images = cloneImageSet(ctx.Basics.Images)
		input.ReviewNotes = append([]string(nil), ctx.Basics.ReviewNotes...)
	}
	if ctx.Category != nil {
		input.CategoryResolution = CloneCategoryResolutionPatch(ctx.Category.SuggestedPatch)
	}
	if ctx.Attributes != nil {
		input.AttributeResolution = CloneAttributeResolutionPatch(ctx.Attributes.SuggestedPatch)
	}
	if ctx.SaleAttributes != nil {
		input.SaleAttributeResolution = CloneSaleAttributeResolutionPatch(ctx.SaleAttributes.SuggestedResolutionPatch)
		input.SKCPatches = CloneSKCRevisionPatches(ctx.SaleAttributes.SuggestedSKCPatches)
	}
	if IsEmptyRevisionInput(input) {
		return nil
	}
	return input
}

func IsEmptyRevisionInput(input *RevisionInput) bool {
	return input == nil ||
		(input.SpuName == nil &&
			input.ProductNameEn == nil &&
			input.BrandName == nil &&
			input.Description == nil &&
			len(input.SellingPoints) == 0 &&
			input.CategoryName == nil &&
			len(input.CategoryPath) == 0 &&
			input.CategoryID == nil &&
			len(input.CategoryIDList) == 0 &&
			input.ProductTypeID == nil &&
			input.TopCategoryID == nil &&
			input.Images == nil &&
			len(input.ProductAttributes) == 0 &&
			len(input.ResolvedAttributes) == 0 &&
			input.CategoryResolution == nil &&
			input.AttributeResolution == nil &&
			input.SaleAttributeResolution == nil &&
			len(input.SKCPatches) == 0 &&
			input.RequestDraft == nil &&
			len(input.ReviewNotes) == 0)
}
