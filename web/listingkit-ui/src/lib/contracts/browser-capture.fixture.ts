// Transport-only fixture. No provider or business acceptance evidence.
export const browserCaptureFixture = () => ({
  captureVersion: 1 as const,
  evidence: {
    schemaVersion: 1 as const,
    sourceURL: "https://detail.1688.com/offer/981645030344.html",
    offerID: "981645030344", title: "Browser fixture", description: null,
    attributes: [{ name: "material", value: "cotton" }], variants: [],
    priceFacts: [{ amount: "12.34000001", currency: null, minQuantity: "2" }],
    images: [], capturedAt: "2026-09-12T00:00:00.000Z",
    contentSHA256: "a".repeat(64), parserVersion: "1688-browser-dom/v1",
    warnings: [{ code: "MISSING_FACT", field: "priceFacts[0].currency" }],
    missingFacts: [{ field: "priceFacts[0].currency", reason: "not_observed" }],
  },
});
