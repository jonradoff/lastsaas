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
}
