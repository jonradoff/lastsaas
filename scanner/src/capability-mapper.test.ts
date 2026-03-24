import { describe, it, expect } from "vitest";
import { mapCapabilities, type ToolDefinition } from "./capability-mapper.js";

describe("mapCapabilities", () => {
  it("maps a typical Shopify MCP tool set", () => {
    const tools: ToolDefinition[] = [
      {
        name: "search_products",
        description: "Search the product catalog",
        inputSchema: {
          type: "object",
          properties: { query: { type: "string" } },
          required: ["query"],
        },
      },
      {
        name: "get_product",
        description: "Get product details by ID",
        inputSchema: {
          type: "object",
          properties: { product_id: { type: "string" } },
          required: ["product_id"],
        },
      },
      {
        name: "add_to_cart",
        description: "Add an item to the cart",
        inputSchema: {
          type: "object",
          properties: { product_id: { type: "string" }, quantity: { type: "number" } },
          required: ["product_id"],
        },
      },
      {
        name: "checkout",
        description: "Create a checkout session",
        inputSchema: { type: "object", properties: {} },
      },
    ];

    const mappings = mapCapabilities(tools);

    expect(mappings).toHaveLength(4);

    const search = mappings.find((m) => m.capability === "search");
    expect(search).toBeDefined();
    expect(search!.toolName).toBe("search_products");
    expect(search!.confidence).toBe("high");

    const detail = mappings.find((m) => m.capability === "get_detail");
    expect(detail).toBeDefined();
    expect(detail!.toolName).toBe("get_product");
    expect(detail!.confidence).toBe("high");

    const cart = mappings.find((m) => m.capability === "add_to_cart");
    expect(cart).toBeDefined();
    expect(cart!.toolName).toBe("add_to_cart");
    expect(cart!.confidence).toBe("high");

    const checkout = mappings.find((m) => m.capability === "checkout");
    expect(checkout).toBeDefined();
    expect(checkout!.toolName).toBe("checkout");
    expect(checkout!.confidence).toBe("high");
  });

  it("returns empty mappings for unrelated tools", () => {
    const tools: ToolDefinition[] = [
      { name: "weather_forecast", description: "Get weather", inputSchema: { type: "object" } },
      { name: "send_email", description: "Send an email", inputSchema: { type: "object" } },
    ];

    const mappings = mapCapabilities(tools);
    expect(mappings).toHaveLength(0);
  });

  it("prevents a tool from being claimed by multiple capabilities", () => {
    // A tool named "search" should only be claimed once
    const tools: ToolDefinition[] = [
      {
        name: "search",
        description: "Search products and get details",
        inputSchema: {
          type: "object",
          properties: { query: { type: "string" }, id: { type: "string" } },
        },
      },
    ];

    const mappings = mapCapabilities(tools);
    // Should be claimed by "search" (first in check order)
    expect(mappings).toHaveLength(1);
    expect(mappings[0].capability).toBe("search");
  });

  it("matches description keywords with low confidence", () => {
    const tools: ToolDefinition[] = [
      {
        name: "execute_query",
        description: "Search the product catalog by keyword",
        inputSchema: {
          type: "object",
          properties: { keyword: { type: "string" } },
        },
      },
    ];

    const mappings = mapCapabilities(tools);
    const search = mappings.find((m) => m.capability === "search");
    expect(search).toBeDefined();
    expect(search!.confidence).toBe("low");
  });

  it("handles empty tool list", () => {
    expect(mapCapabilities([])).toHaveLength(0);
  });

  it("matches shop_catalog prefix for search", () => {
    const tools: ToolDefinition[] = [
      {
        name: "shop_catalog",
        description: "Browse shop catalog",
        inputSchema: {
          type: "object",
          properties: { search: { type: "string" } },
        },
      },
    ];

    const mappings = mapCapabilities(tools);
    const search = mappings.find((m) => m.capability === "search");
    expect(search).toBeDefined();
  });
});
