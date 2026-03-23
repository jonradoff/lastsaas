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
  .command("scan <domain>")
  .description("Scan a Shopify store's MCP endpoint (https://{domain}/api/mcp)")
  .option("--header <header...>", "HTTP header in Key: Value format (repeatable)")
  .option("--compare", "Compare results against a baseline (reserved for future use)")
  .option("--format <fmt>", 'Output format: "html" (default) or "json"', "html")
  .option("--out <path>", "Output file path")
  .option("--open", "Open report in default browser after generation")
  .option("--verbose", "Print interaction log to stderr")
  .option("--fail-under <score>", "Exit code 1 if composite score is below this threshold", parseFloat)
  .action(async (domain: string, opts: { header?: string[]; compare?: boolean; format?: string; out?: string; open?: boolean; verbose?: boolean; failUnder?: number }) => {
    try {
      // Auto-construct the Shopify MCP URL
      const url = `https://${domain}/api/mcp`;

      if (opts.verbose) {
        process.stderr.write(`Scanning Shopify store: ${domain}\n`);
        process.stderr.write(`Connecting via Streamable HTTP: ${url}\n`);
      }

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

      const connection = await connectHTTP(url, Object.keys(headers).length > 0 ? headers : undefined);

      try {
        const verboseLog = opts.verbose ? (msg: string) => process.stderr.write(msg + "\n") : undefined;

        // Use Shopify-specific scenarios for the scan command
        const shopifyScenariosDir = path.resolve(__dirname, "../scenarios/shopify");

        const result = await runTests(connection, {
          verbose: opts.verbose,
          log: verboseLog,
          scenariosDir: shopifyScenariosDir,
        });

        const format = (opts.format === "json" ? "json" : "html") as "html" | "json";
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
          const { exec } = await import("child_process");
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

        await connection.close();

        if (opts.failUnder !== undefined && result.compositeScore < opts.failUnder) {
          process.stderr.write(
            `Score ${result.compositeScore} is below --fail-under threshold of ${opts.failUnder}\n`
          );
          process.exit(1);
        }

        process.exit(0);
      } catch (err) {
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

program.parse(process.argv);
