import type {
  CreateListingKitTaskRequest,
  ListingKitTaskListItem,
} from "@/lib/types/listingkit/tasks";

const legacyStoreFallbackField = `shein_store_${"fallback"}` as const;

const taskListItemWithoutLegacyStoreFallback = {
  task_id: "task-explicit-store",
  // @ts-expect-error Legacy fallback-store response fields are not part of the current UI contract.
  [legacyStoreFallbackField]: true,
} satisfies ListingKitTaskListItem;

type CreateTaskOptions = NonNullable<CreateListingKitTaskRequest["options"]>;

export const createTaskOptionsExcludeRetiredSheinStudio: Extract<
  keyof CreateTaskOptions,
  "shein_studio"
> extends never
  ? true
  : false = true;

void taskListItemWithoutLegacyStoreFallback;
