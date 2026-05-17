"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  getPromptTemplateSchema,
  listPromptTemplateCatalog,
  listPromptTemplates,
  setPromptTemplateStatus,
  upsertPromptTemplate,
} from "@/lib/api/prompt-management";
import type { PromptTemplate } from "@/lib/types/prompt-management";

const listingKitPromptKeys = {
  all: ["listingkit", "prompts"] as const,
  catalog: ["listingkit", "prompts", "catalog"] as const,
  schema: (key: string) => ["listingkit", "prompts", "schema", key] as const,
};

export function usePromptTemplates() {
  return useQuery({
    queryKey: listingKitPromptKeys.all,
    queryFn: listPromptTemplates,
  });
}

export function usePromptTemplateCatalog() {
  return useQuery({
    queryKey: listingKitPromptKeys.catalog,
    queryFn: listPromptTemplateCatalog,
  });
}

export function usePromptTemplateSchema(key: string, enabled = true) {
  return useQuery({
    queryKey: listingKitPromptKeys.schema(key),
    queryFn: () => getPromptTemplateSchema(key),
    enabled: enabled && key.trim().length > 0,
  });
}

export function useUpsertPromptTemplate() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: PromptTemplate) => upsertPromptTemplate(request),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitPromptKeys.all });
      await client.invalidateQueries({ queryKey: listingKitPromptKeys.catalog });
    },
  });
}

export function useSetPromptTemplateStatus() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      setPromptTemplateStatus(key, enabled),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitPromptKeys.all });
      await client.invalidateQueries({ queryKey: listingKitPromptKeys.catalog });
    },
  });
}
