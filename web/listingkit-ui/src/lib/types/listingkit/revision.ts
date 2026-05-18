import type { SheinStoreResolutionSummary } from "./shein";

export type ListingKitRevisionTimelineSummary = {
  headline?: string;
  badge?: string;
  relation_text?: string;
  change_count?: number;
};

export type RevisionDiffPreview = {
  change_count?: number;
  field_changes?: Array<{
    field_path?: string;
    before?: string;
    after?: string;
  }>;
};

export type RevisionHistoryNavigation = {
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

export type ListingKitRevisionHistoryCounts = {
  all?: number;
  edit?: number;
  restore?: number;
};

export type ListingKitRevisionHistoryPageMeta = {
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
