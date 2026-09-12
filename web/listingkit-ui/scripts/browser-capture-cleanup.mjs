// Task acceptance cleanup only. Attempt every owned cleanup step and retain
// the original failure before any later cleanup failure.
export async function finishOwnedBrowserAndProxy({ failure, closeBrowser, verifyBrowser, closeProxy, record }) {
  const failures = failure ? [failure.error] : [];
  const result = { browserClosed: false, browserVerified: false, proxyClosed: false, primaryFailed: Boolean(failure) };
  for (const [name, action] of [["browserClosed", closeBrowser], ["browserVerified", verifyBrowser], ["proxyClosed", closeProxy]]) {
    try { await action(); result[name] = true; } catch (error) { failures.push(error); }
  }
  try { await record({ ...result, failureCount: failures.length }); } catch (error) { failures.push(error); }
  if (failures.length === 1) throw failures[0];
  if (failures.length > 1) throw new AggregateError(failures, "OWNED_BROWSER_ACCEPTANCE_OR_CLEANUP_FAILED");
  return result;
}
