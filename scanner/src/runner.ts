import path from "path";
import { fileURLToPath } from "url";
import type { McpConnection } from "./connection.js";
import { mapCapabilities, type ToolDefinition } from "./capability-mapper.js";
import { loadScenarios } from "./scenario-loader.js";
import { evaluateAssertion } from "./assertion-engine.js";
import { computeCategoryScore, applyScoreKillers, computeCompositeScore } from "./scoring.js";
import { CATEGORY_WEIGHTS } from "./constants.js";
import type {
  CapabilityMapping,
  CapabilityName,
  CategoryName,
  CategoryResult,
  ScenarioResult,
  AssertionResult,
  TestRunResult,
  Scenario,
  ScenarioStatus,
} from "./types.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Resolve a "source" path like "search_results.products" against the variable map.
 * First part is the variable name (from setup save_as), second part is the field on that object.
 */
function resolveSource(
  source: string,
  variables: Record<string, unknown>,
): unknown {
  const parts = source.split(".");
  const varName = parts[0];
  let data = variables[varName];

  for (let i = 1; i < parts.length; i++) {
    if (data !== null && typeof data === "object" && !Array.isArray(data)) {
      data = (data as Record<string, unknown>)[parts[i]];
    } else if (Array.isArray(data)) {
      const idx = Number(parts[i]);
      if (!Number.isNaN(idx)) {
        data = data[idx];
      } else {
        return undefined;
      }
    } else {
      return undefined;
    }
  }

  return data;
}

/**
 * Main test orchestration function.
 */
