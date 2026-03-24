#!/usr/bin/env node

import path from "path";
import { fileURLToPath } from "url";
import { Command } from "commander";
import { connectStdio, connectSSE, connectHTTP } from "./connection.js";
import { runTests } from "./runner.js";
import { writeReport } from "./report/html-report.js";
import { CliOptions } from "./types.js";
import { redactHeaders } from "./redact.js";
import { exec } from "child_process";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const program = new Command();

program
  .name("mcplens")
  .description("Lighthouse for agent commerce — test your MCP server's agent readiness")
  .version("0.1.0");

program
  .command("test")
  .description("Run AgentLens tests against an MCP server")
  .option("--url <url>", "Remote MCP server URL (SSE)")
  .option("--command <cmd>", "Local MCP server command (stdio)")
  .option("--header <header...>", "HTTP header in Key: Value format (repeatable)")
  .option("--category <list>", "Comma-separated list of categories to test")
  .option("--non-interactive", "CI/CD mode — no prompts")
  .option("--fail-under <score>", "Exit code 1 if composite score is below this threshold", parseFloat)
  .option("--format <fmt>", 'Output format: "html" (default) or "json"', "html")
  .option("--out <path>", "Output file path")
  .option("--open", "Open report in default browser after generation")
  .option("--verbose", "Print interaction log to stderr")
  .option("--assess", "Run AI quality assessment (requires GEMINI_API_KEY)")
  .option("--simulate", "Run buyer agent simulation (requires GEMINI_API_KEY)")
  .option("--personas <list>", "Comma-separated agent personas: default,price,quality,speed")
  .action(async (opts: CliOptions & { nonInteractive?: boolean; failUnder?: number }) => {
    try {
      // 1. Validate: either --url or --command required (not both, not neither)
      if (!opts.url && !opts.command) {
        process.stderr.write("Error: either --url or --command is required\n");
        process.exit(2);
      }
      if (opts.url && opts.command) {
        process.stderr.write("Error: --url and --command are mutually exclusive\n");
        process.exit(2);
      }

      // 2. Parse headers from --header flags and AGENTLENS_HEADERS env var, merge them
      const headers: Record<string, string> = {};

      // Parse headers from --header flags (format: "Key: Value" or "Key:Value")
      if (opts.header && opts.header.length > 0) {
        for (const h of opts.header) {
          const colonIdx = h.indexOf(":");
          if (colonIdx === -1) {
            process.stderr.write(`Warning: ignoring malformed header (no colon): ${h}\n`);
            continue;
          }
          const key = h.slice(0, colonIdx).trim();
          const value = h.slice(colonIdx + 1).trim();
          headers[key] = value;
        }
      }

      // Parse headers from AGENTLENS_HEADERS env var (JSON format)
      const envHeaders = process.env.AGENTLENS_HEADERS;
      if (envHeaders) {
        try {
          const parsed = JSON.parse(envHeaders);
          if (typeof parsed === "object" && parsed !== null) {
            for (const [key, value] of Object.entries(parsed)) {
              if (typeof value === "string") {
                headers[key] = value;
              }
            }
          }
        } catch {
          process.stderr.write("Warning: failed to parse AGENTLENS_HEADERS as JSON — ignoring\n");
        }
      }

      // 3. If verbose, log headers (redacted) to stderr
      if (opts.verbose && Object.keys(headers).length > 0) {
        const redacted = redactHeaders(headers);
        process.stderr.write(`Headers: ${JSON.stringify(redacted)}\n`);
      }

      // 4. Connect via connectStdio or connectSSE
      if (opts.verbose) {
        process.stderr.write(opts.command ? `Connecting via stdio: ${opts.command}\n` : `Connecting via SSE: ${opts.url}\n`);
      }

      const connection = opts.command
        ? await connectStdio(opts.command)
        : await connectSSE(opts.url!, Object.keys(headers).length > 0 ? headers : undefined);


      try {
        // 5. Parse category filter from comma-separated string
        const categoryFilter = opts.category
          ? opts.category.split(",").map((c: string) => c.trim()).filter((c: string) => c.length > 0)
          : undefined;

        // 6. Call runTests with the connection
        const verboseLog = opts.verbose ? (msg: string) => process.stderr.write(msg + "\n") : undefined;

        const result = await runTests(connection, {
          categoryFilter,
          verbose: opts.verbose,
          log: verboseLog,
          assess: opts.assess,
          simulate: opts.simulate,
          personas: opts.personas?.split(",").map(p => p.trim()).filter(Boolean),
        });

        // 7. Write report to output file
        const format = (opts.format === "json" ? "json" : "html") as "html" | "json";
        const defaultOut = format === "json" ? "agentlens-report.json" : "agentlens-report.html";
        const outputPath = opts.out ?? defaultOut;

        writeReport(result, outputPath, format);

        // 8. Print summary to stdout
        process.stdout.write(`AgentLens Report: ${result.compositeScore}/100\n`);
        process.stdout.write(`Report saved to: ${outputPath}\n`);

        for (const category of result.categories) {
          if (!category.tested) {
            process.stdout.write(`  ${category.category}: not tested\n`);
          } else if (category.cappedScore < category.score) {
            process.stdout.write(`  ${category.category}: ${category.cappedScore}/100 (capped from ${category.score})\n`);
          } else {
            process.stdout.write(`  ${category.category}: ${category.score}/100\n`);
          }
        }

        // 9. If --open and format is html, open in browser (platform-aware)
        if (opts.open && format === "html") {
          const platform = process.platform;
          let openCmd: string;
          if (platform === "win32") {
            openCmd = `start "" "${outputPath}"`;
          } else if (platform === "darwin") {
            openCmd = `open "${outputPath}"`;
          } else {
            openCmd = `xdg-open "${outputPath}"`;
          }
          exec(openCmd, (err) => {
            if (err) {
              process.stderr.write(`Warning: failed to open report in browser: ${err.message}\n`);
            }
          });
        }

        // 10. Close connection
        await connection.close();

        // 11. If --fail-under is set and score is below, print message and exit 1
        if (opts.failUnder !== undefined && result.compositeScore < opts.failUnder) {
          process.stderr.write(
            `Score ${result.compositeScore} is below --fail-under threshold of ${opts.failUnder}\n`
          );
          process.exit(1);
        }

        // Exit 0 on success
        process.exit(0);
      } catch (err) {
        // Close connection on error (best effort)
        try {
          await connection.close();
        } catch {
          // ignore
        }
        throw err;
      }
    } catch (err) {
      process.stderr.write(`Error: ${err instanceof Error ? err.message : String(err)}\n`);
      process.exit(2);
    }
  });

