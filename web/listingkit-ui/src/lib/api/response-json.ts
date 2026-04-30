export class ResponseJsonParseError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly responseText: string,
  ) {
    super(message);
  }
}

export async function parseJsonResponse<T>(response: Response): Promise<T | undefined> {
  const text = await response.text();
  if (!text) {
    return undefined;
  }

  try {
    return JSON.parse(text) as T;
  } catch {
    throw new ResponseJsonParseError(
      `Invalid JSON response: ${response.status}`,
      response.status,
      text,
    );
  }
}
