import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";

function SearchProbe() {
  const searchParams = useLiveSearchParams();
  return <div>keyword:{searchParams.get("keyword") ?? ""}</div>;
}

describe("useLiveSearchParams", () => {
  it("hydrates from the browser location after mount", async () => {
    window.history.replaceState(null, "", "/listing-kits/shein?keyword=beer&page=1");

    render(<SearchProbe />);

    await waitFor(() =>
      expect(screen.getByText("keyword:beer")).toBeInTheDocument(),
    );
  });

  it("updates after replaceBrowserHistory changes the query string", async () => {
    window.history.replaceState(null, "", "/listing-kits/shein?keyword=beer&page=1");

    render(<SearchProbe />);

    await waitFor(() =>
      expect(screen.getByText("keyword:beer")).toBeInTheDocument(),
    );

    replaceBrowserHistory("/listing-kits/shein?keyword=cola&page=1");

    await waitFor(() =>
      expect(screen.getByText("keyword:cola")).toBeInTheDocument(),
    );
  });
});