program
  .command("scan <domains...>")
  .description("Scan one or more Shopify stores (https://{domain}/api/mcp). Pass --compare when scanning multiple domains to generate a side-by-side comparison.")
  .option("--header <header...>", "HTTP header in Key: Value format (repeatable)")
  .option("--compare", "Generate a side-by-side comparison when multiple domains are provided")
  .option("--format <fmt>", 'Output format: "html" (default) or "json"', "html")
  .option("--out <path>", "Output file path (single domain) or directory (multiple domains)")
  .option("--open", "Open report in default browser after generation")
  .option("--verbose", "Print interaction log to stderr")
  .option("--fail-under <score>", "Exit code 1 if composite score is below this threshold", parseFloat)
  .option("--assess", "Run AI quality assessment (requires GEMINI_API_KEY)")
  .option("--simulate", "Run buyer agent simulation (requires GEMINI_API_KEY)")
  .option("--personas <list>", "Comma-separated agent personas: default,price,quality,speed")
  .action(async (
    domains: string[],
    opts: { header?: string[]; compare?: boolean; format?: string; out?: string; open?: boolean; verbose?: boolean; failUnder?: number; assess?: boolean; simulate?: boolean; personas?: string }
  ) => {
    try {
      // Parse headers from --header flags
      const headers: Record<string, string> = {};
      if (opts.header && opts.header.length > 0) {
        for (const h of opts.header) {
          const colonIdx = h.indexOf(":");
          if (colonIdx === -1) {
            process.stderr.write(`Warning: ignoring malformed header (no colon): ${h}\n`);
            continue;
          }
          const key = h.slice(0, colonIdx).trim();
          const value = h.slice(colonIdx + 1).trim();
          headers[key] = value;
        }
      }
      const headersArg = Object.keys(headers).length > 0 ? headers : undefined;

      // Use Shopify-specific scenarios for the scan command
      const shopifyScenariosDir = path.resolve(__dirname, "../scenarios/shopify");
      const format = (opts.format === "json" ? "json" : "html") as "html" | "json";

      // --- Single domain (original behaviour) ---
      if (domains.length === 1) {
        const domain = domains[0];
        const url = `https://${domain}/api/mcp`;
        if (opts.verbose) {
          process.stderr.write(`Scanning Shopify store: ${domain}\n`);
          process.stderr.write(`Connecting via Streamable HTTP: ${url}\n`);
        }

        const connection = await connectHTTP(url, headersArg);
        try {
          const verboseLog = opts.verbose ? (msg: string) => process.stderr.write(msg + "\n") : undefined;
          const result = await runTests(connection, { verbose: opts.verbose, log: verboseLog, scenariosDir: shopifyScenariosDir, assess: opts.assess, simulate: opts.simulate, personas: opts.personas?.split(",").map(p => p.trim()).filter(Boolean) });

          const safeLabel = domain.replace(/[^a-z0-9.-]/gi, "_");
          const defaultOut = format === "json" ? `mcplens-${safeLabel}.json` : `mcplens-${safeLabel}.html`;
          const outputPath = opts.out ?? defaultOut;

          writeReport(result, outputPath, format);

          process.stdout.write(`McpLens Report — ${domain}: ${result.compositeScore}/100\n`);
          process.stdout.write(`Report saved to: ${outputPath}\n`);

          for (const category of result.categories) {
            if (!category.tested) {
              process.stdout.write(`  ${category.category}: not tested\n`);
            } else if (category.cappedScore < category.score) {
              process.stdout.write(`  ${category.category}: ${category.cappedScore}/100 (capped from ${category.score})\n`);
            } else {
              process.stdout.write(`  ${category.category}: ${category.score}/100\n`);
            }
          }

          if (opts.open && format === "html") {
            const platform = process.platform;
            const openCmd = platform === "win32" ? `start "" "${outputPath}"` : platform === "darwin" ? `open "${outputPath}"` : `xdg-open "${outputPath}"`;
            exec(openCmd, (err) => {
              if (err) process.stderr.write(`Warning: failed to open report: ${err.message}\n`);
            });
          }

          await connection.close();

          if (opts.failUnder !== undefined && result.compositeScore < opts.failUnder) {
            process.stderr.write(`Score ${result.compositeScore} is below --fail-under threshold of ${opts.failUnder}\n`);
            process.exit(1);
          }
          process.exit(0);
        } catch (err) {
          try { await connection.close(); } catch { /* ignore */ }
          throw err;
        }
      }

      // --- Multiple domains (compare mode) ---
      process.stdout.write(`Scanning ${domains.length} domains...\n`);

      type DomainResult = { domain: string; result: import("./types.js").TestRunResult };
      const results: DomainResult[] = [];

      for (const domain of domains) {
        const url = `https://${domain}/api/mcp`;
        process.stdout.write(`  Scanning ${domain}...\n`);
        if (opts.verbose) {
          process.stderr.write(`Connecting via Streamable HTTP: ${url}\n`);
        }

        const connection = await connectHTTP(url, headersArg);
        try {
          const verboseLog = opts.verbose ? (msg: string) => process.stderr.write(msg + "\n") : undefined;
          const result = await runTests(connection, { verbose: opts.verbose, log: verboseLog, scenariosDir: shopifyScenariosDir, assess: opts.assess, simulate: opts.simulate, personas: opts.personas?.split(",").map(p => p.trim()).filter(Boolean) });
          results.push({ domain, result });
          process.stdout.write(`  ${domain}: ${result.compositeScore}/100\n`);
          await connection.close();
        } catch (err) {
          try { await connection.close(); } catch { /* ignore */ }
          process.stderr.write(`  Warning: failed to scan ${domain}: ${err instanceof Error ? err.message : String(err)}\n`);
        }
      }

      if (results.length === 0) {
        process.stderr.write("Error: all domain scans failed\n");
        process.exit(2);
      }

      if (format === "json") {
        // Output an array of results, each annotated with domain
        const outputPath = opts.out ?? "mcplens-compare.json";
        const fs = await import("fs");
        fs.writeFileSync(outputPath, JSON.stringify(results.map(r => ({ domain: r.domain, ...r.result })), null, 2), "utf-8");
        process.stdout.write(`Comparison JSON saved to: ${outputPath}\n`);
      } else {
        // Build HTML with comparison table prepended
        const outputPath = opts.out ?? "mcplens-compare.html";
        const htmlContent = buildCompareReport(results, opts.compare ?? false);
        const fs = await import("fs");
        fs.writeFileSync(outputPath, htmlContent, "utf-8");
        process.stdout.write(`Comparison report saved to: ${outputPath}\n`);

        if (opts.open) {
          const platform = process.platform;
          const openCmd = platform === "win32" ? `start "" "${outputPath}"` : platform === "darwin" ? `open "${outputPath}"` : `xdg-open "${outputPath}"`;
          exec(openCmd, (err) => {
            if (err) process.stderr.write(`Warning: failed to open report: ${err.message}\n`);
          });
        }
      }

      // --fail-under: check if any domain is below threshold
      if (opts.failUnder !== undefined) {
        const worst = results.reduce((a, b) => a.result.compositeScore < b.result.compositeScore ? a : b);
        if (worst.result.compositeScore < opts.failUnder) {
          process.stderr.write(`Score ${worst.result.compositeScore} (${worst.domain}) is below --fail-under threshold of ${opts.failUnder}\n`);
          process.exit(1);
        }
      }

      process.exit(0);
    } catch (err) {
      process.stderr.write(`Error: ${err instanceof Error ? err.message : String(err)}\n`);
      process.exit(2);
    }
  });

