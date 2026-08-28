package listingkit_release_authority

import rego.v1

documents := [entry | some entry in input]

release_policies := [entry.contents.releaseAuthority |
  some entry in documents
  entry.contents.releaseAuthority
]

policy := release_policies[0] if count(release_policies) == 1

workflow_documents(owner) := [entry.contents |
  some entry in documents
  endswith(sprintf("%v", [entry.path]), workflow_path(owner))
]

workflow_path(owner) := path if {
  identity := policy.oidc.identities[owner]
  repository_prefix := sprintf("%v/", [policy.repository])
  repository_relative_ref := trim_prefix(identity.workflowRef, repository_prefix)
  path := split(repository_relative_ref, "@")[0]
}

workflow(owner) := contents if {
	count(workflow_documents(owner)) == 1
	contents := workflow_documents(owner)[0]
}

deploy_job(owner) := job if {
  identity := policy.oidc.identities[owner]
  job := workflow(owner).jobs[identity.job]
}

role(owner) := contents if {
  identity := policy.oidc.identities[owner]
  some entry in documents
  entry.contents.kind == "Role"
  entry.contents.metadata.name == identity.role
  contents := entry.contents
}

role_binding(owner) := contents if {
  identity := policy.oidc.identities[owner]
  some entry in documents
  entry.contents.kind == "RoleBinding"
  entry.contents.roleRef.name == identity.role
  contents := entry.contents
}

authentication_config := contents if {
  some entry in documents
  entry.contents.kind == "AuthenticationConfiguration"
  contents := entry.contents
}

role_documents(owner) := [entry.contents |
  identity := policy.oidc.identities[owner]
  some entry in documents
  entry.contents.kind == "Role"
  entry.contents.metadata.name == identity.role
]

role_binding_documents(owner) := [entry.contents |
  identity := policy.oidc.identities[owner]
  some entry in documents
  entry.contents.kind == "RoleBinding"
  entry.contents.roleRef.name == identity.role
]

authentication_configs := [entry.contents |
  some entry in documents
  entry.contents.kind == "AuthenticationConfiguration"
]

runner_deployments := [entry.contents |
  some entry in documents
  entry.contents.kind == "Deployment"
  endswith(entry.contents.metadata.name, "-runner")
]

runner_names := {runner.metadata.name | some runner in runner_deployments}
expected_runner_names := {name | some name in policy.releaseGateRunners.names}

step_names(owner) := {step.name | some step in deploy_job(owner).steps}
expected_step_names(owner) := {name | some name in policy.oidc.identities[owner].allowedSteps}

action_uses(owner) := {step.name: step.uses |
  some step in deploy_job(owner).steps
  step.uses
}

expected_action_uses(owner) := policy.oidc.identities[owner].allowedActions

release_identity_steps := [step |
  some step in deploy_job("api").steps
  step.name == policy.releaseIdentity.step
]

release_identity_step := release_identity_steps[0] if count(release_identity_steps) == 1

routing_inspection_steps := [step |
  some step in deploy_job("api").steps
  contains(step.run, policy.producerRouting.imageLabel)
]

release_gate_steps := [step |
  some step in deploy_job("api").steps
  contains(step.run, "listingkit-run-release-gate-deployment.sh")
]

rule_signatures(owner) := {signature |
  some rule in role(owner).rules
  some resource in rule.resources
  signature := sprintf("%v|%v|%v|%v", [sort(rule.apiGroups), resource, sort(rule.resourceNames), sort(rule.verbs)])
}

expected_rule_signatures(owner) := {signature |
  some protected in policy.protectedResources
  protected.owner == owner
  signature := sprintf("%v|%v|%v|%v", [sort(protected.apiGroups), protected.resource, sort(protected.resourceNames), sort(protected.verbs)])
}

deny contains "release bundle must contain exactly one machine policy" if count(release_policies) != 1

