import type { ZodType } from "zod";

import { ApiError } from "@/lib/api/client";

export function parseApiResponseShape<T>(
  payload: unknown,
  schema: ZodType<T>,
  message: string,
): T {
  const result = schema.safeParse(payload);

  if (!result.success) {
    throw new ApiError(message, 502, {
      issues: result.error.issues.map((issue) => ({
        path: issue.path.join("."),
        message: issue.message,
      })),
    });
  }

  return result.data;
}
