package listingkit

import listingworkflow "task-processor/internal/listingkit/workflow"

func shouldRunStudioInline(req *GenerateRequest) bool {
	return listingworkflow.ShouldRunStudioInline(buildWorkflowRequestPolicyInput(req))
}

func shouldRunRemoteSDSDesignSync(req *GenerateRequest) bool {
	return listingworkflow.ShouldRunRemoteSDSDesignSync(buildWorkflowRequestPolicyInput(req))
}
