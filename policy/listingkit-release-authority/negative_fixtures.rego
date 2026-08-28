package listingkit_release_fixture

import rego.v1

deny contains "long-lived kubeconfig fallback" if input.workflow.credentials[_] == "KUBE_CONFIG"
deny contains "shared production environment" if input.identities.api.environment == input.identities.ui.environment
deny contains "shared OIDC subject" if input.identities.api.subject == input.identities.ui.subject
deny contains "missing id-token write" if input.workflow.permissions["id-token"] != "write"
deny contains "wrong OIDC audience" if input.workflow.audience != input.identity.audience
deny contains "workflow job owner drift" if input.workflow.job != input.identity.job
deny contains "workflow step owner drift" if input.workflow.steps != input.identity.allowedSteps
deny contains "reusable workflow bypass" if input.workflow.uses
deny contains "composite action bypass" if startswith(input.workflow.stepUses, "./")
deny contains "with target bypass" if input.workflow.with.target != input.identity.target
deny contains "excess RBAC verbs" if input.rbac.verbs[_] == input.excessVerb
deny contains "excess RBAC resources" if input.rbac.resources[_] == input.excessResource
deny contains "excess RBAC resource names" if input.rbac.resourceNames[_] == input.excessResourceName
deny contains "machine policy drift" if input.policy.repository != "qq550723504/task-processor"
deny contains "release identity owner drift" if input.releaseIdentity.owner != "api"
deny contains "untrusted workflow ref" if input.token.workflowRef != input.identity.workflowRef
deny contains "missing workflow ref" if not input.token.workflowRef
deny contains "trusted workflow-ref policy drift" if input.policy.workflowRef != input.expectedWorkflowRef
