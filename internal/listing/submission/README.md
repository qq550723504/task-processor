# Listing Submission

Owns submit, retry, recovery, state, and submission orchestration that is generic to listing flows.

Current stable ownership:

- generic submission refresh orchestration seam (`RefreshStatus` style load/resolve/finish flow)
- generic task requeue orchestration seam (`RequeueTasks` style load/check/submit flow)
- generic immediate recovery orchestration seam (`RecoverNow` style load/recover/submit/reload flow)
- generic batch recovery orchestration seam (`RecoverBatch` style list/recover/submit/aggregate flow)
- generic direct-submit phase orchestration seam (`DirectSubmit` style build/prepare/upload/pre-validate/submit flow)
- generic prepared-payload phase seam (`Prepare/Upload/PreValidate` stage flow)
- generic remote-submit attempt seam (`prepare state -> execute attempt -> shape result`)
- generic post-success persistence seam (`persist result/phase -> complete attempt -> remember -> persist success`)
- generic failure-record persistence seam (`record failure event/state`)
- source-facts readiness policy for 1688-derived facts
- in-process submit lock manager

Does not own yet:

- full submit orchestration and platform routing
- SHEIN-specific submit package loading and remote confirmation details
- Temporal-facing submit workflow adapters
- durable retry/reblock persistence policies
