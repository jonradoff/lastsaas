import { GoogleGenAI } from "@google/genai";
import type { TestRunResult, AIAssessment } from "./types.js";

const MODEL = "gemini-3-flash-preview";

// Approximate pricing for Gemini 2.5 Flash (per million tokens)
const INPUT_COST_PER_M = 0.15;
const OUTPUT_COST_PER_M = 0.60;

/**
 * Truncate raw MCP data to keep prompt under ~3K tokens.
 */
function truncateForPrompt(data: Record<string, unknown>, maxItems = 8): Record<string, unknown> {
  const truncated: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(data)) {
    if (Array.isArray(value)) {
      truncated[key] = value.slice(0, maxItems);
    } else if (typeof value === "object" && value !== null) {
      truncated[key] = truncateForPrompt(value as Record<string, unknown>, maxItems);
    } else if (typeof value === "string" && value.length > 500) {
      truncated[key] = value.slice(0, 500) + "...";
    } else {
      truncated[key] = value;
    }
  }
  return truncated;
}

function buildPrompt(rawData: Record<string, unknown>, scanResult: TestRunResult): string {
  const truncated = truncateForPrompt(rawData);

  const categorySummary = scanResult.categories
    .map((c) => `${c.category}: ${c.tested ? `${Math.round(c.cappedScore)}/100` : "not tested"}`)
    .join(", ");

  return `You are an e-commerce MCP server quality auditor. Analyze the following raw data from a Shopify MCP server scan and return a JSON assessment.

## Store Data (raw MCP responses, truncated)
${JSON.stringify(truncated, null, 2)}

## Tool Definitions
${JSON.stringify(scanResult.mappings, null, 2)}

## Deterministic Scan Scores
Composite: ${scanResult.compositeScore}/100
Categories: ${categorySummary}

## Instructions
Analyze this data from the perspective of an AI buyer agent trying to shop this store. Return ONLY a JSON object (no markdown fences, no explanation outside the JSON) with this exact schema:

{
  "overallQualityScore": <0-100 integer>,
  "productRelevance": {
    "score": <0-100>,
    "summary": "<1-2 sentences: would a buyer agent find satisfactory results?>"
  },
  "descriptionQuality": {
    "score": <0-100>,
    "summary": "<1-2 sentences: are descriptions useful for comparison or marketing fluff?>"
  },
  "dataCompleteness": {
    "score": <0-100>,
    "missingAttributes": ["<attribute>"],
    "summary": "<1-2 sentences: what structured attributes are missing for agent filtering?>"
  },
  "querySimulations": [
    {
      "query": "<realistic buyer query>",
      "wouldFindResult": <boolean>,
      "confidence": <0-100>,
      "explanation": "<1 sentence>"
    }
  ],
  "findings": [
    {
      "title": "<short title>",
      "category": "<relevance|description-quality|data-completeness|query-simulation|competitive>",
      "severity": "<high|medium|low>",
      "explanation": "<plain English, 1-3 sentences>",
      "revenueImpact": "<high|medium|low>",
      "fix": "<actionable recommendation with specific code/config if applicable>"
    }
  ],
  "competitiveComparison": "<paragraph comparing this store's agent readiness to typical best practices>"
}

Generate 3-5 query simulations with realistic buyer queries for this store's product category.
Generate 3-7 findings sorted by revenue impact (high first).
Be specific and actionable in fix recommendations — include Shopify Liquid code or GraphQL examples where relevant.`;
}

/**
 * Extract JSON from a response that might be wrapped in markdown code fences.
 */
function extractJson(text: string): string {
  const fenceMatch = text.match(/```(?:json)?\s*\n?([\s\S]*?)\n?\s*```/);
  if (fenceMatch) return fenceMatch[1].trim();

  const jsonStart = text.indexOf("{");
  const jsonEnd = text.lastIndexOf("}");
  if (jsonStart !== -1 && jsonEnd > jsonStart) {
    return text.slice(jsonStart, jsonEnd + 1);
  }

  return text;
}

/**
 * Run AI quality assessment using Gemini Flash.
 * Returns null if GEMINI_API_KEY is not set or the API call fails.
 */
export async function runAIAssessment(
  rawData: Record<string, unknown>,
  scanResult: TestRunResult,
  options: { verbose?: boolean; log?: (msg: string) => void } = {},
): Promise<AIAssessment | null> {
  const log = options.log ?? (() => {});

  const apiKey = process.env.GEMINI_API_KEY || process.env.GOOGLE_AI_API_KEY;
  if (!apiKey) {
    log("AI assessment skipped: GEMINI_API_KEY not set");
    return null;
  }

  const start = Date.now();

  try {
    const ai = new GoogleGenAI({ apiKey });

    const prompt = buildPrompt(rawData, scanResult);

    log("AI assessment: calling Gemini Flash...");
    const response = await ai.models.generateContent({
      model: MODEL,
      contents: prompt,
    });
    const text = response.text ?? "";

    const durationMs = Date.now() - start;

    const jsonStr = extractJson(text);
    const assessment = JSON.parse(jsonStr) as AIAssessment;

    // Attach metadata
    const inputTokens = response.usageMetadata?.promptTokenCount ?? 0;
    const outputTokens = response.usageMetadata?.candidatesTokenCount ?? 0;

    assessment.modelUsed = MODEL;
    assessment.tokenUsage = { input: inputTokens, output: outputTokens };
    assessment.costEstimateUsd = parseFloat(
      (
        (inputTokens / 1_000_000) * INPUT_COST_PER_M +
        (outputTokens / 1_000_000) * OUTPUT_COST_PER_M
      ).toFixed(6),
    );
    assessment.durationMs = durationMs;

    log(`AI assessment complete: quality score ${assessment.overallQualityScore}/100 ($${assessment.costEstimateUsd})`);
    return assessment;
  } catch (err) {
    const durationMs = Date.now() - start;
    log(`AI assessment failed after ${durationMs}ms: ${err instanceof Error ? err.message : String(err)}`);
    return null;
  }
}
