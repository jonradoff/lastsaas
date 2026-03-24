import { describe, it, expect } from "vitest";
import { computeCategoryScore, applyScoreKillers, computeCompositeScore } from "./scoring.js";
import type { ScenarioResult, CategoryResult, CategoryName } from "./types.js";

function makeScenarioResult(overrides: Partial<ScenarioResult>): ScenarioResult {
  return {
    scenario: { name: "test", description: "", category: "data-quality", requires_capabilities: [], setup: [], steps: [], teardown: [] },
    status: "passed",
    score: 100,
    assertions: [],
    durationMs: 100,
    ...overrides,
  };
}

function makeCategoryResult(overrides: Partial<CategoryResult>): CategoryResult {
  return {
    category: "data-quality",
    tested: true,
    score: 80,
    cappedScore: 80,
    weight: 0.35,
    effectiveWeight: 0.35,
    scenarios: [],
    scoreKillers: [],
    ...overrides,
  };
}

describe("computeCategoryScore", () => {
  it("returns average of passed/failed scenario scores", () => {
    const scenarios = [
      makeScenarioResult({ status: "passed", score: 100 }),
      makeScenarioResult({ status: "failed", score: 60 }),
    ];
    expect(computeCategoryScore(scenarios)).toBe(80);
  });

  it("skips scenarios with skipped status", () => {
    const scenarios = [
      makeScenarioResult({ status: "passed", score: 100 }),
      makeScenarioResult({ status: "skipped", score: 0 }),
    ];
    expect(computeCategoryScore(scenarios)).toBe(100);
  });

  it("skips scenarios with setup-failed status", () => {
    const scenarios = [
      makeScenarioResult({ status: "passed", score: 80 }),
      makeScenarioResult({ status: "setup-failed", score: 0 }),
    ];
    expect(computeCategoryScore(scenarios)).toBe(80);
  });

  it("returns 0 for empty input", () => {
    expect(computeCategoryScore([])).toBe(0);
  });

  it("returns 0 when all scenarios are skipped", () => {
    const scenarios = [
      makeScenarioResult({ status: "skipped", score: 0 }),
      makeScenarioResult({ status: "setup-failed", score: 0 }),
    ];
    expect(computeCategoryScore(scenarios)).toBe(0);
  });
});

describe("applyScoreKillers", () => {
  it("returns score unchanged when no killers", () => {
    expect(applyScoreKillers(90, [])).toBe(90);
  });

  it("caps score to lowest killer cap", () => {
    const killers = [
      { condition: "missing prices", cap: 50 },
      { condition: "search errors", cap: 40 },
    ];
    expect(applyScoreKillers(90, killers)).toBe(40);
  });

  it("does not raise score above original", () => {
    const killers = [{ condition: "test", cap: 100 }];
    expect(applyScoreKillers(30, killers)).toBe(30);
  });

  it("caps to single killer value", () => {
    expect(applyScoreKillers(80, [{ condition: "test", cap: 20 }])).toBe(20);
  });
});

describe("computeCompositeScore", () => {
  it("returns 0 for no tested categories", () => {
    const categories = [
      makeCategoryResult({ tested: false }),
    ];
    const result = computeCompositeScore(categories);
    expect(result.compositeScore).toBe(0);
    expect(result.testedCategories).toHaveLength(0);
  });

  it("computes weighted average of tested categories", () => {
    const categories = [
      makeCategoryResult({ category: "data-quality", tested: true, cappedScore: 80, weight: 0.5 }),
      makeCategoryResult({ category: "product-discovery", tested: true, cappedScore: 60, weight: 0.5 }),
    ];
    const result = computeCompositeScore(categories);
    expect(result.compositeScore).toBe(70); // (80*0.5 + 60*0.5) / 1.0
    expect(result.testedCategories).toHaveLength(2);
  });

  it("scales weights proportionally for partial testing", () => {
    const categories = [
      makeCategoryResult({ category: "data-quality", tested: true, cappedScore: 100, weight: 0.35 }),
      makeCategoryResult({ category: "product-discovery", tested: false, cappedScore: 0, weight: 0.30 }),
    ];
    const result = computeCompositeScore(categories);
    // Only data-quality tested: effective weight = 0.35/0.35 = 1.0
    expect(result.compositeScore).toBe(100);
    expect(result.testedCategories).toEqual(["data-quality"]);
  });

  it("returns empty tested categories for no input", () => {
    const result = computeCompositeScore([]);
    expect(result.compositeScore).toBe(0);
    expect(result.testedCategories).toHaveLength(0);
  });
});
