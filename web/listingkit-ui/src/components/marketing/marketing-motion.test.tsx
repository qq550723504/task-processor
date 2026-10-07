import { act } from "@testing-library/react";
import { useLayoutEffect, useRef } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";

vi.mock("motion/react", async (importOriginal) => ({
  ...await importOriginal<typeof import("motion/react")>(),
  useReducedMotion: () => true,
}));

import { MotionLayer } from "./marketing-motion";

it("hydrates the Figma server pose before applying a browser reduced-motion preference", async () => {
  const firstPaint: string[] = [];
  const hydrationErrors: unknown[] = [];
  function Probe() {
    const root = useRef<HTMLDivElement>(null);
    useLayoutEffect(() => {
      firstPaint.push(root.current?.querySelector<HTMLElement>('[data-node-id="243:205"]')?.style.opacity ?? "");
    }, []);
    return <div ref={root}><MotionLayer nodeId="243:205">Decorative light</MotionLayer></div>;
  }
  const container = document.createElement("div");
  container.innerHTML = renderToString(<Probe />);
  document.body.append(container);
  let root: ReturnType<typeof hydrateRoot> | undefined;
  try {
    await act(async () => {
      root = hydrateRoot(container, <Probe />, {
        onRecoverableError: (error) => hydrationErrors.push(error),
      });
    });
    expect(firstPaint[0]).toBe("0.45");
    expect(hydrationErrors).toEqual([]);
  } finally {
    await act(async () => root?.unmount());
    container.remove();
  }
});
