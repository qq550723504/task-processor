import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingSensitiveWordSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    word: z.string(),
    language: z.string(),
    tags: z.string().optional(),
    level: z.number(),
    replaceWord: z.string().optional(),
    remark: z.string().optional(),
    status: z.number(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const sensitiveWordPageSchema = z
  .object({
    items: z.array(listingSensitiveWordSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingSensitiveWord = z.infer<typeof listingSensitiveWordSchema>;
export type ListingSensitiveWordPage = z.infer<
  typeof sensitiveWordPageSchema
>;

export type ListingSensitiveWordQuery = Omit<QueueQuery, "status"> & {
  page?: number;
  page_size?: number;
  word?: string;
  language?: string;
  tags?: string;
  level?: number;
  status?: string;
  remark?: string;
};

export type ListingSensitiveWordInput = {
  word: string;
  language: string;
  tags?: string;
  level?: number;
  replaceWord?: string;
  remark?: string;
  status?: number;
};

export function parseSensitiveWordPageResponse(
  payload: unknown,
): ListingSensitiveWordPage {
  return parseApiResponseShape(
    payload,
    sensitiveWordPageSchema,
    "ListingKit API returned an unexpected sensitive word page response",
  );
}

export function parseSensitiveWordResponse(
  payload: unknown,
): ListingSensitiveWord {
  return parseApiResponseShape(
    payload,
    listingSensitiveWordSchema,
    "ListingKit API returned an unexpected sensitive word response",
  );
}

export async function getListingSensitiveWords(
  query: ListingSensitiveWordQuery = {},
): Promise<ListingSensitiveWordPage> {
  const payload = await apiRequest<unknown>("/admin/sensitive-words", {
    query,
  });
  return parseSensitiveWordPageResponse(payload);
}

export async function createListingSensitiveWord(
  input: ListingSensitiveWordInput,
): Promise<ListingSensitiveWord> {
  const payload = await apiRequest<unknown>("/admin/sensitive-words", {
    method: "POST",
    body: input,
  });
  return parseSensitiveWordResponse(payload);
}

export async function updateListingSensitiveWordStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingSensitiveWord> {
  const payload = await apiRequest<unknown>(
    `/admin/sensitive-words/${id}/status`,
    {
      method: "PATCH",
      body: { status, remark },
    },
  );
  return parseSensitiveWordResponse(payload);
}

export async function deleteListingSensitiveWord(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/sensitive-words/${id}`, {
    method: "DELETE",
  });
}
