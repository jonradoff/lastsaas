import { TestRunResult, AssertionResult, CategoryResult, ScenarioResult } from "../types.js";

function scoreColor(score: number): string {
  if (score >= 80) return "#22c55e";
  if (score >= 50) return "#f59e0b";
  return "#ef4444";
}

function scoreLabel(score: number): string {
  if (score >= 80) return "Good";
  if (score >= 50) return "Needs Improvement";
  return "Poor";
}

function severityColor(severity: string): string {
  if (severity === "high") return "#ef4444";
  if (severity === "medium") return "#f59e0b";
  return "#3b82f6";
}

function severityBg(severity: string): string {
  if (severity === "high") return "rgba(239,68,68,0.15)";
  if (severity === "medium") return "rgba(245,158,11,0.15)";
  return "rgba(59,130,246,0.15)";
}

function categoryLabel(cat: string): string {
  return cat.split("-").map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");
}

function formatDuration(ms: number): string {
  return (ms / 1000).toFixed(1);
}

function formatTimestamp(ts: number): string {
  return new Date(ts).toLocaleString();
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// --- Finding context mapping ---

interface FindingContext {
  title: string;
  impact: string;
  fix: string;
  effort: string;
}

function classifyFinding(assertionType: string, field?: string, message?: string): FindingContext {
  const f = (field ?? "").toLowerCase();
  const m = (message ?? "").toLowerCase();

  // Missing price data
  if ((assertionType === "percentage_threshold" || assertionType === "each_item_has_field") && f.includes("price")) {
    return {
      title: "Missing Price Data",
      impact: "Products without prices are invisible to buyer agents for comparison and purchase.",
      fix: "Add a non-null, positive `price` field to every product in your search results.",
      effort: "~30 minutes",
    };
  }

  // Short / empty descriptions
  if ((assertionType === "field_non_empty" || assertionType === "percentage_threshold") && f.includes("description")) {
    return {
      title: "Short or Empty Descriptions",
      impact: "Agents match products to user queries using descriptions. Short descriptions give agents nothing to work with.",
      fix: "Expand descriptions to 50+ characters with key features, specs, and differentiators.",
      effort: "~2 hours for full catalog",
    };
  }

  // Missing structured attributes
  if (assertionType === "each_item_has_field" && f.includes("attribute")) {
    return {
      title: "Missing Structured Attributes",
      impact: "Without structured attributes, agents can't filter by specs like RAM, battery life, or weight.",
      fix: "Add an `attributes` object with key specs as structured fields: { \"ram_gb\": 16, \"battery_hours\": 10 }.",
      effort: "~1-2 hours",
    };
  }

  // Invalid image URLs
  if ((assertionType === "url_format" || assertionType === "each_item_has_field") && f.includes("image")) {
    return {
      title: "Invalid or Missing Image URLs",
      impact: "Agents may display broken images or skip products with invalid media URLs.",
      fix: "Ensure all image_url values are fully-qualified URLs (https://...) that resolve correctly.",
      effort: "~30 minutes",
    };
  }

  // Unrealistic inventory
  if (assertionType === "value_range" && f.includes("inventory")) {
    return {
      title: "Unrealistic Inventory Values",
      impact: "Inventory counts of 99999 signal dummy data. Agents may deprioritize merchants with unreliable stock data.",
      fix: "Use real inventory counts. If inventory tracking isn't available, omit the field rather than using placeholder values.",
      effort: "~15 minutes",
    };
  }

  // Search not returning results
  if (assertionType === "array_non_empty") {
    return {
      title: "Search Returning Empty Results",
      impact: "If search returns no results for common queries, agents will skip your store entirely.",
      fix: "Verify your search implementation matches on product name, description, and category.",
      effort: "~1 hour",
    };
  }

  // Slow response time
  if (assertionType === "response_time") {
    return {
      title: "Slow Response Time",
      impact: "Agents query multiple merchants in parallel with timeout budgets. Slow responses mean your products are excluded.",
      fix: "Optimize query performance. Target < 1 second response time.",
      effort: "Variable",
    };
  }

  // Missing required fields
  if (assertionType === "field_present") {
    const fieldName = field ? ` (${field})` : "";
    return {
      title: `Missing Required Field${fieldName}`,
      impact: "Products missing core fields (id, name) cannot be referenced or displayed by agents.",
      fix: "Ensure every product has id, name, and description fields in all API responses.",
      effort: "~15 minutes",
    };
  }

  // Error responses
  if (assertionType === "status_code") {
    return {
      title: "Error Responses",
      impact: "Error responses for valid requests indicate endpoint reliability issues.",
      fix: "Check server logs for the failing endpoints and fix the underlying error.",
      effort: "Variable",
    };
  }

  // Value positive (e.g. price > 0)
  if (assertionType === "value_positive" && f.includes("price")) {
    return {
      title: "Zero or Negative Prices",
      impact: "Products without prices are invisible to buyer agents for comparison and purchase.",
      fix: "Add a non-null, positive `price` field to every product in your search results.",
      effort: "~30 minutes",
    };
  }

  // Generic fallback
  const fieldPart = field ? ` (${field})` : "";
  return {
    title: `${assertionType}${fieldPart}`,
    impact: "This issue may affect how AI agents interact with your MCP server.",
    fix: "Review the assertion details and adjust your MCP server response accordingly.",
    effort: "Variable",
  };
}

// --- Opportunity items ---

interface OpportunityItem {
  context: FindingContext;
  severity: "high" | "medium" | "low";
  message: string;
  category: string;
  assertionType: string;
  field?: string;
}

function collectOpportunities(result: TestRunResult): OpportunityItem[] {
  const opportunities: OpportunityItem[] = [];

  for (const cat of result.categories) {
    for (const scenario of cat.scenarios) {
      for (const ar of scenario.assertions) {
        if (!ar.passed) {
          const ctx = classifyFinding(ar.assertion.assertion, ar.assertion.field, ar.message);
          opportunities.push({
            context: ctx,
            severity: ar.assertion.severity,
            message: ar.message,
            category: cat.category,
            assertionType: ar.assertion.assertion,
            field: ar.assertion.field,
          });
        }
      }
    }
  }

  const order = { high: 0, medium: 1, low: 2 };
  opportunities.sort((a, b) => order[a.severity] - order[b.severity]);
  return opportunities;
}

function collectPassedChecks(result: TestRunResult): Array<{ name: string; category: string }> {
  const passed: Array<{ name: string; category: string }> = [];
  for (const cat of result.categories) {
    for (const scenario of cat.scenarios) {
      if (scenario.status === "passed") {
        passed.push({ name: scenario.scenario.name, category: cat.category });
      }
    }
  }
  return passed;
}

function scenarioCounts(cat: CategoryResult): { passed: number; failed: number; skipped: number } {
  let passed = 0, failed = 0, skipped = 0;
  for (const s of cat.scenarios) {
    if (s.status === "passed") passed++;
    else if (s.status === "skipped" || s.status === "setup-failed") skipped++;
    else failed++;
  }
  return { passed, failed, skipped };
}

// --- Executive summary ---

function generateExecutiveSummary(result: TestRunResult, opportunities: OpportunityItem[]): string {
  const score = result.compositeScore;
  const testedCount = result.testedCategories.length;
  const totalCats = result.categories.length;

  // Count categories with strong scores (>= 80)
  const strongCats = result.categories.filter(c => c.tested && c.cappedScore >= 80).length;

  // Find the biggest score killer
  let topKiller = "";
  for (const cat of result.categories) {
    for (const k of cat.scoreKillers) {
      topKiller = `Your ${categoryLabel(cat.category)} score is capped at ${k.cap} because ${k.condition}.`;
      break;
    }
    if (topKiller) break;
  }

  // Find highest-severity issue
  const topOpp = opportunities.length > 0 ? opportunities[0] : null;

  let summary = "";
  if (score >= 80) {
    summary = `Your MCP server is <strong>ready</strong> for AI buyer agents. ${strongCats} of ${testedCount} tested categories passed with strong scores.`;
  } else if (score >= 50) {
    const issueSnippet = topOpp
      ? `Key issues include ${topOpp.context.title.toLowerCase()}.`
      : "Some categories need improvement.";
    summary = `Your MCP server is <strong>partially ready</strong> for AI buyer agents. ${issueSnippet} Fixing these issues would make your catalog fully accessible to the growing number of AI-powered shoppers.`;
  } else {
    const issueSnippet = topOpp
      ? `Critical issues include ${topOpp.context.title.toLowerCase()}.`
      : "Multiple categories need attention.";
    summary = `Your MCP server has <strong>significant issues</strong> that prevent AI buyer agents from shopping effectively. ${issueSnippet}`;
  }

  if (topKiller) {
    summary += ` ${topKiller}`;
  }

  return summary;
}

// --- Render functions ---

function renderScoreCircle(result: TestRunResult, executiveSummary: string): string {
  const score = result.compositeScore;
  const color = scoreColor(score);
  const circumference = 2 * Math.PI * 54;
  const offset = circumference * (1 - score / 100);

  return `
    <div class="score-hero">
      <div class="score-circle-wrap">
        <svg width="140" height="140" viewBox="0 0 140 140">
          <circle cx="70" cy="70" r="54" fill="none" stroke="#334155" stroke-width="10"/>
          <circle
            cx="70" cy="70" r="54"
            fill="none"
            stroke="${color}"
            stroke-width="10"
            stroke-linecap="round"
            stroke-dasharray="${circumference.toFixed(2)}"
            stroke-dashoffset="${offset.toFixed(2)}"
            transform="rotate(-90 70 70)"
          />
        </svg>
        <div class="score-number" style="color:${color}">${score}</div>
      </div>
      <div class="score-label" style="color:${color}">${scoreLabel(score)}</div>
      <div class="executive-summary">${executiveSummary}</div>
      <div class="score-based-on">Based on: ${result.testedCategories.map(categoryLabel).join(", ")}</div>
    </div>`;
}

function renderScoreKillerAlerts(result: TestRunResult): string {
  const alerts: string[] = [];

  for (const cat of result.categories) {
    if (cat.scoreKillers.length > 0) {
      for (const killer of cat.scoreKillers) {
        // Build an explanatory message
        const capExplanation = `Your ${categoryLabel(cat.category)} score is capped at ${killer.cap} because ${killer.condition}. Buyer agents cannot compare or purchase products without complete data. This is the single highest-impact issue to fix.`;
        alerts.push(`
          <div class="alert-box">
            <span class="alert-icon">&#9888;</span>
            <div class="alert-content">
              <span class="alert-title">Score Cap Applied &mdash; ${categoryLabel(cat.category)}</span>
              <span class="alert-detail">${escapeHtml(capExplanation)}</span>
            </div>
          </div>`);
      }
    }
  }

  if (alerts.length === 0) return "";

  return `<div class="section killer-section">${alerts.join("")}</div>`;
}

function renderPartialBanner(result: TestRunResult): string {
  if (!result.partialResults) return "";
  const reason = result.partialReason ? ` &mdash; ${escapeHtml(result.partialReason)}` : "";
  return `
    <div class="partial-banner">
      <span class="alert-icon">&#8505;</span>
      <strong>Partial Results${reason}</strong>
      Some scenarios could not be run. The composite score reflects only tested categories.
    </div>`;
}

function renderCategoryCards(result: TestRunResult): string {
  const cards = result.categories.map((cat) => {
    if (!cat.tested) {
      return `
        <div class="category-card untested">
          <div class="cat-label">${categoryLabel(cat.category)}</div>
          <div class="cat-score-wrap">
            <span class="cat-score untested-score">&mdash;</span>
          </div>
          <div class="cat-meta">Not Tested</div>
          <div class="cat-weight">Weight: ${Math.round(cat.weight * 100)}%</div>
        </div>`;
    }

    const color = scoreColor(cat.cappedScore);
    const capped = cat.cappedScore < cat.score;
    const counts = scenarioCounts(cat);

    return `
      <div class="category-card">
        <div class="cat-label">${categoryLabel(cat.category)}</div>
        <div class="cat-score-wrap">
          <span class="cat-score" style="color:${color}">${cat.cappedScore}</span>
          ${capped ? `<span class="cat-raw-score">raw: ${Math.round(cat.score)}</span>` : ""}
        </div>
        <div class="cat-meta">${scoreLabel(cat.cappedScore)}</div>
        <div class="cat-scenario-counts">
          <span class="count-passed">${counts.passed} passed</span>
          ${counts.failed > 0 ? `<span class="count-failed">${counts.failed} failed</span>` : ""}
          ${counts.skipped > 0 ? `<span class="count-skipped">${counts.skipped} skipped</span>` : ""}
        </div>
        <div class="cat-weight">Weight: ${Math.round(cat.effectiveWeight * 100)}%</div>
        ${cat.scoreKillers.length > 0 ? `<div class="cat-killer-tag">Score capped</div>` : ""}
      </div>`;
  });

  return `
    <div class="section">
      <h2 class="section-title">Category Breakdown</h2>
      <div class="category-grid">${cards.join("")}</div>
    </div>`;
}

function renderOpportunities(opportunities: OpportunityItem[]): string {
  if (opportunities.length === 0) {
    return `
      <div class="section">
        <h2 class="section-title">Opportunities</h2>
        <div class="empty-state">No issues found &mdash; all tested assertions passed.</div>
      </div>`;
  }

  const items = opportunities.map((opp) => `
    <div class="opportunity-card" style="border-left-color:${severityColor(opp.severity)}">
      <div class="opp-header">
        <span class="opp-title">${escapeHtml(opp.context.title)}</span>
        <span class="severity-tag" style="color:${severityColor(opp.severity)};background:${severityBg(opp.severity)}">${opp.severity.toUpperCase()}</span>
        <span class="opp-category-tag">${categoryLabel(opp.category)}</span>
      </div>
      <div class="opp-impact">${escapeHtml(opp.context.impact)}</div>
      <details class="opp-details-toggle">
        <summary>Details &amp; How to Fix</summary>
        <div class="opp-details-content">
          <div class="opp-detail-row">
            <span class="opp-detail-label">Details:</span>
            <span class="opp-detail-value">${escapeHtml(opp.message)}</span>
          </div>
          <div class="opp-detail-row">
            <span class="opp-detail-label">How to fix:</span>
            <span class="opp-detail-value">${escapeHtml(opp.context.fix)}</span>
          </div>
          <div class="opp-detail-row">
            <span class="opp-detail-label">Estimated effort:</span>
            <span class="opp-detail-value">${escapeHtml(opp.context.effort)}</span>
          </div>
        </div>
      </details>
    </div>`);

  return `
    <div class="section">
      <h2 class="section-title">Opportunities <span class="count-badge">${opportunities.length}</span></h2>
      <div class="opportunities-list">${items.join("")}</div>
    </div>`;
}

function renderPassedChecks(result: TestRunResult): string {
  const passed = collectPassedChecks(result);
  if (passed.length === 0) return "";

  const items = passed.map((p) => `
    <div class="passed-item">
      <span class="passed-check">&#10003;</span>
      <span class="passed-name">${escapeHtml(p.name)}</span>
      <span class="passed-category">${categoryLabel(p.category)}</span>
    </div>`);

  return `
    <div class="section passed-section">
      <details>
        <summary class="section-title passed-summary">Show ${passed.length} passed check${passed.length !== 1 ? "s" : ""}</summary>
        <div class="passed-list">${items.join("")}</div>
      </details>
    </div>`;
}

function renderScenarioDetail(result: TestRunResult): string {
  const testedCats = result.categories.filter(c => c.tested && c.scenarios.length > 0);
  if (testedCats.length === 0) return "";

  const catSections = testedCats.map((cat) => {
    const scenarioItems = cat.scenarios.map((s) => {
      const statusIcon = s.status === "passed" ? "&#10003;" :
                         s.status === "skipped" || s.status === "setup-failed" ? "&#9711;" : "&#10007;";
      const statusClass = s.status === "passed" ? "status-pass" :
                          s.status === "skipped" || s.status === "setup-failed" ? "status-skip" : "status-fail";

      const assertionRows = s.assertions.map((ar) => {
        const aIcon = ar.passed ? "&#10003;" : "&#10007;";
        const aClass = ar.passed ? "status-pass" : "status-fail";
        const fieldPart = ar.assertion.field ? ` &mdash; ${escapeHtml(ar.assertion.field)}` : "";
        return `
          <div class="assertion-row">
            <span class="${aClass}">${aIcon}</span>
            <span class="assertion-name">${escapeHtml(ar.assertion.assertion)}${fieldPart}</span>
            <span class="assertion-score">${Math.round(ar.score * 100)}%</span>
            <span class="assertion-msg">${escapeHtml(ar.message)}</span>
          </div>`;
      }).join("");

      return `
        <details class="scenario-detail-item">
          <summary class="scenario-summary">
            <span class="${statusClass}">${statusIcon}</span>
            <span class="scenario-name">${escapeHtml(s.scenario.name)}</span>
            <span class="scenario-score">${Math.round(s.score)}</span>
          </summary>
          <div class="assertion-list">${assertionRows || '<div class="empty-state">No assertions</div>'}</div>
        </details>`;
    }).join("");

    return `
      <details class="cat-detail-block">
        <summary class="cat-detail-summary">${categoryLabel(cat.category)}</summary>
        <div class="cat-detail-scenarios">${scenarioItems}</div>
      </details>`;
  }).join("");

  return `
    <div class="section">
      <h2 class="section-title">Scenario Detail</h2>
      ${catSections}
    </div>`;
}

function renderNotTested(result: TestRunResult): string {
  const skipped: Array<{ name: string; reason: string; category: string }> = [];

  for (const cat of result.categories) {
    for (const scenario of cat.scenarios) {
      if (scenario.status === "skipped" || scenario.status === "setup-failed") {
        skipped.push({
          name: scenario.scenario.name,
          reason: scenario.skipReason ?? scenario.status,
          category: cat.category,
        });
      }
    }
  }

  const untestedCats = result.categories.filter((c) => !c.tested);

  if (skipped.length === 0 && untestedCats.length === 0) return "";

  const catItems = untestedCats.map((c) => `
    <div class="not-tested-item">
      <span class="not-tested-name">${categoryLabel(c.category)}</span>
      <span class="not-tested-reason">Category not tested &mdash; no matching capabilities</span>
    </div>`);

  const scenarioItems = skipped.map((s) => `
    <div class="not-tested-item">
      <span class="not-tested-name">${escapeHtml(s.name)}</span>
      <span class="not-tested-category">${categoryLabel(s.category)}</span>
      <span class="not-tested-reason">${escapeHtml(s.reason)}</span>
    </div>`);

  return `
    <div class="section not-tested-section">
      <h2 class="section-title">Not Tested</h2>
      <div class="not-tested-list">
        ${catItems.join("")}
        ${scenarioItems.join("")}
      </div>
    </div>`;
}

function renderFooter(result: TestRunResult, jsonData: string): string {
  return `
    <footer class="footer">
      <div class="footer-meta">
        <span>AgentLens v${escapeHtml(result.version)}</span>
        <span class="sep">&middot;</span>
        <span>${result.scenarioCount} scenarios</span>
        <span class="sep">&middot;</span>
        <span>${formatDuration(result.durationMs)}s runtime</span>
        <span class="sep">&middot;</span>
        <span>${formatTimestamp(result.timestamp)}</span>
      </div>
      <button id="download-json" class="download-btn">Download JSON</button>
    </footer>
    <script type="application/json" id="agentlens-data">${jsonData}</script>
    <script>
document.getElementById('download-json').addEventListener('click', function() {
  var data = document.getElementById('agentlens-data').textContent;
  var blob = new Blob([data], {type: 'application/json'});
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url; a.download = 'agentlens-report.json'; a.click();
});
</script>`;
}

const CSS = `
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  body {
    background: #0f172a;
    color: #e2e8f0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    min-height: 100vh;
    font-size: 15px;
    line-height: 1.6;
  }

  .topbar {
    background: #1e293b;
    border-bottom: 1px solid #334155;
    padding: 14px 32px;
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .topbar-logo {
    font-size: 18px;
    font-weight: 700;
    color: #38bdf8;
    letter-spacing: -0.5px;
  }

  .topbar-server {
    font-size: 13px;
    color: #94a3b8;
    padding: 3px 10px;
    background: #0f172a;
    border-radius: 4px;
    font-family: "SF Mono", "Fira Code", "Consolas", monospace;
  }

  .container {
    max-width: 1000px;
    margin: 0 auto;
    padding: 40px 24px 80px;
  }

  /* Score Hero */
  .score-hero {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 48px 24px 40px;
    text-align: center;
  }

  .score-circle-wrap {
    position: relative;
    width: 140px;
    height: 140px;
    margin-bottom: 16px;
  }

  .score-circle-wrap svg {
    position: absolute;
    top: 0; left: 0;
  }

  .score-number {
    position: absolute;
    top: 50%; left: 50%;
    transform: translate(-50%, -50%);
    font-size: 44px;
    font-weight: 800;
    letter-spacing: -2px;
    line-height: 1;
  }

  .score-label {
    font-size: 20px;
    font-weight: 700;
    margin-bottom: 12px;
  }

  .executive-summary {
    color: #cbd5e1;
    font-size: 15px;
    line-height: 1.7;
    max-width: 640px;
    margin-bottom: 14px;
  }

  .executive-summary strong {
    color: #f1f5f9;
  }

  .score-based-on {
    font-size: 13px;
    color: #64748b;
  }

  /* Alerts */
  .killer-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .alert-box {
    display: flex;
    align-items: flex-start;
    gap: 14px;
    background: rgba(239,68,68,0.1);
    border: 1px solid rgba(239,68,68,0.35);
    border-radius: 8px;
    padding: 14px 18px;
  }

  .alert-icon {
    font-size: 18px;
    flex-shrink: 0;
    color: #ef4444;
    margin-top: 1px;
  }

  .alert-content {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .alert-title {
    font-weight: 600;
    color: #fca5a5;
    font-size: 14px;
  }

  .alert-detail {
    font-size: 13px;
    color: #d1d5db;
    line-height: 1.5;
  }

  .partial-banner {
    display: flex;
    align-items: center;
    gap: 12px;
    background: rgba(59,130,246,0.1);
    border: 1px solid rgba(59,130,246,0.3);
    border-radius: 8px;
    padding: 14px 18px;
    margin-bottom: 24px;
    font-size: 14px;
    color: #93c5fd;
  }

  /* Sections */
  .section {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 12px;
    padding: 28px;
    margin-bottom: 20px;
  }

  .section-title {
    font-size: 16px;
    font-weight: 700;
    color: #f1f5f9;
    margin-bottom: 20px;
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .count-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: #334155;
    color: #94a3b8;
    font-size: 12px;
    font-weight: 600;
    border-radius: 12px;
    padding: 1px 8px;
    min-width: 24px;
  }

  /* Category Grid */
  .category-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 14px;
  }

  .category-card {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: 20px 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .category-card.untested {
    opacity: 0.5;
  }

  .cat-label {
    font-size: 12px;
    font-weight: 600;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .cat-score-wrap {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin: 4px 0;
  }

  .cat-score {
    font-size: 36px;
    font-weight: 800;
    letter-spacing: -1.5px;
    line-height: 1;
  }

  .untested-score {
    color: #475569;
    font-size: 32px;
  }

  .cat-raw-score {
    font-size: 12px;
    color: #64748b;
  }

  .cat-meta {
    font-size: 12px;
    color: #64748b;
  }

  .cat-scenario-counts {
    display: flex;
    gap: 10px;
    font-size: 11px;
    margin-top: 2px;
  }

  .count-passed { color: #22c55e; }
  .count-failed { color: #ef4444; }
  .count-skipped { color: #94a3b8; }

  .cat-weight {
    font-size: 11px;
    color: #475569;
    margin-top: 2px;
  }

  .cat-killer-tag {
    display: inline-block;
    background: rgba(239,68,68,0.15);
    color: #fca5a5;
    font-size: 10px;
    font-weight: 600;
    padding: 2px 7px;
    border-radius: 4px;
    margin-top: 4px;
    width: fit-content;
  }

  /* Opportunities */
  .opportunities-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .opportunity-card {
    background: #0f172a;
    border: 1px solid #334155;
    border-left: 4px solid;
    border-radius: 8px;
    padding: 16px 18px;
  }

  .opp-header {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    margin-bottom: 8px;
  }

  .opp-title {
    font-size: 14px;
    font-weight: 600;
    color: #f1f5f9;
    flex: 1;
    min-width: 150px;
  }

  .severity-tag {
    font-size: 10px;
    font-weight: 700;
    padding: 2px 7px;
    border-radius: 4px;
    letter-spacing: 0.5px;
    flex-shrink: 0;
  }

  .opp-category-tag {
    font-size: 11px;
    color: #64748b;
    flex-shrink: 0;
  }

  .opp-impact {
    font-size: 13px;
    color: #cbd5e1;
    line-height: 1.5;
    margin-bottom: 8px;
  }

  .opp-details-toggle {
    border-top: 1px solid #1e293b;
    padding-top: 8px;
  }

  .opp-details-toggle summary {
    cursor: pointer;
    font-size: 12px;
    font-weight: 600;
    color: #94a3b8;
    padding: 4px 0;
    list-style: none;
  }

  .opp-details-toggle summary::-webkit-details-marker { display: none; }

  .opp-details-toggle summary::before {
    content: "\\25B6  ";
    font-size: 9px;
    transition: transform 0.15s;
    display: inline-block;
  }

  .opp-details-toggle[open] summary::before {
    transform: rotate(90deg);
  }

  .opp-details-content {
    padding: 10px 0 4px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .opp-detail-row {
    display: flex;
    gap: 8px;
    font-size: 13px;
    line-height: 1.5;
  }

  .opp-detail-label {
    color: #94a3b8;
    font-weight: 600;
    flex-shrink: 0;
    min-width: 110px;
  }

  .opp-detail-value {
    color: #cbd5e1;
  }

  .empty-state {
    color: #64748b;
    font-size: 14px;
    padding: 16px 0;
    text-align: center;
  }

  /* Passed Checks */
  .passed-section {
    border-color: rgba(34,197,94,0.2);
  }

  .passed-summary {
    cursor: pointer;
    list-style: none;
    margin-bottom: 0;
  }

  .passed-summary::-webkit-details-marker { display: none; }

  .passed-summary::before {
    content: "\\25B6  ";
    font-size: 10px;
    display: inline-block;
    transition: transform 0.15s;
    color: #22c55e;
  }

  details[open] > .passed-summary::before {
    transform: rotate(90deg);
  }

  .passed-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 14px;
  }

  .passed-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: #0f172a;
    border: 1px solid #1e293b;
    border-radius: 6px;
  }

  .passed-check {
    color: #22c55e;
    font-size: 14px;
    font-weight: 700;
    flex-shrink: 0;
  }

  .passed-name {
    font-size: 13px;
    color: #e2e8f0;
    flex: 1;
  }

  .passed-category {
    font-size: 11px;
    color: #64748b;
    flex-shrink: 0;
  }

  /* Scenario Detail */
  .cat-detail-block {
    margin-bottom: 10px;
  }

  .cat-detail-summary {
    cursor: pointer;
    font-size: 14px;
    font-weight: 600;
    color: #cbd5e1;
    padding: 10px 0;
    list-style: none;
  }

  .cat-detail-summary::-webkit-details-marker { display: none; }

  .cat-detail-summary::before {
    content: "\\25B6  ";
    font-size: 10px;
    display: inline-block;
    transition: transform 0.15s;
  }

  .cat-detail-block[open] > .cat-detail-summary::before {
    transform: rotate(90deg);
  }

  .cat-detail-scenarios {
    padding-left: 16px;
  }

  .scenario-detail-item {
    margin-bottom: 6px;
  }

  .scenario-summary {
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: #0f172a;
    border: 1px solid #1e293b;
    border-radius: 6px;
    list-style: none;
    font-size: 13px;
  }

  .scenario-summary::-webkit-details-marker { display: none; }

  .scenario-name {
    flex: 1;
    color: #e2e8f0;
  }

  .scenario-score {
    color: #94a3b8;
    font-weight: 600;
    font-size: 12px;
    flex-shrink: 0;
  }

  .status-pass { color: #22c55e; font-weight: 700; }
  .status-fail { color: #ef4444; font-weight: 700; }
  .status-skip { color: #94a3b8; font-weight: 700; }

  .assertion-list {
    padding: 8px 12px 8px 36px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .assertion-row {
    display: flex;
    align-items: baseline;
    gap: 8px;
    font-size: 12px;
    padding: 4px 0;
    flex-wrap: wrap;
  }

  .assertion-name {
    color: #94a3b8;
    font-family: "SF Mono", "Fira Code", "Consolas", monospace;
    flex: 1;
    min-width: 120px;
  }

  .assertion-score {
    color: #64748b;
    flex-shrink: 0;
  }

  .assertion-msg {
    color: #64748b;
    font-size: 11px;
    width: 100%;
    padding-left: 22px;
  }

  /* Not Tested */
  .not-tested-section {
    border-color: #1e293b;
    background: #161f2e;
  }

  .not-tested-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .not-tested-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    background: #0f172a;
    border: 1px solid #1e293b;
    border-radius: 7px;
    flex-wrap: wrap;
  }

  .not-tested-name {
    font-size: 13px;
    font-weight: 600;
    color: #64748b;
    flex: 1;
    min-width: 120px;
  }

  .not-tested-category {
    font-size: 11px;
    color: #475569;
  }

  .not-tested-reason {
    font-size: 12px;
    color: #475569;
    font-style: italic;
  }

  /* Footer */
  .footer {
    margin-top: 40px;
    padding-top: 24px;
    border-top: 1px solid #1e293b;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 12px;
  }

  .footer-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: #475569;
    flex-wrap: wrap;
  }

  .sep {
    color: #334155;
  }

  .download-btn {
    background: #334155;
    color: #e2e8f0;
    border: 1px solid #475569;
    border-radius: 6px;
    padding: 7px 16px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s;
  }

  .download-btn:hover {
    background: #475569;
  }

  /* Responsive */
  @media (max-width: 800px) {
    .container { padding: 24px 12px 60px; }
    .section { padding: 20px 16px; }
    .category-grid { grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); }
    .opp-detail-row { flex-direction: column; gap: 2px; }
    .opp-detail-label { min-width: unset; }
  }
`;

export function renderHtmlTemplate(result: TestRunResult): string {
  const embeddable = { ...result, schema_version: result.schemaVersion };
  const jsonData = JSON.stringify(embeddable, null, 2).replace(/<\/script>/gi, "<\\/script>");

  const opportunities = collectOpportunities(result);
  const executiveSummary = generateExecutiveSummary(result, opportunities);

  const body = [
    `<div class="topbar">
      <span class="topbar-logo">AgentLens</span>
      <span class="topbar-server">${escapeHtml(result.serverIdentifier)}</span>
    </div>`,
    `<div class="container">`,
    renderScoreCircle(result, executiveSummary),
    renderPartialBanner(result),
    renderScoreKillerAlerts(result),
    renderCategoryCards(result),
    renderOpportunities(opportunities),
    renderPassedChecks(result),
    renderScenarioDetail(result),
    renderNotTested(result),
    renderFooter(result, jsonData),
    `</div>`,
  ].join("\n");

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>AgentLens Report &mdash; ${escapeHtml(result.serverIdentifier)}</title>
  <style>${CSS}</style>
</head>
<body>
${body}
</body>
</html>`;
}
