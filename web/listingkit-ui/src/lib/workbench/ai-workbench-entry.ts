export type AIWorkbenchEntryCapabilities = {
  aiWorkbenchAvailable?: boolean;
  projectCenterAvailable?: boolean;
  knowledgeAvailable?: boolean;
  reportCenterAvailable?: boolean;
  productReviewAvailable?: boolean;
  sheinRecordsAvailable?: boolean;
};

// Keep the existing AI navigation order. A route never grants resource access.
export function firstConnectedAIEntry(capabilities: AIWorkbenchEntryCapabilities): string | null {
  if (capabilities.aiWorkbenchAvailable) return "/workbench/ai/chat";
  if (capabilities.projectCenterAvailable) return "/workbench/ai/projects";
  if (capabilities.knowledgeAvailable) return "/workbench/ai/knowledge";
  if (capabilities.reportCenterAvailable) return "/workbench/ai/reports";
  if (capabilities.productReviewAvailable) return "/workbench/ai/tasks/pending/other";
  if (capabilities.sheinRecordsAvailable) return "/workbench/ai/tasks/completed/history";
  // Packaged read-only guides require no optional provider/storage service.
  return "/workbench/ai/knowledge/official";
}
