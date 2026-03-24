import { describe, it, expect } from "vitest";
import { validateScenario } from "./scenario-loader.js";

/**
 * Scanner integration tests.
 *
 * Tests marked with .skip require a running MCP server and are intended
 * to be run manually or in CI with real Shopify store endpoints.
 *
 * To enable: remove .skip and set SCANNER_INTEGRATION=1 in env.
 */

const INTEGRATION = process.env.SCANNER_INTEGRATION === "1";

describe("scenario validation", () => {
  it("accepts a valid scenario", () => {
    const valid = {
      name: "Basic product search",
      description: "Test searching for products",
      category: "product-discovery",
      requires_capabilities: ["search"],
      setup: [],
      steps: [
        {
          assertion: "array_non_empty",
          source: "search_result",
          severity: "high",
          score_weight: 1,
        },
      ],
      teardown: [],
    };
    const errors = validateScenario(valid);
    expect(errors).toHaveLength(0);
  });

  it("rejects scenario missing required fields", () => {
    const invalid = { name: "Missing fields" };
    const errors = validateScenario(invalid);
    expect(errors.length).toBeGreaterThan(0);
  });

  it("detects duplicate save_as in setup", () => {
    const scenario = {
      name: "Duplicate save_as",
      description: "Test duplicate detection",
      category: "data-quality",
      requires_capabilities: ["search"],
      setup: [
        { action: "search", params: { query: "shoes" }, save_as: "result1" },
        { action: "search", params: { query: "boots" }, save_as: "result1" },
      ],
      steps: [
        {
          assertion: "field_present",
          source: "result1",
          field: "name",
          severity: "medium",
          score_weight: 1,
        },
      ],
      teardown: [],
    };
    const errors = validateScenario(scenario);
    const dupError = errors.find((e) => e.includes("duplicate save_as"));
    expect(dupError).toBeDefined();
  });

  it("rejects invalid category", () => {
    const scenario = {
      name: "Bad category",
      description: "Test invalid category",
      category: "nonexistent-category",
      requires_capabilities: ["search"],
      setup: [],
      steps: [
        {
          assertion: "field_present",
          source: "test",
          field: "name",
          severity: "high",
          score_weight: 1,
        },
      ],
      teardown: [],
    };
    const errors = validateScenario(scenario);
    expect(errors.length).toBeGreaterThan(0);
  });

  it("rejects invalid assertion type", () => {
    const scenario = {
      name: "Bad assertion",
      description: "Test invalid assertion type",
      category: "data-quality",
      requires_capabilities: ["search"],
      setup: [],
      steps: [
        {
          assertion: "nonexistent_assertion",
          source: "test",
          severity: "high",
          score_weight: 1,
        },
      ],
      teardown: [],
    };
    const errors = validateScenario(scenario);
    expect(errors.length).toBeGreaterThan(0);
  });
});

// These tests actually hit real stores — only run with SCANNER_INTEGRATION=1
describe.skipIf(!INTEGRATION)("real store scans", () => {
  // NOTE: These would use the scanner runner to actually connect to MCP endpoints.
  // They're gated behind SCANNER_INTEGRATION to avoid hitting external APIs in CI.
  it.todo("allbirds.com scores 80+ (well-implemented MCP)");
  it.todo("colourpop.com scores below 30 (minimal MCP)");
  it.todo("gymshark.com scores 60+ (partial MCP)");
});
