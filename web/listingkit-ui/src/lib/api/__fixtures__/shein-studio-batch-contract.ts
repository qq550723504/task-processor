export const sheinStudioBatchTaskCreationContractFixture = {
  response: {
    batch: {
      id: "batch-1",
      status: "tasks_created",
      prompt: "botanical",
      style_count: "3",
      shein_store_id: 7,
      created_at: "2026-06-01T10:00:00Z",
      updated_at: "2026-06-01T10:05:00Z",
    },
    items: [
      {
        item: {
          id: "item-1",
          batch_id: "batch-1",
          target_group_key: "size:1200x1200",
          target_group_label: "1200 x 1200",
          status: "review_ready",
          selection_count: 1,
          created_at: "2026-06-01T10:00:00Z",
          updated_at: "2026-06-01T10:05:00Z",
        },
        designs: [
          {
            id: "design-1",
            batch_id: "batch-1",
            item_id: "item-1",
            source_attempt_id: "attempt-1",
            target_group_key: "size:1200x1200",
            image_url: "https://cdn.example.com/design-1.png",
            review_status: "approved",
            created_at: "2026-06-01T10:01:00Z",
            updated_at: "2026-06-01T10:05:00Z",
          },
        ],
      },
    ],
    created_tasks: [
      {
        id: "task-1",
        title: "Task 1",
        design_id: "design-1",
        item_id: "item-1",
        selection_id: "selection-1",
        compatibility_fingerprint: "fp-1",
        status: "task_created",
        submission_state: "ready_to_submit",
        last_submission_action: "validated",
      },
    ],
    reused_tasks: [
      {
        id: "task-2",
        title: "Task 2",
        design_id: "design-2",
        item_id: "item-2",
        selection_id: "selection-2",
        compatibility_fingerprint: "fp-2",
        status: "draft_saved",
        submission_state: "draft_saved",
        last_submission_action: "save_draft",
      },
    ],
    rejected_tasks: [
      {
        design_id: "design-3",
        item_id: "item-3",
        selection_id: "selection-3",
        compatibility_fingerprint: "fp-3",
        status: "rejected",
        reason_code: "baseline_not_ready",
        message: "baseline 还没准备好",
      },
    ],
    failed_tasks: [
      {
        design_id: "design-4",
        item_id: "item-4",
        selection_id: "selection-4",
        compatibility_fingerprint: "fp-4",
        title: "Task 4",
        status: "failed",
        reason_code: "task_create_failed",
        message: "create timeout",
      },
    ],
    status_groups: {
      items: [
        { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
        { key: "submission_failed", label: "提交失败", count: 1, ids: ["design-4"] },
      ],
      by_key: {
        submittable: { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
        submission_failed: {
          key: "submission_failed",
          label: "提交失败",
          count: 1,
          ids: ["design-4"],
        },
      },
    },
  },
  expected: {
    batch: {
      id: "batch-1",
      status: "tasks_created",
      prompt: "botanical",
      styleCount: "3",
      sheinStoreId: 7,
    },
    items: [
      {
        item: {
          id: "item-1",
          batchId: "batch-1",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "1200 x 1200",
          status: "review_ready",
          selectionCount: 1,
        },
        designs: [
          {
            id: "design-1",
            batchId: "batch-1",
            itemId: "item-1",
            sourceAttemptId: "attempt-1",
            targetGroupKey: "size:1200x1200",
            imageUrl: "https://cdn.example.com/design-1.png",
            reviewStatus: "approved",
          },
        ],
      },
    ],
    createdTasks: [
      {
        id: "task-1",
        title: "Task 1",
        designId: "design-1",
        itemId: "item-1",
        selectionId: "selection-1",
        compatibilityFingerprint: "fp-1",
        status: "task_created",
        submissionState: "ready_to_submit",
        lastSubmissionAction: "validated",
        outcome: "created",
      },
    ],
    reusedTasks: [
      {
        id: "task-2",
        title: "Task 2",
        designId: "design-2",
        itemId: "item-2",
        selectionId: "selection-2",
        compatibilityFingerprint: "fp-2",
        status: "draft_saved",
        submissionState: "draft_saved",
        lastSubmissionAction: "save_draft",
        outcome: "reused",
      },
    ],
    rejectedTasks: [
      {
        designId: "design-3",
        itemId: "item-3",
        selectionId: "selection-3",
        compatibilityFingerprint: "fp-3",
        status: "rejected",
        reasonCode: "baseline_not_ready",
        message: "baseline 还没准备好",
        outcome: "rejected",
      },
    ],
    failedTasks: [
      {
        designId: "design-4",
        itemId: "item-4",
        selectionId: "selection-4",
        compatibilityFingerprint: "fp-4",
        title: "Task 4",
        status: "failed",
        reasonCode: "task_create_failed",
        message: "create timeout",
        outcome: "failed",
      },
    ],
    statusGroups: {
      items: [
        { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
        { key: "submission_failed", label: "提交失败", count: 1, ids: ["design-4"] },
      ],
      byKey: {
        submittable: { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
        submission_failed: {
          key: "submission_failed",
          label: "提交失败",
          count: 1,
          ids: ["design-4"],
        },
      },
    },
  },
} as const;
