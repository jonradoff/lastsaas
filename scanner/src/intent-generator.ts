import { GoogleGenAI } from "@google/genai";

const MODEL = "gemini-3-flash-preview";

const DEFAULT_INTENTS = [
  "Find a popular product under $100",
  "Compare the top-rated items in the catalog",
  "Find a gift for someone who likes this store",
  "Search for the cheapest option in the most popular category",
  "Find a product with the most detailed specifications",
];

/**
 * Generate contextually relevant shopping intents from raw MCP data.
 * Falls back to default intents if Gemini is unavailable.
 */
export async function generateShoppingIntents(
  rawData: Record<string, unknown>,
  options: { count?: number; log?: (msg: string) => void } = {},
): Promise<string[]> {
  const log = options.log ?? (() => {});
  const count = options.count ?? 5;

  const apiKey = process.env.GEMINI_API_KEY || process.env.GOOGLE_AI_API_KEY;
  if (!apiKey) {
    log("Intent generator: no API key, using default intents");
    return DEFAULT_INTENTS.slice(0, count);
  }

  // Extract product samples from raw data for context
  const products: unknown[] = [];
  for (const value of Object.values(rawData)) {
    if (value && typeof value === "object" && "products" in (value as Record<string, unknown>)) {
      const arr = (value as Record<string, unknown>).products;
      if (Array.isArray(arr)) {
        products.push(...arr.slice(0, 5));
        break;
      }
    }
  }

  if (products.length === 0) {
    log("Intent generator: no product data found, using default intents");
    return DEFAULT_INTENTS.slice(0, count);
  }

  try {
    const ai = new GoogleGenAI({ apiKey });

    const response = await ai.models.generateContent({
      model: MODEL,
      contents: `You are generating shopping scenarios to test an e-commerce store's AI agent readiness.

Here are sample products from the store:
${JSON.stringify(products.slice(0, 5), null, 2)}

Generate exactly ${count} realistic shopping intents a buyer agent might have for this store. Each should be a single sentence that a customer would say to an AI shopping assistant. Mix these types:
1. Specific product search with price constraint
2. Comparison shopping between options
3. Gift buying scenario
4. Feature-specific search
5. Quick purchase of a popular item

Return ONLY a JSON array of strings. No explanation, no markdown fences.
Example: ["Find running shoes under $150", "Compare wireless earbuds by battery life"]`,
    });

    const text = (response.text ?? "").trim();
    const jsonStr = text.startsWith("[") ? text : text.slice(text.indexOf("["), text.lastIndexOf("]") + 1);
    const intents = JSON.parse(jsonStr) as string[];

    if (Array.isArray(intents) && intents.length > 0 && intents.every((i) => typeof i === "string")) {
      log(`Intent generator: generated ${intents.length} contextual intents`);
      return intents.slice(0, count);
    }
  } catch (err) {
    log(`Intent generator failed: ${err instanceof Error ? err.message : String(err)}`);
  }

  return DEFAULT_INTENTS.slice(0, count);
}
