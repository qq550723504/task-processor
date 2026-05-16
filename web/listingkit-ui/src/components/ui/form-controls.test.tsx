import { render, screen } from "@testing-library/react";

import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

describe("form controls", () => {
  it("renders shadcn styled text inputs", () => {
    render(<Input aria-label="Name" placeholder="Name" />);

    expect(screen.getByLabelText("Name")).toHaveClass("border-input");
    expect(screen.getByLabelText("Name")).toHaveClass("rounded-md");
  });

  it("renders textarea and select controls with the same field contract", () => {
    render(
      <>
        <Textarea aria-label="Prompt" />
        <Select aria-label="Mode">
          <option value="strict">Strict</option>
        </Select>
      </>,
    );

    expect(screen.getByLabelText("Prompt")).toHaveClass("border-input");
    expect(screen.getByLabelText("Mode")).toHaveClass("border-input");
  });

  it("renders labels and checkboxes", () => {
    render(
      <Label>
        <Checkbox aria-label="Enabled" />
        Enabled
      </Label>,
    );

    expect(screen.getByText("Enabled")).toHaveClass("text-sm");
    expect(screen.getByLabelText("Enabled")).toHaveClass("border-input");
  });
});
