import { CapabilityName, CategoryName } from "./types.js";

export interface KeywordRule {
  capability: CapabilityName;
  nameKeywords: string[];
  descriptionKeywords: string[];
  requiredParamPatterns: string[];
}

export const KEYWORD_DICTIONARY: KeywordRule[] = [
  {
    capability: "search",
    nameKeywords: ["search", "find", "query", "browse", "list_products", "catalog", "search_shop_catalog", "shop_catalog"],
    descriptionKeywords: ["search", "find", "browse", "catalog", "query"],
    requiredParamPatterns: ["query", "keyword", "q", "search", "term"],
  },
  {
    capability: "get_detail",
    nameKeywords: ["get_product", "product_detail", "item_detail", "product_info", "get_item"],
    descriptionKeywords: ["detail", "information", "specific product"],
    requiredParamPatterns: ["id", "product_id", "item_id", "sku"],
  },
  {
    capability: "add_to_cart",
    nameKeywords: ["add_to_cart", "cart_add", "add_item", "create_cart", "update_cart", "get_cart"],
    descriptionKeywords: ["add", "cart"],
    requiredParamPatterns: ["product_id", "item_id", "id"],
  },
  {
    capability: "checkout",
    nameKeywords: ["checkout", "create_order", "place_order", "purchase"],
    descriptionKeywords: ["checkout", "order", "purchase"],
    requiredParamPatterns: [],
  },
];

export const CAPABILITY_CHECK_ORDER: CapabilityName[] = [
  "search",
  "get_detail",
  "add_to_cart",
  "checkout",
];

export const CATEGORY_WEIGHTS: Record<CategoryName, number> = {
  "data-quality": 0.35,
  "product-discovery": 0.30,
  "checkout-flow": 0.25,
  "protocol-compliance": 0.10,
};

export const SCORE_KILLERS = [
  { condition: "Products missing price data (>20%)", threshold: 0.20, category: "data-quality" as CategoryName, cap: 50 },
  { condition: "Search endpoint errors (>50%)", threshold: 0.50, category: "product-discovery" as CategoryName, cap: 40 },
  { condition: "MCP capability negotiation fails", threshold: 0, category: "protocol-compliance" as CategoryName, cap: 20 },
  { condition: "Request timeouts (>50% exceed 5s)", threshold: 0.50, category: "protocol-compliance" as CategoryName, cap: 10 },
];

export const LATENCY_THRESHOLDS = {
  ideal: 1000,
  acceptable: 3000,
  fail: 5000,
};

export const TOOL_CALL_TIMEOUT_MS = 10_000;