// ---- Batch command ----

program
  .command("batch")
  .description("Scan multiple domains from a text file (one per line) and write individual JSON results to an output directory")
  .requiredOption("--domains <file>", "Path to a text file with one domain per line")
  .option("--output <dir>", "Directory to write JSON result files", "results")
  .option("--delay <ms>", "Milliseconds to wait between scans", "1000")
  .option("--verbose", "Print interaction log to stderr")
  .option("--fail-under <score>", "Exit code 1 if any domain's score is below this threshold", parseFloat)
  .action(async (opts: { domains: string; output: string; delay: string; verbose?: boolean; failUnder?: number }) => {
    const fs = await import("fs");
    const nodePath = await import("path");

    // Read domain list
    if (!fs.existsSync(opts.domains)) {
      process.stderr.write(`Error: domains file not found: ${opts.domains}\n`);
      process.exit(2);
    }
    const rawLines = fs.readFileSync(opts.domains, "utf-8").split(/\r?\n/);
    const domains = rawLines
      .map((l: string) => l.trim())
      .filter((l: string) => l.length > 0 && !l.startsWith("#"));

    if (domains.length === 0) {
      process.stderr.write("Error: no domains found in file\n");
      process.exit(2);
    }

    // Ensure output directory exists
    fs.mkdirSync(opts.output, { recursive: true });

    const delayMs = parseInt(opts.delay, 10) || 1000;
    const shopifyScenariosDir = path.resolve(__dirname, "../scenarios/shopify");

    process.stdout.write(`Batch scanning ${domains.length} domains → ${opts.output}/\n`);

    let failed = 0;
    let worstScore = Infinity;

    for (let i = 0; i < domains.length; i++) {
      const domain = domains[i];
      process.stdout.write(`[${i + 1}/${domains.length}] ${domain}... `);

      if (i > 0 && delayMs > 0) {
        await new Promise<void>((resolve) => setTimeout(resolve, delayMs));
      }

      const url = `https://${domain}/api/mcp`;
      try {
        const connection = await connectHTTP(url);
        const verboseLog = opts.verbose ? (msg: string) => process.stderr.write(msg + "\n") : undefined;
        const result = await runTests(connection, { verbose: opts.verbose, log: verboseLog, scenariosDir: shopifyScenariosDir });
        await connection.close();

        const safeLabel = domain.replace(/[^a-z0-9.-]/gi, "_");
        const outFile = nodePath.join(opts.output, `${safeLabel}.json`);
        const payload = JSON.stringify({ domain, ...result }, null, 2);
        fs.writeFileSync(outFile, payload, "utf-8");

        process.stdout.write(`${result.compositeScore}/100 → ${outFile}\n`);
        if (result.compositeScore < worstScore) worstScore = result.compositeScore;
      } catch (err) {
        process.stdout.write(`FAILED\n`);
        process.stderr.write(`  Error scanning ${domain}: ${err instanceof Error ? err.message : String(err)}\n`);
        failed++;
      }
    }

    process.stdout.write(`\nBatch complete: ${domains.length - failed} succeeded, ${failed} failed\n`);

    if (opts.failUnder !== undefined && worstScore < opts.failUnder) {
      process.stderr.write(`Worst score ${worstScore} is below --fail-under threshold of ${opts.failUnder}\n`);
      process.exit(1);
    }

    process.exit(failed > 0 ? 1 : 0);
  });

