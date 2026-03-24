import { GoogleGenAI, Type } from "@google/genai";
import type { McpConnection } from "./connection.js";
import type { ToolDefinition } from "./capability-mapper.js";
import type {
  AgentSimulation,
  ShoppingScenario,
  AgentStep,
  AgentPersona,
  SelectedProduct,
  FailurePoint,
} from "./types.js";
import { PERSONA_PROMPTS, DEFAULT_PERSONAS } from "./personas.js";
import { generateShoppingIntents } from "./intent-generator.js";

const MODEL = "gemini-3-flash-preview";
const MAX_STEPS_PER_SCENARIO = 20;
const SCENARIO_TIMEOUT_MS = 60_000;

// Pricing for cost tracking
const INPUT_COST_PER_M = 0.15;
const OUTPUT_COST_PER_M = 0.60;

/**
 * Convert MCP tool definitions to Gemini function declarations.
 * Only includes tools we've mapped to capabilities (search, get_detail).
 * Cart/checkout tools are declared as "read-only evaluation" tools.
 */
function buildFunctionDeclarations(
  toolDefinitions: ToolDefinition[],
  capabilityToTool: Record<string, string>,
) {
  const mappedToolNames = new Set(Object.values(capabilityToTool));
  const declarations: Array<{
    name: string;
    description: string;
    parameters: Record<string, unknown>;
  }> = [];

  for (const tool of toolDefinitions) {
    if (!mappedToolNames.has(tool.name)) continue;

    // For cart/checkout tools, modify description to indicate read-only evaluation
    const isCartOrCheckout =
      capabilityToTool["add_to_cart"] === tool.name ||
      capabilityToTool["checkout"] === tool.name;

    const description = isCartOrCheckout
      ? `${tool.description ?? tool.name}. NOTE: Do NOT call this tool. Instead, evaluate whether the product data you have (product_id, variant_id, price, availableForSale) would allow a successful cart/checkout operation, and narrate your assessment.`
      : tool.description ?? tool.name;

    declarations.push({
      name: tool.name,
      description,
      parameters: tool.inputSchema ?? { type: "object", properties: {} },
    });
  }

  return declarations;
}

/**
 * Build the system prompt for a shopping scenario.
 */
function buildSystemPrompt(persona: AgentPersona, intent: string): string {
  return `${PERSONA_PROMPTS[persona]}

IMPORTANT RULES:
- You are testing a Shopify store's MCP endpoint to evaluate its AI agent readiness.
- Use the search and product detail tools to find and evaluate products.
- Do NOT actually call add_to_cart or checkout tools. Instead, when you're ready to "buy", evaluate the product data and narrate whether a real agent could complete the purchase.
- After finding and evaluating products, provide your final decision as a text response (not a tool call) in this exact JSON format:
{"decision": true, "product": {"id": "...", "name": "...", "price": 0, "reason": "..."}, "cartReadiness": "...", "checkoutReadiness": "..."}
- If you cannot find a suitable product or the data is insufficient, respond with:
{"decision": false, "failureReason": "...", "failureStep": "...", "suggestion": "..."}
- Keep your search focused. Don't make more than 3-4 tool calls.

SHOPPING TASK: ${intent}`;
}

/**
 * Run a single shopping scenario with the agent loop.
 */
