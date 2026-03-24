// --- Capability Mapping ---

export type CapabilityName = "search" | "get_detail" | "add_to_cart" | "checkout";

export type ConfidenceLevel = "high" | "low" | "none";

export interface CapabilityMapping {
  capability: CapabilityName;
  toolName: string;
  confidence: ConfidenceLevel;
}

export interface MappingCacheEntry {
  serverIdentifier: string;
  mappings: Record<CapabilityName, string>;
  timestamp: number;
}

// --- Scenarios ---

export interface ScenarioSetupStep {
  action: CapabilityName;
  params: Record<string, unknown>;
  save_as: string;
}

export interface ScenarioAssertion {
  assertion: AssertionType;
  source: string;
  field?: string;
  condition?: string;
  threshold?: number;
  severity: "high" | "medium" | "low";
  score_weight: number;
  on_fail?: "score_killer";
  score_killer_cap?: number;
  message?: string;
  expected_type?: string;
  schema?: Record<string, unknown>;
  min?: number;
  max?: number;
  min_length?: number;
  ideal?: number;
  acceptable?: number;
  fail?: number;
}

export type AssertionType =
  | "field_present"
  | "field_type"
  | "field_non_empty"
  | "each_item_has_field"
  | "percentage_threshold"
  | "response_time"
  | "status_code"
  | "schema_match"
  | "array_non_empty"
  | "url_format"
  | "value_range"
  | "value_positive";

export interface Scenario {
  name: string;
  description: string;
  category: CategoryName;
  requires_capabilities: CapabilityName[];
  setup: ScenarioSetupStep[];
  steps: ScenarioAssertion[];
  teardown: ScenarioSetupStep[];
}

// --- Scoring ---

export type CategoryName =
  | "data-quality"
  | "product-discovery"
  | "checkout-flow"
  | "protocol-compliance";

export interface AssertionResult {
  assertion: ScenarioAssertion;
  passed: boolean;
  score: number;
  message: string;
  details?: Record<string, unknown>;
  scoreKillerTriggered?: {
    category: CategoryName;
    cap: number;
  };
}

export type ScenarioStatus = "passed" | "failed" | "skipped" | "setup-failed" | "error";

export interface ScenarioResult {
  scenario: Scenario;
  status: ScenarioStatus;
  score: number;
  assertions: AssertionResult[];
  skipReason?: string;
  durationMs: number;
}

export interface CategoryResult {
  category: CategoryName;
  tested: boolean;
  score: number;
  cappedScore: number;
  weight: number;
  effectiveWeight: number;
  scenarios: ScenarioResult[];
  scoreKillers: Array<{ condition: string; cap: number }>;
}

export interface TestRunResult {
  serverIdentifier: string;
  timestamp: number;
  durationMs: number;
  compositeScore: number;
  categories: CategoryResult[];
  partialResults: boolean;
  partialReason?: string;
  mappings: CapabilityMapping[];
  version: string;
  schemaVersion: string;
  scenarioCount: number;
  testedCategories: CategoryName[];
  aiAssessment?: AIAssessment;
  agentSimulation?: AgentSimulation;
}

// --- AI Quality Assessment ---

export interface AIFinding {
  title: string;
  category: "relevance" | "description-quality" | "data-completeness" | "query-simulation" | "competitive";
  severity: "high" | "medium" | "low";
  explanation: string;
  revenueImpact: "high" | "medium" | "low";
  fix: string;
}

export interface AIQuerySimulation {
  query: string;
  wouldFindResult: boolean;
  confidence: number;
  explanation: string;
}

export interface AIAssessment {
  overallQualityScore: number;
  productRelevance: { score: number; summary: string };
  descriptionQuality: { score: number; summary: string };
  dataCompleteness: { score: number; missingAttributes: string[]; summary: string };
  querySimulations: AIQuerySimulation[];
  findings: AIFinding[];
  competitiveComparison: string;
  modelUsed: string;
  tokenUsage: { input: number; output: number };
  costEstimateUsd: number;
  durationMs: number;
}

// --- Agent Simulation ---

export type AgentPersona = "default" | "price" | "quality" | "speed";

export interface AgentStep {
  stepNumber: number;
  action: "think" | "tool_call" | "tool_result" | "decision" | "failure";
  toolName?: string;
  toolArgs?: Record<string, unknown>;
  toolResult?: unknown;
  reasoning: string;
  durationMs: number;
}

export interface SelectedProduct {
  id: string;
  name: string;
  price?: number;
  reason: string;
}

export interface FailurePoint {
  stepNumber: number;
  reason: string;
  context: string;
}

export interface ShoppingScenario {
  intent: string;
  persona: AgentPersona;
  steps: AgentStep[];
  outcome: "completed" | "failed" | "abandoned";
  selectedProduct?: SelectedProduct;
  failurePoint?: FailurePoint;
  totalSteps: number;
  durationMs: number;
  tokenUsage: { input: number; output: number };
}

export interface AgentSimulation {
  scenarios: ShoppingScenario[];
  modelUsed: string;
  totalTokenUsage: { input: number; output: number };
  costEstimateUsd: number;
  durationMs: number;
}

// --- CLI ---

export interface CliOptions {
  url?: string;
  command?: string;
  header?: string[];
  category?: string;
  nonInteractive?: boolean;
  failUnder?: number;
  format?: "html" | "json";
  out?: string;
  open?: boolean;
  verbose?: boolean;
  assess?: boolean;
  simulate?: boolean;
  personas?: string;
}