deny contains msg if {
  owner := ["api", "ui"][_]
  count(workflow_documents(owner)) != 1
  msg := sprintf("release bundle must contain exactly one %v workflow", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  count(role_documents(owner)) != 1
  msg := sprintf("release bundle must contain exactly one %v Role", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  count(role_binding_documents(owner)) != 1
  msg := sprintf("release bundle must contain exactly one %v RoleBinding", [owner])
}

deny contains "release bundle must contain exactly one AuthenticationConfiguration" if count(authentication_configs) != 1

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  workflow_doc := workflow(owner)
  object.get(workflow_doc.jobs, identity.job, null) == null
  msg := sprintf("%v release workflow is missing its owned deploy job", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  identity.workflowRef != sprintf("%v/%v@refs/heads/main", [policy.repository, workflow_path(owner)])
  msg := sprintf("%v trusted workflow ref drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  some step in deploy_job(owner).steps
  step.name == identity.kubeconfigStep
  not contains(step.run, sprintf("--workflow-ref %v", [identity.workflowRef]))
  msg := sprintf("%v kubeconfig step does not bind the trusted workflow ref", [owner])
}

deny contains "release-gate runner inventory drifted" if runner_names != expected_runner_names

deny contains "release-gate workflow invocation inventory drifted" if count(release_gate_steps) != 4

deny contains msg if {
  some step in release_gate_steps
  manifest_argument := sprintf("--manifest .workflow-tools/%v", [policy.releaseGateRunners.manifest])
  not contains(step.run, manifest_argument)
  msg := sprintf("release-gate step %v does not select the reviewed aggregate manifest", [step.name])
}

deny contains msg if {
  some step in release_gate_steps
  contains(step.run, "--container")
  msg := sprintf("release-gate step %v exposes caller-controlled container selection", [step.name])
}

deny contains "API release Role must not grant Pod permissions" if {
  some rule in role("api").rules
  startswith(rule.resources[_], "pods")
}

deny contains msg if {
  some step in release_gate_steps
  contains(step.run, "get pods")
  msg := sprintf("release-gate step %v must prove the Deployment rollout without Pod queries", [step.name])
}

deny contains msg if {
  some runner in runner_deployments
  runner.spec.replicas != 0
  msg := sprintf("release-gate runner %v must start at zero replicas", [runner.metadata.name])
}

deny contains msg if {
  some runner in runner_deployments
  count(runner.spec.template.spec.initContainers) != 1
  msg := sprintf("release-gate runner %v must have exactly one init container", [runner.metadata.name])
}

deny contains msg if {
  some runner in runner_deployments
  count(runner.spec.template.spec.containers) != 1
  msg := sprintf("release-gate runner %v must have exactly one hold container", [runner.metadata.name])
}

deny contains msg if {
  some runner in runner_deployments
  runner.spec.template.spec.containers[0].image != policy.releaseGateRunners.holdImage
  msg := sprintf("release-gate runner %v hold image drifted", [runner.metadata.name])
}

deny contains "machine policy repository or namespace drifted" if {
  count(release_policies) == 1
  policy.repository != "qq550723504/task-processor"
}

deny contains "machine policy repository or namespace drifted" if {
  count(release_policies) == 1
  policy.namespace != "task-processor"
}

deny contains "machine policy must pin Conftest and OPA" if {
  count(release_policies) == 1
  policy.tools.conftestVersion != "0.68.2"
}

deny contains "producer routing policy drifted" if {
  policy.producerRouting != {
    "imageLabel": "org.opencontainers.image.listingkit.image-agent-routing",
    "annotation": "listingkit.sh/image-agent-routing-contract",
    "contract": "image-agent-v3-new-starts-v1",
  }
}

deny contains "release identity policy drifted" if {
  policy.releaseIdentity != {
    "owner": "api",
    "deployment": "product-listing-api",
    "step": "Stamp API release identity and restart Pods",
    "annotations": {
      "runId": "listingkit.sh/api-release-run-id",
      "runAttempt": "listingkit.sh/api-release-run-attempt",
      "image": "listingkit.sh/api-release-image",
      "routingContract": "listingkit.sh/image-agent-routing-contract",
    },
  }
}

deny contains "API release identity stamp owner drifted" if count(release_identity_steps) != 1

deny contains "API release identity stamp owner drifted" if {
  policy.releaseIdentity.step in step_names("ui")
}

deny contains "API release identity stamp target drifted" if {
  not contains(release_identity_step.run, sprintf("patch deployment %v", [policy.releaseIdentity.deployment]))
}

deny contains "API release identity annotation drifted" if {
  annotation := [
    policy.releaseIdentity.annotations.runId,
    policy.releaseIdentity.annotations.runAttempt,
    policy.releaseIdentity.annotations.image,
    policy.releaseIdentity.annotations.routingContract,
  ][_]
  not contains(release_identity_step.run, annotation)
}

deny contains "API candidate routing-contract inspection drifted" if count(routing_inspection_steps) != 1

deny contains "API candidate routing-contract inspection drifted" if {
  count(routing_inspection_steps) == 1
  not contains(routing_inspection_steps[0].run, policy.producerRouting.contract)
}

deny contains "machine policy must pin Conftest and OPA" if {
  count(release_policies) == 1
  policy.tools.opaVersion != "1.15.2"
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  job := deploy_job(owner)
  job.environment.name != identity.environment
  msg := sprintf("%v release environment drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  deploy_job(owner).permissions["id-token"] != "write"
  msg := sprintf("%v release job is missing id-token write", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  deploy_job(owner).uses
  msg := sprintf("%v deploy job cannot delegate to a reusable workflow", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  step_names(owner) != expected_step_names(owner)
  msg := sprintf("%v release step ownership drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  count(deploy_job(owner).steps) != count(step_names(owner))
  msg := sprintf("%v release has duplicate or unnamed steps", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  action_uses(owner) != expected_action_uses(owner)
  msg := sprintf("%v action ownership drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  some step in deploy_job(owner).steps
  step.name == identity.oidcTokenStep
  step.env.LISTINGKIT_OIDC_AUDIENCE != identity.audience
  msg := sprintf("%v OIDC audience drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  contains(json.marshal(workflow(owner)), "KUBE_CONFIG")
  msg := sprintf("%v workflow retains a long-lived kubeconfig credential", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  rule_signatures(owner) != expected_rule_signatures(owner)
  msg := sprintf("%v Kubernetes Role differs from the protected-resource policy", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  binding := role_binding(owner)
  count(binding.subjects) != 1
  msg := sprintf("%v RoleBinding subject drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  binding := role_binding(owner)
  binding.subjects[0].kind != "User"
  msg := sprintf("%v RoleBinding subject drifted", [owner])
}

deny contains msg if {
  owner := ["api", "ui"][_]
  identity := policy.oidc.identities[owner]
  binding := role_binding(owner)
  binding.subjects[0].name != sprintf("%v%v", [policy.oidc.usernamePrefix, identity.subject])
  msg := sprintf("%v RoleBinding subject drifted", [owner])
}

deny contains "API and UI release identities must be distinct" if {
  policy.oidc.identities.api.environment == policy.oidc.identities.ui.environment
}

deny contains "API and UI release identities must be distinct" if {
  policy.oidc.identities.api.audience == policy.oidc.identities.ui.audience
}

deny contains "API and UI release identities must be distinct" if {
  policy.oidc.identities.api.subject == policy.oidc.identities.ui.subject
}

deny contains "Kubernetes authentication issuer or audience drifted" if {
  auth := authentication_config.jwt[0]
  auth.issuer.url != policy.oidc.issuer
}

deny contains "Kubernetes authentication issuer or audience drifted" if {
  auth := authentication_config.jwt[0]
  {x | some x in auth.issuer.audiences} != {policy.oidc.identities.api.audience, policy.oidc.identities.ui.audience}
}

deny contains "Kubernetes authentication claim rules do not bind both exact release identities" if {
  encoded := json.marshal(authentication_config)
  not contains(encoded, policy.oidc.identities.api.subject)
}

deny contains "Kubernetes authentication claim rules do not bind both exact release identities" if {
  encoded := json.marshal(authentication_config)
  not contains(encoded, policy.oidc.identities.ui.subject)
}

deny contains "Kubernetes authentication claim rules do not bind both exact release identities" if {
  encoded := json.marshal(authentication_config)
  not contains(encoded, policy.oidc.identities.api.workflowRef)
}

deny contains "Kubernetes authentication claim rules do not bind both exact release identities" if {
  encoded := json.marshal(authentication_config)
  not contains(encoded, policy.oidc.identities.ui.workflowRef)
}

deny contains "Kubernetes authentication claim rules use workflow-ref prefix authorization" if {
  contains(json.marshal(authentication_config), "workflow_ref.startsWith")
}