async function runSingleScenario(
  ai: InstanceType<typeof GoogleGenAI>,
  connection: McpConnection,
  functionDeclarations: ReturnType<typeof buildFunctionDeclarations>,
  capabilityToTool: Record<string, string>,
  intent: string,
  persona: AgentPersona,
  options: { verbose?: boolean; log?: (msg: string) => void },
): Promise<ShoppingScenario> {
  const log = options.log ?? (() => {});
  const scenarioStart = Date.now();
  const steps: AgentStep[] = [];
  let totalInput = 0;
  let totalOutput = 0;

  const systemPrompt = buildSystemPrompt(persona, intent);

  // Only include read-only tools (search, get_detail) as actual callable tools
  const readOnlyTools = functionDeclarations.filter((d) => {
    const cartTool = capabilityToTool["add_to_cart"];
    const checkoutTool = capabilityToTool["checkout"];
    return d.name !== cartTool && d.name !== checkoutTool;
  });

  const contents: Array<{ role: string; parts: Array<Record<string, unknown>> }> = [
    { role: "user", parts: [{ text: systemPrompt }] },
  ];

  const tools = readOnlyTools.length > 0
    ? [{ functionDeclarations: readOnlyTools }]
    : undefined;

  let stepNumber = 0;

  try {
    while (stepNumber < MAX_STEPS_PER_SCENARIO) {
      if (Date.now() - scenarioStart > SCENARIO_TIMEOUT_MS) {
        steps.push({
          stepNumber: ++stepNumber,
          action: "failure",
          reasoning: "Scenario timed out after 60 seconds",
          durationMs: Date.now() - scenarioStart,
        });
        break;
      }

      const callStart = Date.now();
      const response = await ai.models.generateContent({
        model: MODEL,
        contents,
        ...(tools ? { config: { tools } } : {}),
      });

      totalInput += response.usageMetadata?.promptTokenCount ?? 0;
      totalOutput += response.usageMetadata?.candidatesTokenCount ?? 0;

      const functionCalls = response.functionCalls;

      if (functionCalls && functionCalls.length > 0) {
        // Agent wants to call a tool
        const fc = functionCalls[0];
        const toolName = fc.name ?? "unknown";
        const toolArgs = (fc.args ?? {}) as Record<string, unknown>;

        steps.push({
          stepNumber: ++stepNumber,
          action: "tool_call",
          toolName,
          toolArgs,
          reasoning: `Calling ${toolName} with ${JSON.stringify(toolArgs)}`,
          durationMs: Date.now() - callStart,
        });

        // Execute the tool call via MCP
        try {
          const toolStart = Date.now();
          const toolResult = await connection.callTool(toolName, toolArgs);

          steps.push({
            stepNumber: ++stepNumber,
            action: "tool_result",
            toolName,
            toolResult: toolResult.content,
            reasoning: `Received response from ${toolName}`,
            durationMs: Date.now() - toolStart,
          });

          // Feed result back to Gemini
          contents.push({
            role: "model",
            parts: [{ functionCall: fc }],
          });
          contents.push({
            role: "user",
            parts: [{
              functionResponse: {
                name: toolName,
                response: { result: toolResult.content },
                id: (fc as { id?: string }).id ?? "",
              },
            }],
          });
        } catch (err) {
          const failReason = err instanceof Error ? err.message : String(err);
          steps.push({
            stepNumber: ++stepNumber,
            action: "failure",
            toolName,
            reasoning: `Tool call failed: ${failReason}`,
            durationMs: Date.now() - callStart,
          });

          return {
            intent,
            persona,
            steps,
            outcome: "failed",
            failurePoint: {
              stepNumber,
              reason: `Tool call to ${toolName} failed: ${failReason}`,
              context: `Args: ${JSON.stringify(toolArgs)}`,
            },
            totalSteps: stepNumber,
            durationMs: Date.now() - scenarioStart,
            tokenUsage: { input: totalInput, output: totalOutput },
          };
        }
      } else {
        // Agent returned text — this is the decision
        const text = response.text ?? "";

        steps.push({
          stepNumber: ++stepNumber,
          action: "decision",
          reasoning: text,
          durationMs: Date.now() - callStart,
        });

        // Parse the decision JSON
        try {
          const jsonMatch = text.match(/\{[\s\S]*\}/);
          if (jsonMatch) {
            const decision = JSON.parse(jsonMatch[0]);

            if (decision.decision === true && decision.product) {
              return {
                intent,
                persona,
                steps,
                outcome: "completed",
                selectedProduct: {
                  id: String(decision.product.id ?? ""),
                  name: String(decision.product.name ?? ""),
                  price: Number(decision.product.price ?? 0),
                  reason: String(decision.product.reason ?? decision.cartReadiness ?? ""),
                },
                totalSteps: stepNumber,
                durationMs: Date.now() - scenarioStart,
                tokenUsage: { input: totalInput, output: totalOutput },
              };
            } else if (decision.decision === false) {
              return {
                intent,
                persona,
                steps,
                outcome: "failed",
                failurePoint: {
                  stepNumber,
                  reason: String(decision.failureReason ?? "Agent could not complete shopping task"),
                  context: String(decision.suggestion ?? ""),
                },
                totalSteps: stepNumber,
                durationMs: Date.now() - scenarioStart,
                tokenUsage: { input: totalInput, output: totalOutput },
              };
            }
          }
        } catch {
          // Decision text wasn't valid JSON — treat as narration
        }

        // If we got text but no parseable decision, the agent is done
        return {
          intent,
          persona,
          steps,
          outcome: "completed",
          totalSteps: stepNumber,
          durationMs: Date.now() - scenarioStart,
          tokenUsage: { input: totalInput, output: totalOutput },
        };
      }
    }
  } catch (err) {
    steps.push({
      stepNumber: ++stepNumber,
      action: "failure",
      reasoning: `Agent error: ${err instanceof Error ? err.message : String(err)}`,
      durationMs: Date.now() - scenarioStart,
    });
  }

  // If we hit max steps
  return {
    intent,
    persona,
    steps,
    outcome: "abandoned",
    failurePoint: {
      stepNumber: steps.length,
      reason: `Agent reached maximum step limit (${MAX_STEPS_PER_SCENARIO})`,
      context: "The agent may be stuck in a loop or the task is too complex",
    },
    totalSteps: steps.length,
    durationMs: Date.now() - scenarioStart,
    tokenUsage: { input: totalInput, output: totalOutput },
  };
}

