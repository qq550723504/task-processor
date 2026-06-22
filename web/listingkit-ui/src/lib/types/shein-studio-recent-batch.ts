export type SheinStudioRecentBatchSummary = {
  id: string;
  source: "batch" | "local_draft";
  isRecoverableDraft: boolean;
  title: string;
  primaryProductName: string;
  productCount: number;
  promptPreview: string;
  storeSummary: string;
  designCount: number;
  createdTaskCount: number;
  batchStatus?: string;
  updatedAt: string;
  alerts?: SheinStudioRecentBatchAlert[];
  recentResults?: SheinStudioRecentBatchResult[];
};

export type SheinStudioRecentBatchAlert = {
  tone: "warning" | "danger";
  label: string;
  reasonCode?: string;
  detail?: string;
};

export type SheinStudioRecentBatchResult = {
  tone: "success" | "warning" | "danger";
  label: string;
  detail?: string;
};
