package listingkit

import "testing"

func TestTaskTaxonomyBoundary(t *testing.T) {
	t.Parallel()

	buildSource := readNamedFunctionSource(t, "task_contract.go", "BuildTaskListTaxonomy")
	cloneSource := readNamedFunctionSource(t, "task_contract.go", "cloneTaskFacetDescriptorsFromWorkspace")

	assertSourceContainsAll(t, buildSource, []string{
		"SheinWorkflowStatuses: cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.WorkflowStatusDescriptors())",
		"SheinWorkQueues:       cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.WorkQueueDescriptors())",
		"SheinActionQueues:     cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.ActionQueueDescriptors())",
	})
	assertSourceContainsAll(t, cloneSource, []string{
		"func cloneTaskFacetDescriptorsFromWorkspace(items []sheinworkspace.FacetDescriptor) []TaskFacetDescriptor",
		"Key:         item.Key",
		"Label:       item.Label",
		"Description: item.Description",
		"Severity:    item.Severity",
	})
}
