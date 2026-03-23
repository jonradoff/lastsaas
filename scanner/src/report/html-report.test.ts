import { describe, it, expect } from "vitest";
import { generateHtmlReport } from "./html-report.js";
import { generateJsonReport } from "./json-report.js";
import { TestRunResult } from "../types.js";

const mockResult: TestRunResult = {
  serverIdentifier: "test",
  timestamp: Date.now(),
  durationMs: 2400,
  compositeScore: 65,
  categories: [
    {
      category: "data-quality", tested: true, score: 55, cappedScore: 50,
      weight: 0.35, effectiveWeight: 0.467,
      scenarios: [], scoreKillers: [{ condition: "23% missing prices", cap: 50 }],
    },
    {
      category: "product-discovery", tested: true, score: 75, cappedScore: 75,
      weight: 0.30, effectiveWeight: 0.4,
      scenarios: [], scoreKillers: [],
    },
    {
      category: "checkout-flow", tested: false, score: 0, cappedScore: 0,
      weight: 0.25, effectiveWeight: 0,
      scenarios: [], scoreKillers: [],
    },
    {
      category: "protocol-compliance", tested: true, score: 90, cappedScore: 90,
      weight: 0.10, effectiveWeight: 0.133,
      scenarios: [], scoreKillers: [],
    },
  ],
  partialResults: false,
  mappings: [],
  version: "0.1.0",
  schemaVersion: "1.0",
  scenarioCount: 15,
  testedCategories: ["data-quality", "product-discovery", "protocol-compliance"],
};

describe("generateHtmlReport", () => {
  it("produces valid HTML with score", () => {
    const html = generateHtmlReport(mockResult);
    expect(html).toContain("<!DOCTYPE html>");
    expect(html).toContain("65"); // composite score
    expect(html).toContain("schema_version"); // this should be in the embedded JSON
  });

  it("includes score killer alert", () => {
    const html = generateHtmlReport(mockResult);
    expect(html).toContain("23% missing prices");
  });

  it("shows untested categories", () => {
    const html = generateHtmlReport(mockResult);
    expect(html).toContain("checkout-flow");
    expect(html.toLowerCase()).toContain("not tested");
  });
});

describe("generateJsonReport", () => {
  it("produces valid JSON", () => {
    const json = generateJsonReport(mockResult);
    const parsed = JSON.parse(json);
    expect(parsed.compositeScore).toBe(65);
    expect(parsed.schemaVersion).toBe("1.0");
  });
});
