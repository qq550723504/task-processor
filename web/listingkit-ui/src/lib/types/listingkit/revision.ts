import type { SheinStoreResolutionSummary } from "./shein";

 type ListingKitRevisionTimelineSummary = {
  headline?: string;
  badge?: string;
  relation_text?: string;
  change_count?: number;
};

 type RevisionDiffPreview = {
  change_count?: number;
  field_changes?: Array<{
    field_path?: string;
    before?: string;
    after?: string;
  }>;
};

 type RevisionHistoryNavigation = {
  prev_revision_id?: string;
  next_revision_id?: string;
};

export type ListingKitRevisionRecord = {
  revision_id?: string;
  updated_at?: string;
  updated_by?: string;
  reason?: string;
  platform?: string;
  action_type?: string;
  restored_from_revision_id?: string;
  timeline?: ListingKitRevisionTimelineSummary;
  applied_changes?: RevisionDiffPreview;
  store_resolution?: SheinStoreResolutionSummary;
};

 type ListingKitRevisionHistoryCounts = {
  all?: number;
  edit?: number;
  restore?: number;
};

 type ListingKitRevisionHistoryPageMeta = {
  total_records?: number;
  returned_records?: number;
  has_more?: boolean;
  is_truncated?: boolean;
  max_records?: number;
  next_before?: string;
  action_type?: string;
  counts?: ListingKitRevisionHistoryCounts;
};

export type ListingKitRevisionHistoryPage = {
  task_id: string;
  items?: ListingKitRevisionRecord[];
  meta?: ListingKitRevisionHistoryPageMeta;
};

export type ListingKitRevisionHistoryDetail = {
  task_id: string;
  record?: ListingKitRevisionRecord;
  navigation?: RevisionHistoryNavigation;
  history_index?: number;
  total_records?: number;
  restore_payload?: unknown;
};
