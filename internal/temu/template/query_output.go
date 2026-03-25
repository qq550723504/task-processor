package template

import (
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
)

type TemplateQueryOutput struct {
	TemplateInfo            *temutemplate.TemplateInfo
	UserInputParentSpecList []temutemplate.UserInputParentSpec
	InputMaxSpecNum         int
	SingleSpecValueNum      int
}

func buildTemplateQueryOutput(result *temutemplate.TemplateQueryResult) *TemplateQueryOutput {
	if result == nil {
		return nil
	}

	templateInfo := result.TemplateInfo
	return &TemplateQueryOutput{
		TemplateInfo:            &templateInfo,
		UserInputParentSpecList: result.UserInputParentSpecList,
		InputMaxSpecNum:         result.InputMaxSpecNum,
		SingleSpecValueNum:      result.SingleSpecValueNum,
	}
}

func applyTemplateQueryOutput(temuCtx *temucontext.TemuTaskContext, output *TemplateQueryOutput) {
	if output == nil {
		return
	}

	temuCtx.TemplateInfo = output.TemplateInfo
	temuCtx.UserInputParentSpecList = output.UserInputParentSpecList
	temuCtx.InputMaxSpecNum = output.InputMaxSpecNum
	temuCtx.SingleSpecValueNum = output.SingleSpecValueNum
}
