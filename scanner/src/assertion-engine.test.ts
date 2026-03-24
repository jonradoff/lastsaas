import { describe, it, expect } from "vitest";
import { evaluateAssertion } from "./assertion-engine.js";
import type { ScenarioAssertion } from "./types.js";

function makeAssertion(overrides: Partial<ScenarioAssertion> & { assertion: ScenarioAssertion["assertion"] }): ScenarioAssertion {
  return {
    source: "test",
    severity: "medium",
    score_weight: 1,
    ...overrides,
  } as ScenarioAssertion;
}

describe("evaluateAssertion", () => {
  describe("field_present", () => {
    it("passes when field exists", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_present", field: "name" }),
        { name: "Test Product" }
      );
      expect(result.passed).toBe(true);
      expect(result.score).toBe(100);
    });

    it("fails when field is missing", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_present", field: "name" }),
        { price: 10 }
      );
      expect(result.passed).toBe(false);
      expect(result.score).toBe(0);
    });

    it("fails when field is null", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_present", field: "name" }),
        { name: null }
      );
      expect(result.passed).toBe(false);
      expect(result.score).toBe(0);
    });
  });

  describe("field_type", () => {
    it("passes when type matches", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_type", field: "price", expected_type: "number" }),
        { price: 29.99 }
      );
      expect(result.passed).toBe(true);
      expect(result.score).toBe(100);
    });

    it("fails when type mismatches", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_type", field: "price", expected_type: "number" }),
        { price: "29.99" }
      );
      expect(result.passed).toBe(false);
      expect(result.score).toBe(0);
    });
  });

  describe("field_non_empty", () => {
    it("passes for non-empty string", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_non_empty", field: "title" }),
        { title: "My Product" }
      );
      expect(result.passed).toBe(true);
    });

    it("fails for empty string", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_non_empty", field: "title" }),
        { title: "" }
      );
      expect(result.passed).toBe(false);
    });

    it("respects min_length", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "field_non_empty", field: "desc", min_length: 10 }),
        { desc: "short" }
      );
      expect(result.passed).toBe(false);
    });
  });

  describe("array_non_empty", () => {
    it("passes for non-empty array", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "array_non_empty" }),
        [1, 2, 3]
      );
      expect(result.passed).toBe(true);
    });

    it("fails for empty array", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "array_non_empty" }),
        []
      );
      expect(result.passed).toBe(false);
    });

    it("fails for non-array data", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "array_non_empty" }),
        { not: "array" }
      );
      expect(result.passed).toBe(false);
    });
  });

  describe("each_item_has_field", () => {
    it("passes when all items have the field", () => {
      const data = [{ name: "A" }, { name: "B" }, { name: "C" }];
      const result = evaluateAssertion(
        makeAssertion({ assertion: "each_item_has_field", field: "name" }),
        data
      );
      expect(result.passed).toBe(true);
      expect(result.score).toBe(100);
    });

    it("gives partial score when some items miss the field", () => {
      const data = [{ name: "A" }, { price: 10 }, { name: "C" }];
      const result = evaluateAssertion(
        makeAssertion({ assertion: "each_item_has_field", field: "name" }),
        data
      );
      expect(result.passed).toBe(false);
      expect(result.score).toBe(67); // 2/3 = 66.67 → rounds to 67
    });

    it("returns 0 for empty array", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "each_item_has_field", field: "name" }),
        []
      );
      expect(result.score).toBe(0);
    });
  });

  describe("percentage_threshold", () => {
    it("passes when threshold is met", () => {
      const data = [{ price: 10 }, { price: 20 }, { price: 30 }];
      const result = evaluateAssertion(
        makeAssertion({ assertion: "percentage_threshold", field: "price", threshold: 80 }),
        data
      );
      expect(result.passed).toBe(true);
      expect(result.score).toBe(100);
    });

    it("fails and triggers score killer", () => {
      const data = [{ price: 10 }, {}, {}];
      const result = evaluateAssertion(
        makeAssertion({
          assertion: "percentage_threshold",
          field: "price",
          threshold: 80,
          on_fail: "score_killer",
          score_killer_cap: 50,
        }),
        data
      );
      expect(result.passed).toBe(false);
      expect(result.scoreKillerTriggered).toBeDefined();
      expect(result.scoreKillerTriggered!.cap).toBe(50);
    });

    it("interpolates message template", () => {
      const data = [{ price: 10 }, {}];
      const result = evaluateAssertion(
        makeAssertion({
          assertion: "percentage_threshold",
          field: "price",
          threshold: 100,
          message: "{fail_count} of {fail_pct}% missing",
        }),
        data
      );
      expect(result.message).toBe("1 of 50% missing");
    });
  });

  describe("value_positive", () => {
    it("passes for positive number", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "value_positive", field: "price" }),
        { price: 5 }
      );
      expect(result.passed).toBe(true);
    });

    it("fails for zero", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "value_positive", field: "price" }),
        { price: 0 }
      );
      expect(result.passed).toBe(false);
    });

    it("fails for negative", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "value_positive", field: "price" }),
        { price: -5 }
      );
      expect(result.passed).toBe(false);
    });
  });

  describe("value_range", () => {
    it("passes when in range", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "value_range", field: "rating", min: 0, max: 5 }),
        { rating: 3.5 }
      );
      expect(result.passed).toBe(true);
    });

    it("fails when below range", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "value_range", field: "rating", min: 0, max: 5 }),
        { rating: -1 }
      );
      expect(result.passed).toBe(false);
    });

    it("passes at boundary", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "value_range", field: "rating", min: 0, max: 5 }),
        { rating: 5 }
      );
      expect(result.passed).toBe(true);
    });
  });

  describe("url_format", () => {
    it("passes for valid URL", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "url_format", field: "image" }),
        { image: "https://cdn.example.com/img.jpg" }
      );
      expect(result.passed).toBe(true);
    });

    it("fails for invalid URL", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "url_format", field: "image" }),
        { image: "not-a-url" }
      );
      expect(result.passed).toBe(false);
    });
  });

  describe("response_time", () => {
    it("scores 100 for ideal latency", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "response_time", ideal: 1000, acceptable: 3000, fail: 5000 }),
        500
      );
      expect(result.score).toBe(100);
      expect(result.passed).toBe(true);
    });

    it("scores 0 for latency at fail threshold", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "response_time", ideal: 1000, acceptable: 3000, fail: 5000 }),
        5000
      );
      expect(result.score).toBe(0);
    });

    it("scores between 50-100 for acceptable latency", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "response_time", ideal: 1000, acceptable: 3000, fail: 5000 }),
        2000 // halfway between ideal and acceptable
      );
      expect(result.score).toBeGreaterThan(50);
      expect(result.score).toBeLessThan(100);
    });
  });

  describe("status_code", () => {
    it("passes for non-error status", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "status_code" }),
        "success"
      );
      expect(result.passed).toBe(true);
    });

    it("fails for error status", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "status_code" }),
        "error"
      );
      expect(result.passed).toBe(false);
    });
  });

  describe("schema_match", () => {
    it("passes for object data", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "schema_match" }),
        { name: "test" }
      );
      expect(result.passed).toBe(true);
    });

    it("fails for array data", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "schema_match" }),
        [1, 2, 3]
      );
      expect(result.passed).toBe(false);
    });

    it("fails for null", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "schema_match" }),
        null
      );
      expect(result.passed).toBe(false);
    });
  });

  describe("unknown assertion type", () => {
    it("returns failed with message", () => {
      const result = evaluateAssertion(
        makeAssertion({ assertion: "nonexistent" as any }),
        {}
      );
      expect(result.passed).toBe(false);
      expect(result.score).toBe(0);
      expect(result.message).toContain("Unknown assertion type");
    });
  });
});
