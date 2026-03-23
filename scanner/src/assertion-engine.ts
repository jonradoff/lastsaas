import { ScenarioAssertion, AssertionResult, CategoryName } from "./types.js";

// --- Helper functions ---

function getField(data: unknown, field?: string): unknown {
  if (field === undefined) return data;
  if (data !== null && typeof data === "object" && !Array.isArray(data)) {
    return (data as Record<string, unknown>)[field];
  }
  return undefined;
}

function checkCondition(value: unknown, condition?: string): boolean {
  if (!condition) {
    return value !== null && value !== undefined;
  }
  if (condition === "non_null_and_positive") {
    return typeof value === "number" && value > 0;
  }
  return value !== null && value !== undefined;
}

function isValidUrl(str: unknown): boolean {
  if (typeof str !== "string") return false;
  try {
    new URL(str);
    return true;
  } catch {
    return false;
  }
}

function interpolateMessage(template: string, vars: { fail_pct: number; fail_count: number }): string {
  return template
    .replace("{fail_pct}", String(vars.fail_pct))
    .replace("{fail_count}", String(vars.fail_count));
}

// --- Main evaluator ---

export function evaluateAssertion(assertion: ScenarioAssertion, data: unknown): AssertionResult {
  const { assertion: type } = assertion;

  switch (type) {
    case "field_present": {
      const value = getField(data, assertion.field);
      const passed = value !== null && value !== undefined;
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed
          ? `Field '${assertion.field}' is present`
          : `Field '${assertion.field}' is missing`,
      };
    }

    case "field_type": {
      const value = getField(data, assertion.field);
      const actualType = typeof value;
      const passed = actualType === assertion.expected_type;
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed
          ? `Field '${assertion.field}' has type '${assertion.expected_type}'`
          : `Field '${assertion.field}' expected type '${assertion.expected_type}' but got '${actualType}'`,
      };
    }

    case "field_non_empty": {
      const value = getField(data, assertion.field);
      const minLength = assertion.min_length ?? 1;
      const passed = typeof value === "string" && value.length >= minLength;
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed
          ? `Field '${assertion.field}' meets minimum length of ${minLength}`
          : `Field '${assertion.field}' does not meet minimum length of ${minLength}`,
      };
    }

    case "array_non_empty": {
      const passed = Array.isArray(data) && data.length > 0;
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed ? "Array is non-empty" : "Array is empty or data is not an array",
      };
    }

    case "each_item_has_field": {
      const arr = Array.isArray(data) ? (data as unknown[]) : [];
      const total = arr.length;
      let failCount = 0;
      for (const item of arr) {
        const value = getField(item, assertion.field);
        if (!checkCondition(value, assertion.condition)) {
          failCount++;
        }
      }
      const passCount = total - failCount;
      const score = total === 0 ? 0 : Math.round((passCount / total) * 100);
      const passed = failCount === 0;
      return {
        assertion,
        passed,
        score,
        message: passed
          ? `All ${total} items have field '${assertion.field}'`
          : `${failCount} of ${total} items failed field '${assertion.field}' check`,
        details: { failCount, total },
      };
    }

    case "percentage_threshold": {
      const arr = Array.isArray(data) ? (data as unknown[]) : [];
      const total = arr.length;
      let passCount = 0;
      for (const item of arr) {
        const value = getField(item, assertion.field);
        if (checkCondition(value, assertion.condition)) {
          passCount++;
        }
      }
      const passRate = total === 0 ? 0 : passCount / total;
      const threshold = (assertion.threshold ?? 100) / 100;
      const passed = passRate >= threshold;
      const score = passed ? 100 : Math.round(passRate * 100);

      const failCount = total - passCount;
      const fail_pct = Math.round((1 - passRate) * 100);

      let message: string;
      if (assertion.message) {
        message = interpolateMessage(assertion.message, { fail_pct, fail_count: failCount });
      } else {
        message = passed
          ? `${Math.round(passRate * 100)}% of items passed (threshold: ${assertion.threshold ?? 100}%)`
          : `Only ${Math.round(passRate * 100)}% of items passed (threshold: ${assertion.threshold ?? 100}%)`;
      }

      const result: AssertionResult = {
        assertion,
        passed,
        score,
        message,
        details: { passCount, failCount, total, passRate },
      };

      if (!passed && assertion.on_fail === "score_killer") {
        result.scoreKillerTriggered = {
          category: "data-quality" as CategoryName, // default category sentinel
          cap: assertion.score_killer_cap ?? 0,
        };
      }

      return result;
    }

    case "value_positive": {
      const value = getField(data, assertion.field);
      const passed = typeof value === "number" && value > 0;
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed
          ? `Field '${assertion.field}' is positive`
          : `Field '${assertion.field}' is not a positive number`,
      };
    }

    case "value_range": {
      const value = getField(data, assertion.field);
      const min = assertion.min ?? -Infinity;
      const max = assertion.max ?? Infinity;
      const passed = typeof value === "number" && value >= min && value <= max;
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed
          ? `Field '${assertion.field}' is within range [${min}, ${max}]`
          : `Field '${assertion.field}' is out of range [${min}, ${max}]`,
      };
    }

    case "url_format": {
      const value = getField(data, assertion.field);
      const passed = isValidUrl(value);
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed
          ? `Field '${assertion.field}' is a valid URL`
          : `Field '${assertion.field}' is not a valid URL`,
      };
    }

    case "response_time": {
      const latency = typeof data === "number" ? data : Infinity;
      const ideal = assertion.ideal ?? 1000;
      const acceptable = assertion.acceptable ?? 3000;
      const fail = assertion.fail ?? 5000;

      let score: number;
      if (latency <= ideal) {
        score = 100;
      } else if (latency >= fail) {
        score = 0;
      } else if (latency <= acceptable) {
        // linear from 100 to 50 between ideal and acceptable
        const ratio = (latency - ideal) / (acceptable - ideal);
        score = Math.round(100 - ratio * 50);
      } else {
        // linear from 50 to 0 between acceptable and fail
        const ratio = (latency - acceptable) / (fail - acceptable);
        score = Math.round(50 - ratio * 50);
      }

      const passed = latency <= ideal;
      return {
        assertion,
        passed,
        score,
        message: `Response time ${latency}ms (ideal: ${ideal}ms, acceptable: ${acceptable}ms, fail: ${fail}ms)`,
      };
    }

    case "status_code": {
      const passed = data !== "error";
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed ? "Status is not error" : "Status is error",
      };
    }

    case "schema_match": {
      const passed =
        data !== null && typeof data === "object" && !Array.isArray(data);
      return {
        assertion,
        passed,
        score: passed ? 100 : 0,
        message: passed ? "Data matches schema (is an object)" : "Data does not match schema (not an object)",
      };
    }

    default: {
      return {
        assertion,
        passed: false,
        score: 0,
        message: `Unknown assertion type: ${type}`,
      };
    }
  }
}
