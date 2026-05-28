import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import ListingKitSDSLoginPage from "@/app/listing-kits/sds-login/page";

vi.mock("@/components/listingkit/sds-login/sds-login-page", () => ({
  SdsLoginPage: () => <div>SDS login page</div>,
}));

describe("/listing-kits/sds-login page", () => {
  it("renders the SDS login page shell", () => {
    render(<ListingKitSDSLoginPage />);

    expect(screen.getByText("SDS login page")).toBeInTheDocument();
  });
});
