import { afterEach, beforeEach, it, expect, vi } from "vitest";
import {
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { TitleAgentTemplates } from "./title-agent-templates";
const f = vi.hoisted(() => ({ read: vi.fn() }));
vi.mock("@/lib/api/agent-configuration", async (original) => ({
  ...(await original<typeof import("@/lib/api/agent-configuration")>()),
  configurationRequest: f.read,
}));
const id = "11111111-1111-4111-8111-111111111111",
  scope = { userId: "actor", organizationId: "B" };
const template = {
  templateId: id,
  version: "1",
  targetPlatform: "amazon",
  name: "旧默认",
  lifecycle: "ACTIVE",
};
beforeEach(() => f.read.mockReset());
afterEach(cleanup);
it("loads exact default instead of moving to template head, and preserves explicit none", async () => {
  f.read
    .mockResolvedValueOnce({
      canUse: true,
      agent: { defaultTemplate: { templateId: id, revision: "1" } },
    })
    .mockResolvedValueOnce({
      items: [{ ...template, version: "2", name: "新版本" }],
      nextCursor: "",
    })
    .mockResolvedValueOnce(template);
  const choose = vi.fn(),
    ready = vi.fn();
  render(
    <TitleAgentTemplates scope={scope} onChoose={choose} onReady={ready} />,
  );
  await waitFor(() => expect(choose).toHaveBeenCalledWith(template));
  expect(f.read.mock.calls[2][1]).toBe(
    `product.title.agent/templates/${id}/revisions/1`,
  );
  expect(screen.getByLabelText("采用精确版本")).toHaveValue("1");
  fireEvent.change(screen.getByLabelText("标题模板"), {
    target: { value: "" },
  });
  expect(choose).toHaveBeenLastCalledWith(null);
  expect(ready).toHaveBeenLastCalledWith(true);
  expect(f.read).toHaveBeenCalledTimes(3);
});
it("requires explicit choice when the default version is unavailable", async () => {
  f.read
    .mockResolvedValueOnce({
      canUse: true,
      agent: { defaultTemplate: { templateId: id, revision: "1" } },
    })
    .mockResolvedValueOnce({ items: [], nextCursor: "" })
    .mockRejectedValueOnce(Error());
  const choose = vi.fn(),
    ready = vi.fn();
  render(
    <TitleAgentTemplates scope={scope} onChoose={choose} onReady={ready} />,
  );
  await screen.findByRole("alert");
  expect(choose).not.toHaveBeenCalled();
  expect(ready).toHaveBeenLastCalledWith(false);
  fireEvent.change(screen.getByLabelText("标题模板"), {
    target: { value: "" },
  });
  expect(choose).toHaveBeenLastCalledWith(null);
});
