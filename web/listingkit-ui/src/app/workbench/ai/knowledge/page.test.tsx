import {cleanup, render, screen} from "@testing-library/react";
import {afterEach, expect, it, vi} from "vitest";
import Page from "./page";
import MinePage from "./mine/page";
import OfficialPage from "./official/page";

vi.mock("@/components/workbench/knowledge/knowledge-page", () => ({KnowledgePage: () => <p>企业资料列表消费者</p>}));
vi.mock("@/components/workbench/knowledge/official-knowledge-page", () => ({OfficialKnowledgePage: () => <p>官方资料只读消费者</p>}));
afterEach(() => {cleanup(); vi.unstubAllEnvs();});

it("opens the Figma two-entry overview without loading enterprise documents or sample counters", () => {
  vi.stubEnv("LISTINGKIT_KNOWLEDGE_ENABLED", "true");
  render(<Page />);
  expect(screen.getByRole("heading", {name:"知识库"})).toBeVisible();
  expect(screen.getByRole("link", {name:"查看官方知识库 →"})).toHaveAttribute("href", "/workbench/ai/knowledge/official");
  expect(screen.getByRole("link", {name:"查看我的知识库 →"})).toHaveAttribute("href", "/workbench/ai/knowledge/mine");
  expect(screen.getByText("硕米维护 · 只读资料")).toBeVisible();
  expect(screen.queryByText("企业资料列表消费者")).not.toBeInTheDocument();
  expect(screen.queryByText(/12个可选|当前引用：0|尚未创建/)).not.toBeInTheDocument();
});

it.each([undefined, "false", "TRUE"])("does not advertise an absent enterprise knowledge module (%s)", enabled => {
  vi.stubEnv("LISTINGKIT_KNOWLEDGE_ENABLED", enabled);
  render(<Page />);
  expect(screen.getByText("企业知识库尚未开放")).toBeVisible();
  expect(screen.getByRole("link", {name:"查看官方知识库 →"})).toBeVisible();
  expect(screen.queryByRole("link", {name:"查看我的知识库 →"})).not.toBeInTheDocument();
});

it("uses the existing enterprise list only at My Knowledge and preserves its deployment gate", () => {
  vi.stubEnv("LISTINGKIT_KNOWLEDGE_ENABLED", "true");
  const view = render(<MinePage />);
  expect(screen.getByText("企业资料列表消费者")).toBeVisible();
  vi.stubEnv("LISTINGKIT_KNOWLEDGE_ENABLED", "false");
  view.rerender(<MinePage />);
  expect(screen.getByText("知识库尚未开放")).toBeVisible();
  expect(screen.queryByText("企业资料列表消费者")).not.toBeInTheDocument();
});

it("mounts official reading without enterprise documents", () => {
  render(<OfficialPage />);
  expect(screen.getByText("官方资料只读消费者")).toBeVisible();
  expect(screen.queryByText("企业资料列表消费者")).not.toBeInTheDocument();
});
