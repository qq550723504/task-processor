import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

const SHEIN_DIAGNOSTIC_MAX_BYTES = 2 * 1024 * 1024;
export class InvalidSheinDiagnosticResponseError extends Error {}

/** Bound the bytes actually read, including errors and chunked responses. */
export async function readSheinDiagnosticJSON(response: Response, signal?: AbortSignal): Promise<unknown> {
  try {
    return await readBoundedStrictJSON(response, SHEIN_DIAGNOSTIC_MAX_BYTES, signal);
  } catch (error) {
    if (error instanceof InvalidStrictJSONResponseError) throw new InvalidSheinDiagnosticResponseError(error.message);
    throw error;
  }
}