/**
 * Run the full agent simulation: generate intents, run scenarios, collect results.
 */
export async function runAgentSimulation(
  connection: McpConnection,
  capabilityToTool: Record<string, string>,
  toolDefinitions: ToolDefinition[],
  rawData: Record<string, unknown>,
  options: {
    intents?: string[];
    personas?: AgentPersona[];
    maxStepsPerScenario?: number;
    verbose?: boolean;
    log?: (msg: string) => void;
  } = {},
): Promise<AgentSimulation | null> {
  const log = options.log ?? (() => {});

  const apiKey = process.env.GEMINI_API_KEY || process.env.GOOGLE_AI_API_KEY;
  if (!apiKey) {
    log("Agent simulation skipped: GEMINI_API_KEY not set");
    return null;
  }

  const start = Date.now();
  const ai = new GoogleGenAI({ apiKey });
  const personas = options.personas ?? DEFAULT_PERSONAS;
  const functionDeclarations = buildFunctionDeclarations(toolDefinitions, capabilityToTool);

  // Generate or use provided intents
  log("Generating shopping intents...");
  const intents = options.intents ?? await generateShoppingIntents(rawData, { count: 5, log });
  log(`Running ${intents.length} scenarios × ${personas.length} persona(s)...`);

  const allScenarios: ShoppingScenario[] = [];
  let totalInput = 0;
  let totalOutput = 0;

  for (const intent of intents) {
    for (const persona of personas) {
      log(`  Scenario: "${intent}" [${persona}]`);

      const scenario = await runSingleScenario(
        ai, connection, functionDeclarations, capabilityToTool,
        intent, persona, { verbose: options.verbose, log },
      );

      allScenarios.push(scenario);
      totalInput += scenario.tokenUsage.input;
      totalOutput += scenario.tokenUsage.output;

      const outcomeEmoji = scenario.outcome === "completed" ? "completed" : scenario.outcome;
      log(`    → ${outcomeEmoji} (${scenario.totalSteps} steps, ${scenario.durationMs}ms)`);
    }
  }

  const durationMs = Date.now() - start;
  const costEstimateUsd = parseFloat(
    (
      (totalInput / 1_000_000) * INPUT_COST_PER_M +
      (totalOutput / 1_000_000) * OUTPUT_COST_PER_M
    ).toFixed(4),
  );

  log(`Agent simulation complete: ${allScenarios.length} scenarios, $${costEstimateUsd}`);

  return {
    scenarios: allScenarios,
    modelUsed: MODEL,
    totalTokenUsage: { input: totalInput, output: totalOutput },
    costEstimateUsd,
    durationMs,
  };
}
