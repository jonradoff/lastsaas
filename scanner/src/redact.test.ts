import { describe, it, expect } from "vitest";
import { redactHeaders } from "./redact.js";

describe("redactHeaders", () => {
  it("redacts authorization header", () => {
    const result = redactHeaders({ Authorization: "Bearer secret123" });
    expect(result.Authorization).toBe("[REDACTED]");
  });

  it("redacts x-api-key header", () => {
    const result = redactHeaders({ "X-Api-Key": "key123" });
    expect(result["X-Api-Key"]).toBe("[REDACTED]");
  });

  it("redacts cookie header", () => {
    const result = redactHeaders({ Cookie: "session=abc" });
    expect(result.Cookie).toBe("[REDACTED]");
  });

  it("redacts token-containing headers", () => {
    const result = redactHeaders({ "X-Auth-Token": "tok123" });
    expect(result["X-Auth-Token"]).toBe("[REDACTED]");
  });

  it("preserves non-sensitive headers", () => {
    const result = redactHeaders({
      "Content-Type": "application/json",
      Accept: "text/html",
    });
    expect(result["Content-Type"]).toBe("application/json");
    expect(result.Accept).toBe("text/html");
  });

  it("handles mixed sensitive and non-sensitive headers", () => {
    const result = redactHeaders({
      "Content-Type": "application/json",
      Authorization: "Bearer secret",
      "X-Request-Id": "abc123",
    });
    expect(result["Content-Type"]).toBe("application/json");
    expect(result.Authorization).toBe("[REDACTED]");
    expect(result["X-Request-Id"]).toBe("abc123");
  });

  it("handles empty headers", () => {
    expect(redactHeaders({})).toEqual({});
  });
});
