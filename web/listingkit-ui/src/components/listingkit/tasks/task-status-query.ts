export function shouldPollTaskResult(status?: string) {
  return (
    status === "pending" ||
    status === "processing" ||
    status === "queued" ||
    status === "running"
  );
}