export async function runTests(
  connection: McpConnection,
  options: {
    categoryFilter?: string[];
    scenariosDir?: string;
    verbose?: boolean;
    log?: (msg: string) => void;
    assess?: boolean;
    simulate?: boolean;
    personas?: string[];
  } = {},
): Promise<TestRunResult> {
  const log = options.log ?? console.log;
  const runStart = Date.now();

  // 1. Discover tools
  log("Discovering tools...");
  const tools = await connection.listTools();
  log(`Found ${tools.length} tools`);

  // 2. Map capabilities
  log("Mapping capabilities...");
  const mappings = mapCapabilities(tools as ToolDefinition[]);
  if (mappings.length === 0) {
    throw new Error("No capabilities could be mapped from the server's tools");
  }
  log(`Mapped ${mappings.length} capabilities: ${mappings.map((m) => `${m.capability} → ${m.toolName}`).join(", ")}`);

  const capabilityToTool: Record<string, string> = {};
  for (const m of mappings) {
    capabilityToTool[m.capability] = m.toolName;
  }

  // 3. Load scenarios
  const scenariosDir = options.scenariosDir ?? path.resolve(__dirname, "../scenarios");
  log(`Loading scenarios from ${scenariosDir}...`);
  const scenarios = loadScenarios(scenariosDir, options.categoryFilter);
  log(`Loaded ${scenarios.length} scenarios`);

  if (scenarios.length === 0) {
    throw new Error("No scenarios found");
  }

  // 4. Execute each scenario
  const scenarioResults: ScenarioResult[] = [];
  const allRawData: Record<string, unknown> = {}; // Collect raw MCP data for Layer 2

  for (const scenario of scenarios) {
    log(`Running scenario: ${scenario.name}...`);
    const scenarioStart = Date.now();

    // Check required capabilities
    const missingCaps = scenario.requires_capabilities.filter(
      (cap) => !capabilityToTool[cap],
    );
    if (missingCaps.length > 0) {
      log(`  Skipping: missing capabilities ${missingCaps.join(", ")}`);
      scenarioResults.push({
        scenario,
        status: "skipped",
        score: 0,
        assertions: [],
        skipReason: `Missing capabilities: ${missingCaps.join(", ")}`,
        durationMs: Date.now() - scenarioStart,
      });
      continue;
    }

    // Execute setup steps
    const variables: Record<string, unknown> = {};
    let setupFailed = false;

    for (const step of scenario.setup) {
      const toolName = capabilityToTool[step.action];
      if (!toolName) {
        setupFailed = true;
        break;
      }
      try {
        log(`  Setup: calling ${toolName} with ${JSON.stringify(step.params)}`);
        const result = await connection.callTool(toolName, step.params as Record<string, unknown>);
        if (step.save_as) {
          variables[step.save_as] = result.content;
          // Also store the duration for potential response_time assertions
          variables[`${step.save_as}_durationMs`] = result.durationMs;
        }
      } catch (err) {
        log(`  Setup failed: ${err instanceof Error ? err.message : String(err)}`);
        setupFailed = true;
        break;
      }
    }

    if (setupFailed) {
      scenarioResults.push({
        scenario,
        status: "setup-failed",
        score: 0,
        assertions: [],
        skipReason: "Setup step failed",
        durationMs: Date.now() - scenarioStart,
      });
      continue;
    }

    // Run assertion steps
    const assertionResults: AssertionResult[] = [];

    for (const step of scenario.steps) {
      const resolvedData = step.source ? resolveSource(step.source, variables) : undefined;
      const result = evaluateAssertion(step, resolvedData);

      // If a score killer is triggered, set the category from the scenario
      if (result.scoreKillerTriggered) {
        result.scoreKillerTriggered.category = scenario.category;
      }

      assertionResults.push(result);
    }

    // Compute scenario score (weighted average of assertion scores)
    const totalWeight = assertionResults.reduce((sum, r) => sum + r.assertion.score_weight, 0);
    const weightedScore =
      totalWeight > 0
        ? assertionResults.reduce((sum, r) => sum + r.score * r.assertion.score_weight, 0) / totalWeight
        : 0;
    const scenarioScore = Math.round(weightedScore);

    const allPassed = assertionResults.every((r) => r.passed);
    const status: ScenarioStatus = allPassed ? "passed" : "failed";

    scenarioResults.push({
      scenario,
      status,
      score: scenarioScore,
      assertions: assertionResults,
      durationMs: Date.now() - scenarioStart,
    });

    // Collect raw data for Layer 2 (prefix with scenario name to avoid collisions)
    for (const [key, value] of Object.entries(variables)) {
      allRawData[`${scenario.name}__${key}`] = value;
    }

    // Execute teardown (best effort)
    for (const step of scenario.teardown) {
      const toolName = capabilityToTool[step.action];
      if (!toolName) continue;
      try {
        await connection.callTool(toolName, step.params as Record<string, unknown>);
      } catch {
        // Best effort — ignore teardown errors
      }
    }
  }

  // 5. Group by category and compute scores
  const allCategories: CategoryName[] = [
    "data-quality",
    "product-discovery",
    "checkout-flow",
    "protocol-compliance",
  ];

  const categoryResults: CategoryResult[] = allCategories.map((category) => {
    const categoryScenarios = scenarioResults.filter(
      (r) => r.scenario.category === category,
    );

    const tested = categoryScenarios.some(
      (s) => s.status === "passed" || s.status === "failed",
    );

    const score = computeCategoryScore(categoryScenarios);

    // Collect score killers from assertion results
    const scoreKillers: Array<{ condition: string; cap: number }> = [];
    for (const sr of categoryScenarios) {
      for (const ar of sr.assertions) {
        if (ar.scoreKillerTriggered && ar.scoreKillerTriggered.category === category) {
          scoreKillers.push({
            condition: ar.message,
            cap: ar.scoreKillerTriggered.cap,
          });
        }
      }
    }

    const cappedScore = applyScoreKillers(score, scoreKillers);

    return {
      category,
      tested,
      score,
      cappedScore,
      weight: CATEGORY_WEIGHTS[category],
      effectiveWeight: 0, // Will be set by computeCompositeScore
      scenarios: categoryScenarios,
      scoreKillers,
    };
  });

  // 6. Compute composite score
  const { compositeScore, testedCategories } = computeCompositeScore(categoryResults);

  const durationMs = Date.now() - runStart;

  const result: TestRunResult = {
    serverIdentifier: "local",
    timestamp: Date.now(),
    durationMs,
    compositeScore,
    categories: categoryResults,
    partialResults: testedCategories.length < allCategories.length,
    partialReason:
      testedCategories.length < allCategories.length
        ? `Only ${testedCategories.length} of ${allCategories.length} categories tested`
        : undefined,
    mappings,
    version: "0.1.0",
    schemaVersion: "1.0",
    scenarioCount: scenarios.length,
    testedCategories,
  };

  // 7. AI Quality Assessment (optional)
  if (options.assess) {
    log("Running AI quality assessment...");
    try {
      const { runAIAssessment } = await import("./ai-assessor.js");
      const assessment = await runAIAssessment(allRawData, result, {
        verbose: options.verbose,
        log,
      });
      if (assessment) {
        result.aiAssessment = assessment;
        log(`AI assessment complete: quality score ${assessment.overallQualityScore}/100`);
      }
    } catch (err) {
      log(`AI assessment error: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  // 8. Agent Simulation (optional)
  if (options.simulate) {
    log("Running buyer agent simulation...");
    try {
      const { runAgentSimulation } = await import("./agent-simulator.js");
      const parsedPersonas = options.personas?.map(
        (p) => p as import("./types.js").AgentPersona,
      );
      const simulation = await runAgentSimulation(
        connection,
        capabilityToTool,
        tools as Array<{ name: string; description?: string; inputSchema?: { type: string; properties?: Record<string, unknown>; required?: string[] } }>,
        allRawData,
        {
          personas: parsedPersonas,
          verbose: options.verbose,
          log,
        },
      );
      if (simulation) {
        result.agentSimulation = simulation;
        log(`Simulation complete: ${simulation.scenarios.length} scenarios, $${simulation.costEstimateUsd}`);
      }
    } catch (err) {
      log(`Simulation error: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  return result;
}
