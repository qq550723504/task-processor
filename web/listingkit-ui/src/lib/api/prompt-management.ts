import { apiRequest } from "@/lib/api/client";
import type {
  PromptTemplate,
  PromptTemplateCatalogResponse,
  PromptTemplateListResponse,
  PromptTemplateSchema,
} from "@/lib/types/prompt-management";

export function listPromptTemplateCatalog() {
  return apiRequest<PromptTemplateCatalogResponse>("/prompts/catalog");
}

export function getPromptTemplateSchema(key: string) {
  return apiRequest<PromptTemplateSchema>(
    `/prompts/schema/${encodeURIComponent(key)}`,
  );
}

export function listPromptTemplates() {
  return apiRequest<PromptTemplateListResponse>("/prompts");
}

export function upsertPromptTemplate(body: PromptTemplate) {
  return apiRequest<PromptTemplate>("/prompts", {
    method: "PUT",
    body,
  });
}

export function setPromptTemplateStatus(key: string, enabled: boolean) {
  return apiRequest<{ key: string; enabled: boolean }>(
    `/prompts/${encodeURIComponent(key)}/status`,
    {
      method: "PATCH",
      body: { enabled },
    },
  );
}
