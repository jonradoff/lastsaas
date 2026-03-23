import { CapabilityMapping, CapabilityName, ConfidenceLevel } from "./types.js";
import { KEYWORD_DICTIONARY, CAPABILITY_CHECK_ORDER, KeywordRule } from "./constants.js";

export interface ToolDefinition {
  name: string;
  description?: string;
  inputSchema?: {
    type: string;
    properties?: Record<string, unknown>;
    required?: string[];
  };
}

/**
 * Determines the confidence level for a tool matching a given capability rule.
 *
 * Confidence logic:
 * - "high": tool name contains a name keyword AND a param pattern is present in inputSchema properties
 * - "low": description contains a description keyword AND a param pattern is present (but name doesn't match),
 *          OR name matches but no param pattern present (for capabilities with no required params like checkout)
 * - "none": no meaningful match
 *
 * Special case: capabilities with no requiredParamPatterns (e.g., checkout) — name match alone → high.
 */
function classifyCandidate(
  tool: ToolDefinition,
  rule: KeywordRule
): ConfidenceLevel {
  const toolNameLower = tool.name.toLowerCase();
  const toolDescLower = (tool.description ?? "").toLowerCase();
  const propKeys = Object.keys(tool.inputSchema?.properties ?? {}).map((k) =>
    k.toLowerCase()
  );

  // Split tool name into segments (e.g., "search_products" → ["search", "products"])
  // A single-word keyword must match the FIRST segment (the leading verb/intent of the tool name)
  // so that "execute_query" does NOT match keyword "query", but "query_products" would.
  // Multi-word keywords (e.g., "list_products", "add_to_cart") are matched as substrings.
  const toolNameSegments = toolNameLower.split(/[_\-\s]+/);
  const nameMatch = rule.nameKeywords.some((kw) => {
    const kwLower = kw.toLowerCase();
    // Multi-word keywords: match as substring of tool name (e.g., "add_to_cart" in "add_to_cart_quick")
    if (kwLower.includes("_") || kwLower.includes("-")) {
      return toolNameLower === kwLower || toolNameLower.startsWith(kwLower);
    }
    // Single-word keywords: must be the first segment (the intent prefix)
    return toolNameSegments[0] === kwLower;
  });

  const descMatch = rule.descriptionKeywords.some((kw) =>
    toolDescLower.includes(kw.toLowerCase())
  );

  const paramMatch =
    rule.requiredParamPatterns.length === 0
      ? true // no param requirements → param condition is satisfied by default
      : rule.requiredParamPatterns.some((pattern) =>
          propKeys.some((key) => key.includes(pattern.toLowerCase()))
        );

  if (nameMatch && paramMatch) {
    return "high";
  }

  if ((descMatch && paramMatch) || (nameMatch && !paramMatch)) {
    return "low";
  }

  return "none";
}

/**
 * Maps an array of tool definitions to abstract capabilities using keyword matching.
 *
 * Algorithm:
 * 1. Iterate capabilities in CAPABILITY_CHECK_ORDER
 * 2. For each capability, score all unused tools
 * 3. Pick the best candidate (high > low, ties broken by list order)
 * 4. Mark the winning tool as used so it can't be claimed by another capability
 */
export function mapCapabilities(tools: ToolDefinition[]): CapabilityMapping[] {
  const usedTools = new Set<string>();
  const mappings: CapabilityMapping[] = [];

  for (const capabilityName of CAPABILITY_CHECK_ORDER) {
    const rule = KEYWORD_DICTIONARY.find((r) => r.capability === capabilityName);
    if (!rule) continue;

    let bestTool: ToolDefinition | null = null;
    let bestConfidence: ConfidenceLevel = "none";

    for (const tool of tools) {
      if (usedTools.has(tool.name)) continue;

      const confidence = classifyCandidate(tool, rule);
      if (confidence === "none") continue;

      // high beats low; first occurrence wins ties (preserves list order)
      if (bestTool === null || confidenceRank(confidence) > confidenceRank(bestConfidence)) {
        bestTool = tool;
        bestConfidence = confidence;
      }
    }

    if (bestTool !== null && bestConfidence !== "none") {
      usedTools.add(bestTool.name);
      mappings.push({
        capability: capabilityName,
        toolName: bestTool.name,
        confidence: bestConfidence,
      });
    }
  }

  return mappings;
}

function confidenceRank(confidence: ConfidenceLevel): number {
  switch (confidence) {
    case "high":
      return 2;
    case "low":
      return 1;
    case "none":
      return 0;
  }
}
