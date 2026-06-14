package workspace

import "testing"

func TestWorkflowAndQueueDescriptors(t *testing.T) {
	t.Parallel()

	workflow := WorkflowStatusDescriptors()
	if len(workflow) == 0 || workflow[0].Key != WorkflowStatusPendingConfirmation {
		t.Fatalf("WorkflowStatusDescriptors() = %+v, want pending confirmation first", workflow)
	}

	workQueues := WorkQueueDescriptors()
	if len(workQueues) == 0 || workQueues[0].Key != WorkQueueGeneration {
		t.Fatalf("WorkQueueDescriptors() = %+v, want generation first", workQueues)
	}

	actionQueues := ActionQueueDescriptors()
	if len(actionQueues) == 0 || actionQueues[0].Key != ActionQueueStoreAuth {
		t.Fatalf("ActionQueueDescriptors() = %+v, want store auth first", actionQueues)
	}

	workflow[0].Label = "changed"
	if WorkflowStatusDescriptors()[0].Label == "changed" {
		t.Fatal("WorkflowStatusDescriptors() should return a clone")
	}
}
