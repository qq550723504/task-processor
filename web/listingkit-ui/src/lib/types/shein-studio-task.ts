export type SheinStudioTaskLifecycleStatus =
  | "task_created"
  | "needs_review"
  | "ready_to_submit"
  | "draft_saved"
  | "published"
  | "submit_failed"
  | "unknown";

export type SheinStudioTaskOutcome = "created" | "reused" | "rejected" | "failed";
export type SheinStudioTaskSource =
  | "batch_created"
  | "legacy_session_backfilled"
  | "rejected"
  | string;

export type SheinStudioTaskOutcomeBase = {
  designId: string;
  itemId?: string;
  selectionId?: string;
  compatibilityFingerprint?: string;
  status?: SheinStudioTaskLifecycleStatus | string;
  submissionState?: SheinStudioTaskLifecycleStatus | string;
  lastSubmissionAction?: string;
  source?: SheinStudioTaskSource;
  reasonCode?: string;
  message?: string;
};

export type SheinStudioCreatedTask = SheinStudioTaskOutcomeBase & {
  id: string;
  title: string;
  outcome?: "created" | "reused";
};

export type SheinStudioRejectedTask = SheinStudioTaskOutcomeBase & {
  title?: string;
  outcome?: "rejected";
};

export type SheinStudioFailedTask = SheinStudioTaskOutcomeBase & {
  title: string;
  message: string;
  outcome?: "failed";
};
