export type SheinStudioBatchRunStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "partially_succeeded"
  | "failed"
  | "cancelled";

export type SheinStudioBatchRunItemStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled";

export type SheinStudioBatchRunMode = "generate" | "create_tasks";

export type SheinStudioBatchRunFailurePolicy =
  | "continue_on_error"
  | "stop_on_error";

export type SheinStudioBatchRun = {
  id: string;
  mode: SheinStudioBatchRunMode;
  failurePolicy: SheinStudioBatchRunFailurePolicy;
  status: SheinStudioBatchRunStatus;
  currentBatchId?: string;
  currentIndex: number;
  totalBatches: number;
  completedBatches: number;
  succeededBatches: number;
  failedBatches: number;
  lastError?: string;
  cancelRequested: boolean;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type SheinStudioBatchRunItem = {
  id: string;
  runId: string;
  batchId: string;
  position: number;
  status: SheinStudioBatchRunItemStatus;
  sessionId?: string;
  asyncJobId?: string;
  errorMessage?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type SheinStudioBatchRunStartResponse = {
  run: SheinStudioBatchRun;
  items: SheinStudioBatchRunItem[];
};
