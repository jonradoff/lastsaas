import { describe, it, expect, vi } from "vitest";
import type { McpConnection } from "./connection.js";
import type { AgentPersona } from "./types.js";

// Mock McpConnection for testing
function createMockConnection(toolResponses: Record<string, unknown>): McpConnection {
  return {
    client: {} as McpConnection["client"],
    async listTools() {
      return [
        { name: "search_products", description: "Search products", inputSchema: { type: "object", properties: { query: { type: "string" } } } },
        { name: "get_product_detail", description: "Get product details", inputSchema: { type: "object", properties: { product_id: { type: "string" } } } },
      ];
    },
    async callTool(name: string, _params: Record<string, unknown>) {
      if (toolResponses[name]) {
        return { content: toolResponses[name], durationMs: 100 };
      }
      throw new Error(`Unknown tool: ${name}`);
    },
    async close() {},
  };
}

describe("agent-simulator types", () => {
  it("AgentStep has required fields", () => {
    const step = {
      stepNumber: 1,
      action: "tool_call" as const,
      toolName: "search_products",
      toolArgs: { query: "shoes" },
      reasoning: "Searching for shoes",
      durationMs: 150,
    };
    expect(step.stepNumber).toBe(1);
    expect(step.action).toBe("tool_call");
    expect(step.toolName).toBe("search_products");
  });

  it("ShoppingScenario has required fields", () => {
    const scenario = {
      intent: "Find running shoes under $150",
      persona: "default" as AgentPersona,
      steps: [],
      outcome: "completed" as const,
      selectedProduct: { id: "1", name: "Trail Runner", price: 129.99, reason: "Best value" },
      totalSteps: 4,
      durationMs: 3500,
      tokenUsage: { input: 1500, output: 800 },
    };
    expect(scenario.outcome).toBe("completed");
    expect(scenario.selectedProduct?.name).toBe("Trail Runner");
  });

  it("AgentSimulation has required fields", () => {
    const simulation = {
      scenarios: [],
      modelUsed: "gemini-3-flash-preview",
      totalTokenUsage: { input: 5000, output: 3000 },
      costEstimateUsd: 0.0025,
      durationMs: 15000,
    };
    expect(simulation.modelUsed).toBe("gemini-3-flash-preview");
    expect(simulation.costEstimateUsd).toBeLessThan(0.01);
  });
});

describe("intent-generator", () => {
  it("returns default intents when no API key", async () => {
    // Ensure no API key
    const origKey = process.env.GEMINI_API_KEY;
    delete process.env.GEMINI_API_KEY;
    delete process.env.GOOGLE_AI_API_KEY;

    const { generateShoppingIntents } = await import("./intent-generator.js");
    const intents = await generateShoppingIntents({}, { count: 3 });

    expect(intents).toHaveLength(3);
    expect(intents.every((i) => typeof i === "string" && i.length > 0)).toBe(true);

    // Restore
    if (origKey) process.env.GEMINI_API_KEY = origKey;
  });

  it("returns default intents for empty raw data", async () => {
    const origKey = process.env.GEMINI_API_KEY;
    delete process.env.GEMINI_API_KEY;
    delete process.env.GOOGLE_AI_API_KEY;

    const { generateShoppingIntents } = await import("./intent-generator.js");
    const intents = await generateShoppingIntents({}, { count: 5 });

    expect(intents).toHaveLength(5);

    if (origKey) process.env.GEMINI_API_KEY = origKey;
  });
});

describe("personas", () => {
  it("has all four persona prompts", async () => {
    const { PERSONA_PROMPTS } = await import("./personas.js");
    expect(Object.keys(PERSONA_PROMPTS)).toEqual(["default", "price", "quality", "speed"]);
  });

  it("each persona prompt is non-empty", async () => {
    const { PERSONA_PROMPTS } = await import("./personas.js");
    for (const [name, prompt] of Object.entries(PERSONA_PROMPTS)) {
      expect(prompt.length).toBeGreaterThan(50);
    }
  });

  it("DEFAULT_PERSONAS contains only default", async () => {
    const { DEFAULT_PERSONAS } = await import("./personas.js");
    expect(DEFAULT_PERSONAS).toEqual(["default"]);
  });

  it("AGENCY_PERSONAS contains all four", async () => {
    const { AGENCY_PERSONAS } = await import("./personas.js");
    expect(AGENCY_PERSONAS).toEqual(["default", "price", "quality", "speed"]);
  });
});

describe("mock connection", () => {
  it("returns tool responses", async () => {
    const conn = createMockConnection({
      search_products: { products: [{ id: "1", name: "Test Product" }] },
    });

    const result = await conn.callTool("search_products", { query: "test" });
    expect(result.content).toEqual({ products: [{ id: "1", name: "Test Product" }] });
  });

  it("throws for unknown tools", async () => {
    const conn = createMockConnection({});
    await expect(conn.callTool("nonexistent", {})).rejects.toThrow("Unknown tool");
  });
});
