import { TestRunResult } from "../types.js";
import { generateJsonReport } from "./json-report.js";
import { renderHtmlTemplate } from "./report-template.js";
import fs from "fs";

export function generateHtmlReport(result: TestRunResult): string {
  return renderHtmlTemplate(result);
}

export function writeReport(result: TestRunResult, outputPath: string, format: "html" | "json"): void {
  const content = format === "json" ? generateJsonReport(result) : generateHtmlReport(result);
  fs.writeFileSync(outputPath, content, "utf-8");
}