// ---- Compare report builder ----

type DomainScanResult = { domain: string; result: import("./types.js").TestRunResult };

function buildCompareReport(results: DomainScanResult[], includeCompare: boolean): string {
  const categories = results[0]?.result.categories.map((c) => c.category) ?? [];

  function scoreColor(score: number): string {
    if (score >= 80) return "#22c55e";
    if (score >= 50) return "#f59e0b";
    return "#ef4444";
  }

  function catLabel(cat: string): string {
    return cat.split("-").map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");
  }

  // Build comparison table HTML
  let compareSection = "";
  if (includeCompare && results.length > 1) {
    // Header row
    let headerCells = `<th>Category</th>`;
    for (const r of results) {
      headerCells += `<th>${r.domain}</th>`;
    }
    headerCells += `<th>Winner</th>`;

    // Overall row
    let overallCells = `<td><strong>Overall</strong></td>`;
    let overallWinner = results.reduce((a, b) => a.result.compositeScore >= b.result.compositeScore ? a : b);
    for (const r of results) {
      const color = scoreColor(r.result.compositeScore);
      const bold = r.domain === overallWinner.domain ? " style=\"font-weight:700\"" : "";
      overallCells += `<td${bold}><span style="color:${color}">${r.result.compositeScore}</span></td>`;
    }
    overallCells += `<td>${overallWinner.domain}</td>`;

    // Category rows
    let categoryRows = `<tr>${overallCells}</tr>`;
    for (const cat of categories) {
      let cells = `<td>${catLabel(cat)}</td>`;
      let bestScore = -1;
      let bestDomain = "";
      const scores: { domain: string; score: number; tested: boolean }[] = [];
      for (const r of results) {
        const catResult = r.result.categories.find((c) => c.category === cat);
        const score = catResult?.tested ? Math.round(catResult.cappedScore) : -1;
        scores.push({ domain: r.domain, score, tested: catResult?.tested ?? false });
        if (score > bestScore) { bestScore = score; bestDomain = r.domain; }
      }
      for (const s of scores) {
        if (!s.tested) {
          cells += `<td style="color:#64748b">—</td>`;
        } else {
          const color = scoreColor(s.score);
          const bold = s.domain === bestDomain ? " style=\"font-weight:700\"" : "";
          cells += `<td${bold}><span style="color:${color}">${s.score}</span></td>`;
        }
      }
      cells += `<td>${bestScore >= 0 ? bestDomain : "—"}</td>`;
      categoryRows += `<tr>${cells}</tr>`;
    }

    compareSection = `
<section class="compare-section">
  <h2>Side-by-Side Comparison</h2>
  <table class="compare-table">
    <thead><tr>${headerCells}</tr></thead>
    <tbody>${categoryRows}</tbody>
  </table>
</section>`;
  }

  // Individual domain cards
  let domainCards = "";
  for (const r of results) {
    const color = scoreColor(r.result.compositeScore);
    let catRows = "";
    for (const cat of r.result.categories) {
      const s = cat.tested ? Math.round(cat.cappedScore) : -1;
      const c = s >= 0 ? scoreColor(s) : "#64748b";
      catRows += `<tr><td>${catLabel(cat.category)}</td><td style="color:${c}">${s >= 0 ? s : "—"}</td></tr>`;
    }
    domainCards += `
<div class="domain-card">
  <h3>${r.domain}</h3>
  <div class="big-score" style="color:${color}">${r.result.compositeScore}</div>
  <table class="cat-table"><tbody>${catRows}</tbody></table>
</div>`;
  }

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>MCPLens Comparison Report</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: ui-sans-serif, system-ui, sans-serif; background: #0f172a; color: #f1f5f9; padding: 2rem; }
  h1 { font-size: 1.75rem; font-weight: 700; margin-bottom: 0.25rem; }
  .subtitle { color: #94a3b8; margin-bottom: 2rem; font-size: 0.9rem; }
  .brand { color: #6366f1; font-weight: 700; }
  .compare-section { background: #1e293b; border-radius: 1rem; padding: 1.5rem; margin-bottom: 2rem; overflow-x: auto; }
  .compare-section h2 { font-size: 1.1rem; margin-bottom: 1rem; color: #cbd5e1; }
  .compare-table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
  .compare-table th, .compare-table td { padding: 0.6rem 1rem; text-align: center; border-bottom: 1px solid #334155; }
  .compare-table th { color: #94a3b8; font-weight: 600; }
  .compare-table td:first-child { text-align: left; }
  .domains-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.5rem; }
  .domain-card { background: #1e293b; border-radius: 1rem; padding: 1.5rem; }
  .domain-card h3 { font-size: 1rem; color: #cbd5e1; margin-bottom: 0.75rem; }
  .big-score { font-size: 3.5rem; font-weight: 800; line-height: 1; margin-bottom: 1rem; }
  .cat-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  .cat-table td { padding: 0.3rem 0; border-bottom: 1px solid #334155; }
  .cat-table td:last-child { text-align: right; font-weight: 600; }
</style>
</head>
<body>
<h1><span class="brand">MCPLens</span> — Comparison Report</h1>
<p class="subtitle">${results.length} domains compared &bull; ${new Date().toLocaleString()}</p>
${compareSection}
<div class="domains-grid">${domainCards}
</div>
</body>
</html>`;
}

program.parse(process.argv);
