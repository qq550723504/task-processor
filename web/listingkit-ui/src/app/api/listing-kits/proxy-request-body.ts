export async function readProxyRequestBody(request: Request, timeoutMs: number) {
  const buffer = await withTimeout(
    request.arrayBuffer(),
    timeoutMs,
    "ListingKit proxy request body read timed out",
  );
  if (buffer.byteLength === 0) {
    return undefined;
  }
  return buffer;
}

async function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  message: string,
) {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<T>((_, reject) => {
        timeout = setTimeout(() => reject(new Error(message)), timeoutMs);
      }),
    ]);
  } finally {
    if (timeout) {
      clearTimeout(timeout);
    }
  }
}
