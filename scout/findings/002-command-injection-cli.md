---
id: 002
type: security
severity: critical
status: new
found: 2026-03-24
phase: audit
---

## Command injection vulnerability in scanner CLI exec() calls

**What:** The scanner CLI interpolates user-controlled `--out` flag values directly into shell commands passed to `exec()`, allowing arbitrary command execution. An attacker (or malicious input) can inject shell commands via the output filename.

**Where:** `scanner/src/cli.ts` — lines 144, 146, 148, 258, 324

**Evidence:**
```typescript
// Line 144-148:
openCmd = `start "" "${outputPath}"`;   // Windows
openCmd = `open "${outputPath}"`;        // macOS
openCmd = `xdg-open "${outputPath}"`;    // Linux
exec(openCmd, (err) => { ... });
```

A crafted `--out` value like `'report.html"; rm -rf /; #'` would break out of the quotes and execute arbitrary commands.

This pattern appears in three separate locations in cli.ts (lines 144-148, 258, 324).

**Suggested Fix:** Replace `exec()` with `execFile()` which does not use a shell and prevents injection:

```typescript
import { execFile } from "child_process";

// macOS
execFile("open", [outputPath], (err) => { ... });
// Windows
execFile("cmd", ["/c", "start", "", outputPath], (err) => { ... });
// Linux
execFile("xdg-open", [outputPath], (err) => { ... });
```

**Additional scanner issues found this cycle (for future findings):**
- 6x unvalidated JSON.parse calls (HIGH)
- Greedy JSON extraction from LLM responses in ai-assessor.ts and agent-simulator.ts (HIGH)
- Promise.race timeout timer never cleaned up in connection.ts:30-36 (MEDIUM)
- Unvalidated type coercion in agent-simulator.ts:233-238 (MEDIUM)
- Command argument splitting without quote handling in connection.ts:71-73 (HIGH)
- 5x hardcoded values that should be configurable (model names, pricing, timeouts) (MEDIUM)
- Unused import `Type` in agent-simulator.ts:1 (LOW)
- MCP protocol assumptions about inputSchema structure in capability-mapper.ts:31 (MEDIUM)
