import { expect, it } from "vitest";
import { consoleNavigation, findConsoleRoute } from "./console-navigation";
it("connects customization introduction and exact form/progress breadcrumbs", () => {
 for (const [path,label] of [["", "智能体定制"],["/new","提交需求"],["/progress","定制进度"]]) {
  const route=findConsoleRoute(`/workbench/agents/custom${path}`);
  expect(route?.node.availability).toBe("connected");
  expect(route?.trail.at(-1)?.label).toBe(label);
 }
 expect(findConsoleRoute("/workbench/agents/custom/evil")).toBeUndefined();
 expect(JSON.stringify(consoleNavigation)).not.toContain("/workbench/admin/agent-customization");
});
