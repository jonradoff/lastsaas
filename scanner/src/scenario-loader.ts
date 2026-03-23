import { Ajv2020 } from "ajv/dist/2020.js";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import yaml from "js-yaml";
import type { Scenario, ScenarioSetupStep } from "./types.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const schemaPath = path.resolve(__dirname, "../scenario-schema.json");

// Lazily compiled validator
let _validate: ReturnType<InstanceType<typeof Ajv2020>["compile"]> | undefined;

function getValidator() {
  if (_validate) return _validate;

  const ajv = new Ajv2020({ allErrors: true });
  const schemaRaw = fs.readFileSync(schemaPath, "utf-8");
  const schema = JSON.parse(schemaRaw);
  _validate = ajv.compile(schema);
  return _validate;
}

/**
 * Validate a scenario object against the JSON schema plus custom rules.
 * Returns an array of error message strings (empty = valid).
 */
export function validateScenario(data: unknown): string[] {
  const validate = getValidator();
  const valid = validate(data);
  const errors: string[] = [];

  if (!valid && validate.errors) {
    for (const err of validate.errors) {
      const location = err.instancePath || "(root)";
      errors.push(`${location}: ${err.message ?? "unknown error"}`);
    }
  }

  // Custom rule: duplicate save_as names in setup/teardown
  if (typeof data === "object" && data !== null) {
    const record = data as Record<string, unknown>;

    for (const key of ["setup", "teardown"] as const) {
      const steps = record[key];
      if (!Array.isArray(steps)) continue;

      const seen = new Set<string>();
      for (const step of steps) {
        if (typeof step !== "object" || step === null) continue;
        const s = step as Record<string, unknown>;
        if (typeof s["save_as"] === "string") {
          const name = s["save_as"];
          if (seen.has(name)) {
            errors.push(`${key}: duplicate save_as name "${name}"`);
          } else {
            seen.add(name);
          }
        }
      }
    }
  }

  return errors;
}

/**
 * Load and validate all scenario YAML files from `scenariosDir`.
 * Optionally filter by category subdirectory names.
 * Throws if any scenario fails validation.
 */
export function loadScenarios(scenariosDir: string, categoryFilter?: string[]): Scenario[] {
  if (!fs.existsSync(scenariosDir)) {
    return [];
  }

  const categories = fs.readdirSync(scenariosDir).filter((entry) => {
    const fullPath = path.join(scenariosDir, entry);
    return fs.statSync(fullPath).isDirectory();
  });

  const filteredCategories = categoryFilter
    ? categories.filter((c) => categoryFilter.includes(c))
    : categories;

  const scenarios: Scenario[] = [];

  for (const category of filteredCategories) {
    const categoryDir = path.join(scenariosDir, category);
    const files = fs.readdirSync(categoryDir).filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"));

    for (const file of files) {
      const filePath = path.join(categoryDir, file);
      const raw = fs.readFileSync(filePath, "utf-8");
      const data = yaml.load(raw);

      const errors = validateScenario(data);
      if (errors.length > 0) {
        throw new Error(
          `Invalid scenario in ${filePath}:\n  ${errors.join("\n  ")}`
        );
      }

      // Cast is safe — we've validated against the schema
      const scenario = data as unknown as Scenario;

      // Ensure setup and teardown default to empty arrays if omitted
      if (!scenario.setup) {
        (scenario as Scenario & { setup: ScenarioSetupStep[] }).setup = [];
      }
      if (!scenario.teardown) {
        (scenario as Scenario & { teardown: ScenarioSetupStep[] }).teardown = [];
      }

      scenarios.push(scenario);
    }
  }

  return scenarios;
}
