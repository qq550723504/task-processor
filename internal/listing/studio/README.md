# Listing Studio

Owns studio session, batch, media, and workspace orchestration that is generic to listing flows.

Current stable ownership:

- batch-run service skeleton (`create/get/list/cancel` flow)
- batch-detail read skeleton (`read graph -> fallback -> ensure graph -> project detail`)
- batch review skeleton (`ensure batch -> replace reviews -> reload detail`)
- batch-draft read/delete skeleton (`gallery/list/get/delete` flow)
- session ensure/get skeleton (`ensure/get` flow)
- session async-job sync skeleton (`sync async job -> persist session state`)
- session generation-metadata patch skeleton (`status/job/error` metadata-only updates)
- session review/task-metadata patch skeleton (`approved_design_ids/created_tasks` metadata-only updates)
- session general-metadata patch skeleton (`load session -> apply adapter patch -> persist`)
- batch-run completion skeleton (`cancel unfinished items -> count item statuses -> resolve final run status`)

Boundary guard:

- this package must not depend on `internal/listingkit`, SHEIN marketplace/workspace/publishing packages, or root runtime/integration wiring.
