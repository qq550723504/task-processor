# SHEIN Gallery To SDS Task Design

## Goal

Allow a SHEIN style gallery image to be reused as an approved style design for an SDS-backed SHEIN ListingKit task.

The first version does not publish directly to SHEIN. It sends the selected gallery image into the existing SHEIN Studio workflow, where the user selects or confirms the SDS product variant, reviews the image, creates a ListingKit SHEIN task, and then submits through the existing task workspace.

## Scope

- Add a gallery action that imports one gallery image into SHEIN Studio.
- Preserve the existing SDS selection, review, task creation, and SHEIN submit flows.
- Validate that the gallery image aspect ratio is close to the selected SDS printable area or template ratio before creating SHEIN tasks.
- Keep legacy gallery items usable by requiring the user to select SDS data in Studio when the gallery item has no saved SDS selection.

## Flow

1. The user clicks `生成 SHEIN 任务` on a gallery card.
2. The gallery writes a small handoff payload to browser storage and navigates to SHEIN Studio.
3. If no SDS selection is in the URL, Studio opens on the SDS selection step.
4. Once an SDS selection exists, Studio imports the gallery image as a generated design, selects it by default, and opens the review step.
5. Before creating tasks, Studio compares the imported image ratio with the SDS target ratio.
6. Matching images can continue. Borderline images show a warning. Clearly mismatched images block task creation until the user chooses a better image or SDS style.

## Ratio Rule

The source ratio is `imageWidth / imageHeight`.

The SDS target ratio prefers `selection.printableWidth / selection.printableHeight`. If printable dimensions are unavailable, the UI can only show a warning because image dimensions for remote SDS surfaces are not always available synchronously.

Thresholds:

- `<= 15%` relative difference: pass.
- `> 15%` and `<= 25%`: warning, allow manual continuation.
- `> 25%`: blocking.

The task creation path must enforce the blocking threshold so users cannot bypass the check by navigating directly to the review step.

## Components

- Gallery page: shows the action and stores the handoff payload.
- Handoff utility: owns the local storage key, payload validation, and conversion to `SheinStudioGeneratedDesign`.
- Studio workbench: consumes the handoff after SDS selection is present, appends the design without replacing existing user work, selects it, and surfaces ratio warnings.
- Task creation guard: blocks creation when an imported gallery image has an incompatible SDS ratio.

## Testing

- Gallery page test: clicking the gallery action stores the handoff and navigates to Studio.
- Handoff utility tests: valid payload round-trips; expired or malformed payloads are ignored.
- Ratio utility tests: pass, warning, and blocking thresholds.
- Workbench test: a stored gallery handoff is imported into review once SDS selection exists.
- Workbench test: blocking ratio mismatch disables or prevents task creation.
