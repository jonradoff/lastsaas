import { TestRunResult } from "../types.js";

export function generateJsonReport(result: TestRunResult): string {
  return JSON.stringify(result, null, 2);
}
