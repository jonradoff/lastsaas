import { useState, useEffect, type FormEvent } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { scanApi, leadsApi, type ScanResult, type ScanCategoryResult, type AIAssessment, type AIFinding, type AIQuerySimulation, type AgentSimulation, type ShoppingScenario, type AgentStep } from '../../api/client';

function setMetaTag(property: string, content: string) {
  const selector = `meta[property="${property}"], meta[name="${property}"]`;
  let el = document.querySelector<HTMLMetaElement>(selector);
  if (!el) {
    el = document.createElement('meta');
    // og: and twitter: properties use the "property" attribute; others use "name"
    if (property.startsWith('og:') || property.startsWith('fb:')) {
      el.setAttribute('property', property);
    } else {
      el.setAttribute('name', property);
    }
    document.head.appendChild(el);
  }
  el.setAttribute('content', content);
}

const CATEGORY_LABELS: Record<string, string> = {
  'data-quality': 'Data Quality',
  'product-discovery': 'Product Discovery',
  'checkout-flow': 'Checkout Flow',
  'protocol-compliance': 'Protocol Compliance',
};

const LOADING_MESSAGES = [
  'Connecting to MCP endpoint...',
  'Running scenarios...',
  'Evaluating product discovery...',
  'Testing checkout flow...',
  'Generating report...',
  'Running AI quality assessment...',
];

function scoreColor(score: number): string {
  if (score >= 80) return '#059669';
  if (score >= 50) return '#d97706';
  return '#dc2626';
}

function scoreLabel(score: number): string {
  if (score >= 80) return 'Excellent';
  if (score >= 50) return 'Needs Work';
  return 'Not Ready';
}

function ScoreCircle({ score }: { score: number }) {
  const color = scoreColor(score);
  const radius = 54;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (score / 100) * circumference;

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative w-40 h-40">
        <svg className="w-full h-full -rotate-90" viewBox="0 0 128 128">
          <circle
            cx="64"
            cy="64"
            r={radius}
            stroke="#e2e8f0"
            strokeWidth="12"
            fill="none"
          />
          <circle
            cx="64"
            cy="64"
            r={radius}
            stroke={color}
            strokeWidth="12"
            fill="none"
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            strokeLinecap="round"
            style={{ transition: 'stroke-dashoffset 0.8s ease' }}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-4xl font-bold text-slate-900">{Math.round(score)}</span>
          <span className="text-xs text-slate-400">/ 100</span>
        </div>
      </div>
      <span className="text-sm font-medium" style={{ color }}>{scoreLabel(score)}</span>
    </div>
  );
}

function CategoryCard({ cat }: { cat: ScanCategoryResult }) {
  const color = scoreColor(cat.cappedScore);
  const label = CATEGORY_LABELS[cat.category] ?? cat.category;

  return (
    <div className="bg-white border border-slate-200 rounded-xl shadow-sm p-5 flex flex-col gap-3">
      <div className="flex items-start justify-between gap-2">
        <h3 className="font-semibold text-slate-900 text-sm">{label}</h3>
        {cat.tested ? (
          <span className="px-2 py-0.5 text-xs rounded-full bg-blue-50 text-blue-700 shrink-0">Tested</span>
        ) : (
          <span className="px-2 py-0.5 text-xs rounded-full bg-slate-100 text-slate-500 shrink-0">Not Tested</span>
        )}
      </div>

      <div className="flex items-end justify-between">
        <div>
          <div className="text-3xl font-bold" style={{ color }}>
            {cat.tested ? Math.round(cat.cappedScore) : '—'}
          </div>
          {cat.tested && cat.score !== cat.cappedScore && (
            <div className="text-xs text-slate-400 mt-0.5">
              Raw: {Math.round(cat.score)} (capped)
            </div>
          )}
        </div>
        <div className="text-right">
          <div className="text-xs text-slate-400">Weight</div>
          <div className="text-sm font-medium text-slate-600">{Math.round(cat.weight * 100)}%</div>
        </div>
      </div>

      {cat.tested && (
        <div className="w-full h-1.5 bg-slate-200 rounded-full overflow-hidden">
          <div
            className="h-full rounded-full transition-all duration-700"
            style={{ width: `${cat.cappedScore}%`, backgroundColor: color }}
          />
        </div>
      )}
    </div>
  );
}

function ScoreKillerAlert({ cat }: { cat: ScanCategoryResult }) {
  if (!cat.scoreKillers || cat.scoreKillers.length === 0) return null;
  const label = CATEGORY_LABELS[cat.category] ?? cat.category;

  return (
    <div className="bg-red-50 border border-red-200 rounded-xl p-4">
      <div className="flex items-start gap-3">
        <div className="text-red-600 text-lg mt-0.5">!</div>
        <div>
          <div className="text-red-700 font-semibold text-sm mb-1">
            Score Cap Active — {label}
          </div>
          {cat.scoreKillers.map((sk, i) => (
            <div key={i} className="text-red-600 text-xs">
              {sk.condition} — capped at {sk.cap}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// --- Finding generation from category data ---

interface Finding {
  title: string;
  severity: 'high' | 'medium' | 'low';
  category: string;
  impact: string;
  fix: string;
}

function severityColor(severity: string): string {
  if (severity === 'high') return '#dc2626';
  if (severity === 'medium') return '#d97706';
  return '#2563eb';
}

function severityBg(severity: string): string {
  if (severity === 'high') return 'bg-red-50 border-red-200';
  if (severity === 'medium') return 'bg-amber-50 border-amber-200';
  return 'bg-blue-50 border-blue-200';
}

function generateFindings(result: ScanResult): Finding[] {
  const findings: Finding[] = [];

  for (const cat of result.categories) {
    if (!cat.tested) continue;
    const label = CATEGORY_LABELS[cat.category] ?? cat.category;

    // Generate findings from score killers
    if (cat.scoreKillers) {
      for (const sk of cat.scoreKillers) {
        findings.push({
          title: `Score Capped: ${sk.condition}`,
          severity: 'high',
          category: label,
          impact: `Your ${label} score is capped at ${sk.cap}/100 because of this issue. This directly limits your overall readiness score.`,
          fix: `Address the "${sk.condition}" issue in your MCP server configuration. This is the highest-impact fix you can make for ${label}.`,
        });
      }
    }

    // Generate findings from low category scores
    if (cat.cappedScore < 50 && !cat.scoreKillers?.length) {
      const categoryFindings: Record<string, Finding> = {
        'data-quality': {
          title: 'Poor Data Quality',
          severity: 'high',
          category: label,
          impact: 'Products with missing prices, short descriptions, or invalid images are invisible to AI buyer agents.',
          fix: 'Ensure every product has a valid price, 50+ character description, and working image URLs. Add structured attributes for key specs.',
        },
        'product-discovery': {
          title: 'Weak Product Discovery',
          severity: 'high',
          category: label,
          impact: 'If agents cannot search and find products reliably, your store is skipped entirely.',
          fix: 'Verify your search returns results for common queries. Ensure product names, descriptions, and categories are indexed.',
        },
        'checkout-flow': {
          title: 'Broken Checkout Flow',
          severity: 'high',
          category: label,
          impact: 'Agents that cannot add items to cart or initiate checkout will abandon your store.',
          fix: 'Test cart operations (add, update, remove) and checkout initiation. Ensure all required fields are documented.',
        },
        'protocol-compliance': {
          title: 'Protocol Compliance Issues',
          severity: 'medium',
          category: label,
          impact: 'Non-compliant MCP responses may cause agent errors or fallback behavior.',
          fix: 'Review the MCP specification and ensure your server returns properly formatted responses with correct error codes.',
        },
      };
      const f = categoryFindings[cat.category];
      if (f) findings.push(f);
    } else if (cat.cappedScore < 80 && cat.cappedScore >= 50 && !cat.scoreKillers?.length) {
      findings.push({
        title: `${label} Needs Improvement`,
        severity: 'medium',
        category: label,
        impact: `Your ${label} score of ${Math.round(cat.cappedScore)} means agents can partially use your store but may encounter issues.`,
        fix: `Review the ${label.toLowerCase()} section of your MCP server. Common issues include incomplete data fields, slow responses, or missing edge case handling.`,
      });
    }
  }

  // Sort by severity
  const order = { high: 0, medium: 1, low: 2 };
  findings.sort((a, b) => order[a.severity] - order[b.severity]);
  return findings;
}

// --- Email Capture Component ---

function EmailGate({ domain, score, onUnlocked }: { domain: string; score: number; onUnlocked: (token: string) => void }) {
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const [captured, setCaptured] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = email.trim();
    if (!trimmed || !trimmed.includes('@')) {
      setError('Please enter a valid email address');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const { token } = await leadsApi.capture(trimmed, domain, score);
      // Store in sessionStorage so it persists across page navigations
      sessionStorage.setItem('mcplens_fix_token', token);
      setCaptured(true);
      onUnlocked(token);
    } catch {
      setError('Something went wrong. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="bg-gradient-to-br from-blue-50 to-indigo-50 border border-blue-200 rounded-xl p-6">
      <div className="flex items-start gap-3 mb-3">
        <div className="w-8 h-8 rounded-full bg-blue-100 flex items-center justify-center shrink-0 mt-0.5">
          <svg className="w-4 h-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        </div>
        <div>
          <p className="font-semibold text-slate-900 text-base">Want to know how to fix these issues?</p>
          <p className="text-sm text-slate-600 mt-1">
            Enter your email to unlock detailed fix instructions for all findings. No spam -- just your fixes.
          </p>
        </div>
      </div>
      <form onSubmit={handleSubmit} className="flex flex-col sm:flex-row gap-2 mt-4">
        <input
          type="email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          placeholder="you@company.com"
          className="flex-1 px-4 py-2.5 bg-white border border-slate-300 rounded-lg text-slate-900 text-sm placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
        />
        <button
          type="submit"
          disabled={submitting || !email.trim()}
          className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed text-white font-semibold rounded-lg transition-colors text-sm shrink-0"
        >
          {submitting ? 'Unlocking...' : 'Unlock Fixes'}
        </button>
      </form>
      {error && <p className="text-red-600 text-xs mt-2">{error}</p>}
      {captured && (
        <div className="mt-3 bg-white/70 border border-blue-200 rounded-lg p-3 text-center">
          <p className="text-sm text-slate-700">
            We've saved your email.{' '}
            <Link
              to={`/signup?email=${encodeURIComponent(email.trim())}`}
              className="text-blue-600 hover:text-blue-700 font-semibold underline"
            >
              Create a free account
            </Link>{' '}
            to track this store's score over time.
          </p>
        </div>
      )}
    </div>
  );
}

// --- Finding Card ---

function FindingCard({ finding, fixUnlocked }: { finding: Finding; fixUnlocked: boolean }) {
  return (
    <div className={`border rounded-xl p-4 ${severityBg(finding.severity)}`}>
      <div className="flex items-start gap-3">
        <span
          className="px-2 py-0.5 text-xs font-semibold rounded-full text-white shrink-0 mt-0.5"
          style={{ backgroundColor: severityColor(finding.severity) }}
        >
          {finding.severity.toUpperCase()}
        </span>
        <div className="flex-1 min-w-0">
          <h4 className="font-semibold text-slate-900 text-sm">{finding.title}</h4>
          <p className="text-xs text-slate-500 mt-0.5">{finding.category}</p>
          <p className="text-sm text-slate-700 mt-2">{finding.impact}</p>
          {fixUnlocked ? (
            <div className="mt-3 bg-white/70 border border-slate-200 rounded-lg p-3">
              <div className="text-xs font-semibold text-emerald-700 uppercase tracking-wider mb-1">How to fix</div>
              <p className="text-sm text-slate-700">{finding.fix}</p>
            </div>
          ) : (
            <div className="mt-3 bg-slate-100/80 border border-slate-200 rounded-lg p-3 flex items-center gap-2">
              <svg className="w-4 h-4 text-slate-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
              <span className="text-sm text-slate-500 italic">Enter your email above to unlock fix instructions</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// --- Layer 2: AI Quality Assessment Section ---

function AIScoreCard({ label, score, summary }: { label: string; score: number; summary: string }) {
  const color = scoreColor(score);
  return (
    <div className="bg-white border border-slate-200 rounded-xl p-4">
      <div className="text-xs font-medium text-slate-500 mb-1">{label}</div>
      <div className="text-2xl font-bold mb-1" style={{ color }}>{score}</div>
      <p className="text-xs text-slate-500 leading-relaxed">{summary}</p>
    </div>
  );
}

function AIFindingCard({ finding }: { finding: AIFinding }) {
  const impactColors: Record<string, string> = {
    high: 'bg-red-50 border-red-200',
    medium: 'bg-amber-50 border-amber-200',
    low: 'bg-blue-50 border-blue-200',
  };
  const impactBadge: Record<string, string> = {
    high: 'bg-red-500',
    medium: 'bg-amber-500',
    low: 'bg-blue-500',
  };

  return (
    <div className={`border rounded-xl p-4 ${impactColors[finding.revenueImpact] ?? 'bg-slate-50 border-slate-200'}`}>
      <div className="flex items-start gap-3">
        <span className={`px-2 py-0.5 text-xs font-semibold rounded-full text-white shrink-0 mt-0.5 ${impactBadge[finding.revenueImpact] ?? 'bg-slate-500'}`}>
          {finding.revenueImpact.toUpperCase()}
        </span>
        <div className="flex-1 min-w-0">
          <h4 className="font-semibold text-slate-900 text-sm">{finding.title}</h4>
          <p className="text-sm text-slate-700 mt-1">{finding.explanation}</p>
          <div className="mt-3 bg-white/70 border border-slate-200 rounded-lg p-3">
            <div className="text-xs font-semibold text-emerald-700 uppercase tracking-wider mb-1">How to fix</div>
            <p className="text-sm text-slate-700 whitespace-pre-wrap">{finding.fix}</p>
          </div>
        </div>
      </div>
    </div>
  );
}

function AIQueryTable({ simulations }: { simulations: AIQuerySimulation[] }) {
  return (
    <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
      <div className="px-4 py-3 border-b border-slate-200 bg-slate-50">
        <h3 className="text-sm font-semibold text-slate-700">Simulated Buyer Queries</h3>
      </div>
      <div className="divide-y divide-slate-100">
        {simulations.map((sim, i) => (
          <div key={i} className="px-4 py-3 flex items-start gap-3">
            <div className={`w-6 h-6 rounded-full flex items-center justify-center shrink-0 mt-0.5 ${sim.wouldFindResult ? 'bg-emerald-100' : 'bg-red-100'}`}>
              {sim.wouldFindResult ? (
                <svg className="w-3.5 h-3.5 text-emerald-600" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
              ) : (
                <svg className="w-3.5 h-3.5 text-red-600" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
              )}
            </div>
            <div className="flex-1">
              <div className="text-sm font-medium text-slate-900">"{sim.query}"</div>
              <div className="text-xs text-slate-500 mt-0.5">{sim.explanation}</div>
            </div>
            <div className="text-xs text-slate-400 shrink-0">{sim.confidence}% confidence</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function AIAssessmentSection({ assessment }: { assessment: AIAssessment }) {
  return (
    <div className="flex flex-col gap-5">
      <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-widest">
        AI Quality Assessment
        <span className="ml-2 px-2 py-0.5 text-xs rounded-full bg-indigo-100 text-indigo-700 font-medium normal-case tracking-normal">
          Powered by Claude
        </span>
      </h2>

      {/* Sub-scores grid */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <AIScoreCard label="Product Relevance" score={assessment.productRelevance.score} summary={assessment.productRelevance.summary} />
        <AIScoreCard label="Description Quality" score={assessment.descriptionQuality.score} summary={assessment.descriptionQuality.summary} />
        <AIScoreCard label="Data Completeness" score={assessment.dataCompleteness.score} summary={assessment.dataCompleteness.summary} />
      </div>

      {/* Missing attributes */}
      {assessment.dataCompleteness.missingAttributes.length > 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-xl p-4">
          <div className="text-sm font-semibold text-amber-800 mb-2">Missing Structured Attributes</div>
          <div className="flex flex-wrap gap-2">
            {assessment.dataCompleteness.missingAttributes.map((attr, i) => (
              <span key={i} className="px-2.5 py-1 bg-white border border-amber-300 rounded-lg text-xs text-amber-700">{attr}</span>
            ))}
          </div>
        </div>
      )}

      {/* Query simulations */}
      {assessment.querySimulations.length > 0 && (
        <AIQueryTable simulations={assessment.querySimulations} />
      )}

      {/* Findings */}
      {assessment.findings.length > 0 && (
        <div className="flex flex-col gap-3">
          <h3 className="text-sm font-semibold text-slate-500">AI-Identified Issues ({assessment.findings.length})</h3>
          {assessment.findings.map((f, i) => (
            <AIFindingCard key={i} finding={f} />
          ))}
        </div>
      )}

      {/* Competitive comparison */}
      {assessment.competitiveComparison && (
        <div className="bg-indigo-50 border border-indigo-200 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-indigo-800 mb-2">Competitive Context</h3>
          <p className="text-sm text-indigo-700 leading-relaxed">{assessment.competitiveComparison}</p>
        </div>
      )}

      {/* Meta info */}
      <div className="text-xs text-slate-400 flex flex-wrap gap-4">
        <span>Model: {assessment.modelUsed}</span>
        <span>Tokens: {assessment.tokenUsage.input + assessment.tokenUsage.output}</span>
        <span>Cost: ${assessment.costEstimateUsd.toFixed(4)}</span>
        <span>Duration: {(assessment.durationMs / 1000).toFixed(1)}s</span>
      </div>
    </div>
  );
}

// --- Agent Simulation Section ---

function StepIcon({ action }: { action: AgentStep['action'] }) {
  const icons: Record<string, { bg: string; icon: string }> = {
    tool_call: { bg: 'bg-blue-100', icon: '🔧' },
    tool_result: { bg: 'bg-emerald-100', icon: '📦' },
    decision: { bg: 'bg-purple-100', icon: '✅' },
    failure: { bg: 'bg-red-100', icon: '❌' },
    think: { bg: 'bg-slate-100', icon: '💭' },
  };
  const { bg, icon } = icons[action] ?? icons.think;
  return <div className={`w-7 h-7 rounded-full ${bg} flex items-center justify-center text-sm shrink-0`}>{icon}</div>;
}

function TranscriptTimeline({ steps }: { steps: AgentStep[] }) {
  return (
    <div className="space-y-2">
      {steps.map((step) => (
        <div key={step.stepNumber} className="flex items-start gap-3">
          <StepIcon action={step.action} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <span>Step {step.stepNumber}</span>
              <span>{step.action.replace('_', ' ')}</span>
              <span>{step.durationMs}ms</span>
            </div>
            {step.toolName && (
              <div className="text-xs font-mono text-blue-600 mt-0.5">
                {step.toolName}({step.toolArgs ? JSON.stringify(step.toolArgs).slice(0, 80) : ''})
              </div>
            )}
            <p className="text-sm text-slate-700 mt-0.5 whitespace-pre-wrap leading-relaxed">
              {step.action === 'tool_result'
                ? (typeof step.toolResult === 'string' ? step.toolResult.slice(0, 200) : JSON.stringify(step.toolResult)?.slice(0, 200) + '...')
                : step.reasoning.slice(0, 500)}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

function ScenarioCard({ scenario, index }: { scenario: ShoppingScenario; index: number }) {
  const [expanded, setExpanded] = useState(false);
  const outcomeColors: Record<string, string> = {
    completed: 'bg-emerald-100 text-emerald-700',
    failed: 'bg-red-100 text-red-700',
    abandoned: 'bg-amber-100 text-amber-700',
  };

  return (
    <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-5 py-4 flex items-center gap-4 text-left hover:bg-slate-50 transition-colors"
      >
        <div className="text-lg font-bold text-slate-300 w-6 text-center">{index + 1}</div>
        <div className="flex-1 min-w-0">
          <div className="font-medium text-slate-900 text-sm truncate">"{scenario.intent}"</div>
          <div className="text-xs text-slate-400 mt-0.5">
            {scenario.persona !== 'default' && <span className="capitalize">{scenario.persona} buyer · </span>}
            {scenario.totalSteps} steps · {(scenario.durationMs / 1000).toFixed(1)}s
          </div>
        </div>
        <span className={`px-2.5 py-0.5 text-xs font-medium rounded-full ${outcomeColors[scenario.outcome] ?? 'bg-slate-100 text-slate-600'}`}>
          {scenario.outcome}
        </span>
        <svg className={`w-4 h-4 text-slate-400 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
        </svg>
      </button>

      {expanded && (
        <div className="px-5 pb-5 border-t border-slate-100 pt-4 space-y-4">
          {/* Selected product */}
          {scenario.selectedProduct && (
            <div className="bg-emerald-50 border border-emerald-200 rounded-lg p-4">
              <div className="text-xs font-semibold text-emerald-700 uppercase tracking-wider mb-1">Agent's Selection</div>
              <div className="font-medium text-slate-900">{scenario.selectedProduct.name}</div>
              {scenario.selectedProduct.price != null && scenario.selectedProduct.price > 0 && (
                <div className="text-sm text-slate-600">${scenario.selectedProduct.price}</div>
              )}
              <p className="text-sm text-emerald-700 mt-2 italic">"{scenario.selectedProduct.reason}"</p>
            </div>
          )}

          {/* Failure point */}
          {scenario.failurePoint && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-4">
              <div className="text-xs font-semibold text-red-700 uppercase tracking-wider mb-1">Failure Point — Step {scenario.failurePoint.stepNumber}</div>
              <p className="text-sm text-red-700 font-medium">{scenario.failurePoint.reason}</p>
              {scenario.failurePoint.context && (
                <p className="text-xs text-red-600 mt-1">{scenario.failurePoint.context}</p>
              )}
            </div>
          )}

          {/* Transcript */}
          <div>
            <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-3">Transcript</div>
            <TranscriptTimeline steps={scenario.steps} />
          </div>
        </div>
      )}
    </div>
  );
}

function SimulationSection({ simulation }: { simulation: AgentSimulation }) {
  const personas = [...new Set(simulation.scenarios.map(s => s.persona))];
  const [activePersona, setActivePersona] = useState(personas[0] ?? 'default');
  const filtered = simulation.scenarios.filter(s => s.persona === activePersona);

  const completed = simulation.scenarios.filter(s => s.outcome === 'completed').length;
  const total = simulation.scenarios.length;

  return (
    <div className="flex flex-col gap-5">
      <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-widest">
        Simulated Shopping Experience
        <span className="ml-2 px-2 py-0.5 text-xs rounded-full bg-purple-100 text-purple-700 font-medium normal-case tracking-normal">
          {completed}/{total} completed
        </span>
      </h2>

      {/* Persona tabs (only if multiple) */}
      {personas.length > 1 && (
        <div className="flex gap-2">
          {personas.map(p => (
            <button
              key={p}
              onClick={() => setActivePersona(p)}
              className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors capitalize ${
                activePersona === p
                  ? 'bg-purple-100 text-purple-700'
                  : 'bg-slate-100 text-slate-500 hover:bg-slate-200'
              }`}
            >
              {p === 'default' ? 'Balanced' : p} buyer
            </button>
          ))}
        </div>
      )}

      {/* Scenario cards */}
      <div className="space-y-3">
        {filtered.map((scenario, i) => (
          <ScenarioCard key={`${scenario.persona}-${i}`} scenario={scenario} index={i} />
        ))}
      </div>

      {/* Meta info */}
      <div className="text-xs text-slate-400 flex flex-wrap gap-4">
        <span>Model: {simulation.modelUsed}</span>
        <span>Scenarios: {total}</span>
        <span>Cost: ${simulation.costEstimateUsd.toFixed(4)}</span>
        <span>Duration: {(simulation.durationMs / 1000).toFixed(1)}s</span>
      </div>
    </div>
  );
}

export default function ScanResultPage() {
  const { domain } = useParams<{ domain: string }>();
  const navigate = useNavigate();
  const [result, setResult] = useState<ScanResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loadingMsgIdx, setLoadingMsgIdx] = useState(0);
  const [newDomain, setNewDomain] = useState('');
  const [copied, setCopied] = useState(false);
  const [badgeCopied, setBadgeCopied] = useState(false);
  const [fixToken, setFixToken] = useState<string | null>(() => sessionStorage.getItem('mcplens_fix_token'));
  const [fixUnlocked, setFixUnlocked] = useState(false);

  // Check if existing token is valid on mount
  useEffect(() => {
    if (fixToken) {
      leadsApi.verify(fixToken).then(res => {
        if (res.valid) setFixUnlocked(true);
        else {
          sessionStorage.removeItem('mcplens_fix_token');
          setFixToken(null);
        }
      });
    }
  }, [fixToken]);

  useEffect(() => {
    if (!domain) return;

    let cancelled = false;
    let intervalId: ReturnType<typeof setInterval> | null = null;

    async function run() {
      setLoading(true);
      setError(null);
      setResult(null);
      setLoadingMsgIdx(0);

      // Cycle loading messages
      intervalId = setInterval(() => {
        setLoadingMsgIdx(i => (i + 1) % LOADING_MESSAGES.length);
      }, 3500);

      try {
        // Check cache first
        const cached = await scanApi.getLatest(domain!);
        if (!cancelled && cached) {
          setResult(cached);
          setLoading(false);
          if (intervalId) clearInterval(intervalId);
          return;
        }

        // Trigger fresh scan
        const fresh = await scanApi.trigger(domain!);
        if (!cancelled) {
          setResult(fresh);
          setLoading(false);
          if (intervalId) clearInterval(intervalId);
        }
      } catch (err: unknown) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Scan failed. Please try again.');
          setLoading(false);
          if (intervalId) clearInterval(intervalId);
        }
      }
    }

    run();
    return () => {
      cancelled = true;
      if (intervalId) clearInterval(intervalId);
    };
  }, [domain]);

  // Update document title and OG meta tags when results arrive
  useEffect(() => {
    if (!domain) return;
    if (result) {
      const score = result.compositeScore;
      const apiBase = window.location.origin + '/api';
      const catSummary = result.categories
        .filter(c => c.tested)
        .map(c => `${CATEGORY_LABELS[c.category] ?? c.category}: ${Math.round(c.cappedScore)}`)
        .join(' | ');

      document.title = `${domain} — Agent Readiness: ${score}/100 | MCPLens`;
      setMetaTag('og:title', `${domain} — Agent Readiness: ${score}/100`);
      setMetaTag('og:description', catSummary || `AI agent readiness score for ${domain}`);
      setMetaTag('og:image', `${apiBase}/og/${domain}`);
      setMetaTag('og:url', window.location.href);
      setMetaTag('og:type', 'website');
      setMetaTag('twitter:card', 'summary_large_image');
      setMetaTag('twitter:title', `${domain} — Agent Readiness: ${score}/100`);
      setMetaTag('twitter:description', catSummary || `AI agent readiness score for ${domain}`);
      setMetaTag('twitter:image', `${apiBase}/og/${domain}`);
    } else {
      document.title = `${domain} — MCPLens`;
    }
  }, [result, domain]);

  function handleScanAnother(e: FormEvent) {
    e.preventDefault();
    const trimmed = newDomain.trim().replace(/^https?:\/\//, '').replace(/\/$/, '');
    if (!trimmed) return;
    navigate(`/scan/${trimmed}`);
  }

  function handleCopyUrl() {
    navigator.clipboard.writeText(window.location.href).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  function handleCopyBadge() {
    const apiBase = window.location.origin + '/api';
    const badgeMarkdown = `![Agent Readiness](https://img.shields.io/endpoint?url=${apiBase}/badge/${domain})`;
    navigator.clipboard.writeText(badgeMarkdown).then(() => {
      setBadgeCopied(true);
      setTimeout(() => setBadgeCopied(false), 2000);
    });
  }

  const categoriesWithKillers = result?.categories.filter(
    c => c.scoreKillers && c.scoreKillers.length > 0
  ) ?? [];

  return (
    <div className="min-h-screen bg-slate-50 text-slate-700">
      {/* Nav */}
      <header className="border-b border-slate-200 px-6 py-4 bg-white/80 backdrop-blur-lg">
        <div className="max-w-4xl mx-auto flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-slate-900 text-lg">MCPLens</span>
          </Link>
          <Link to="/scan" className="text-sm text-slate-500 hover:text-slate-900 transition-colors">
            New Scan
          </Link>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-12">
        {/* Domain header */}
        <div className="mb-8">
          <div className="text-xs text-slate-400 uppercase tracking-widest mb-1">Scan Results</div>
          <h1 className="text-2xl font-bold text-slate-900">{domain}</h1>
        </div>

        {/* Loading state */}
        {loading && (
          <div className="flex flex-col items-center justify-center py-24 gap-6">
            <div className="w-12 h-12 border-2 border-blue-200 border-t-blue-500 rounded-full animate-spin" />
            <div className="text-slate-500 text-sm animate-pulse">
              {LOADING_MESSAGES[loadingMsgIdx]}
            </div>
            <div className="text-xs text-slate-400">This may take up to 30 seconds</div>
          </div>
        )}

        {/* Error state */}
        {!loading && error && (
          <div className="flex flex-col items-center justify-center py-16 gap-6">
            <div className="bg-red-50 border border-red-200 rounded-xl p-6 max-w-lg w-full text-center">
              <div className="text-red-700 font-semibold mb-2">Scan Failed</div>
              <div className="text-red-600 text-sm">{error}</div>
            </div>
            <button
              onClick={() => navigate('/scan')}
              className="px-5 py-2.5 bg-blue-500 hover:bg-blue-600 text-white font-semibold rounded-full transition-colors text-sm"
            >
              Try Another Store
            </button>
          </div>
        )}

        {/* Results */}
        {!loading && result && (
          <div className="flex flex-col gap-8">
            {/* Score Hero */}
            <div className="bg-white border border-slate-200 rounded-2xl shadow-sm p-8 flex flex-col sm:flex-row items-center gap-8">
              <ScoreCircle score={result.compositeScore} />
              <div className="flex-1 text-center sm:text-left">
                <h2 className="text-xl font-bold text-slate-900 mb-2">
                  AI Agent Readiness Score
                </h2>
                <p className="text-slate-500 text-sm mb-4 leading-relaxed">
                  Composite score across {result.testedCategories.length} tested categories
                  from {result.scenarioCount} scenario{result.scenarioCount !== 1 ? 's' : ''}.
                </p>
                <div className="flex flex-wrap gap-4 justify-center sm:justify-start text-xs text-slate-400">
                  <span>Version: {result.version}</span>
                  <span>Duration: {(result.durationMs / 1000).toFixed(1)}s</span>
                  {result.partialResults && (
                    <span className="text-amber-600">Partial scan</span>
                  )}
                </div>
              </div>
            </div>

            {/* Score Cap Alerts */}
            {categoriesWithKillers.length > 0 && (
              <div className="flex flex-col gap-3">
                <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-widest">Score Cap Alerts</h2>
                {categoriesWithKillers.map(cat => (
                  <ScoreKillerAlert key={cat.category} cat={cat} />
                ))}
              </div>
            )}

            {/* Category Breakdown */}
            <div>
              <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-widest mb-4">Category Breakdown</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {result.categories.map(cat => (
                  <CategoryCard key={cat.category} cat={cat} />
                ))}
              </div>
            </div>

            {/* AI Quality Assessment — show results or upsell */}
            {result.aiAssessment ? (
              <AIAssessmentSection assessment={result.aiAssessment} />
            ) : (
              <div className="bg-gradient-to-br from-indigo-50 to-blue-50 border border-indigo-200 rounded-2xl p-6">
                <div className="flex items-start gap-4">
                  <div className="w-10 h-10 rounded-full bg-indigo-100 flex items-center justify-center shrink-0">
                    <svg className="w-5 h-5 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
                    </svg>
                  </div>
                  <div>
                    <h3 className="font-semibold text-slate-900 mb-1">Want deeper analysis?</h3>
                    <p className="text-sm text-slate-600 mb-3">
                      Max plan includes AI-powered quality assessment — product relevance scoring, description analysis, simulated buyer queries, and specific fix recommendations with code snippets.
                    </p>
                    <Link
                      to="/plan"
                      className="inline-flex items-center px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white text-sm font-semibold rounded-lg transition-colors"
                    >
                      Upgrade to Max
                    </Link>
                  </div>
                </div>
              </div>
            )}

            {/* Agent Simulation — show results or upsell */}
            {result.agentSimulation ? (
              <SimulationSection simulation={result.agentSimulation} />
            ) : (
              <div className="bg-gradient-to-br from-purple-50 to-indigo-50 border border-purple-200 rounded-2xl p-6">
                <div className="flex items-start gap-4">
                  <div className="w-10 h-10 rounded-full bg-purple-100 flex items-center justify-center shrink-0">
                    <svg className="w-5 h-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z" />
                    </svg>
                  </div>
                  <div>
                    <h3 className="font-semibold text-slate-900 mb-1">See how AI agents actually shop your store</h3>
                    <p className="text-sm text-slate-600 mb-3">
                      Max plan includes simulated buyer agents — watch step-by-step as an AI tries to find products, compare options, and complete a purchase on your store.
                    </p>
                    <Link
                      to="/plan"
                      className="inline-flex items-center px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white text-sm font-semibold rounded-lg transition-colors"
                    >
                      Upgrade to Max
                    </Link>
                  </div>
                </div>
              </div>
            )}

            {/* Findings with email-gated fixes */}
            {(() => {
              const findings = generateFindings(result);
              if (findings.length === 0) return null;
              return (
                <div>
                  <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-widest mb-4">
                    Issues Found ({findings.length})
                  </h2>
                  {!fixUnlocked && (
                    <div className="mb-4">
                      <EmailGate
                        domain={domain!}
                        score={result.compositeScore}
                        onUnlocked={(token) => {
                          setFixToken(token);
                          setFixUnlocked(true);
                        }}
                      />
                    </div>
                  )}
                  <div className="flex flex-col gap-3">
                    {findings.map((f, i) => (
                      <FindingCard key={i} finding={f} fixUnlocked={fixUnlocked} />
                    ))}
                  </div>
                  {fixUnlocked && (
                    <div className="mt-4 bg-emerald-50 border border-emerald-200 rounded-xl p-4 text-center">
                      <p className="text-sm text-emerald-800 font-medium">
                        Want automated scans, CI/CD integration, and store tracking?
                      </p>
                      <Link
                        to="/scan"
                        className="inline-block mt-2 px-5 py-2 bg-emerald-600 hover:bg-emerald-700 text-white font-semibold rounded-lg transition-colors text-sm"
                      >
                        View Pro Plans
                      </Link>
                    </div>
                  )}
                </div>
              );
            })()}

            {/* Partial scan notice */}
            {result.partialResults && result.partialReason && (
              <div className="bg-amber-50 border border-amber-200 rounded-xl p-4">
                <div className="text-amber-700 font-semibold text-sm mb-1">Partial Scan</div>
                <div className="text-amber-600 text-xs">{result.partialReason}</div>
              </div>
            )}

            {/* Share section */}
            <div className="bg-white border border-slate-200 rounded-2xl shadow-sm p-6 flex flex-col sm:flex-row items-center gap-4">
              <div className="flex-1">
                <div className="text-sm font-semibold text-slate-900 mb-1">Share this result</div>
                <div className="text-xs text-slate-400 truncate">{window.location.href}</div>
              </div>
              <button
                onClick={handleCopyUrl}
                className="px-4 py-2 bg-white text-slate-700 border border-slate-300 hover:bg-slate-50 text-sm font-medium rounded-full transition-colors shrink-0"
              >
                {copied ? 'Copied!' : 'Copy URL'}
              </button>
            </div>

            {/* Badge section */}
            <div className="bg-white border border-slate-200 rounded-2xl shadow-sm p-6 flex flex-col gap-4">
              <div>
                <div className="text-sm font-semibold text-slate-900 mb-1">README Badge</div>
                <div className="text-xs text-slate-500 mb-3">
                  Add this badge to your README to display the agent readiness score.
                </div>
                <div className="flex items-center gap-3">
                  <img
                    src={`https://img.shields.io/endpoint?url=${window.location.origin}/api/badge/${domain}`}
                    alt="Agent Readiness Badge"
                    className="h-5"
                  />
                </div>
              </div>
              <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3">
                <code className="flex-1 text-xs text-slate-700 bg-slate-100 border border-slate-200 rounded-lg px-3 py-2 font-mono break-all">
                  {`![Agent Readiness](https://img.shields.io/endpoint?url=${window.location.origin}/api/badge/${domain})`}
                </code>
                <button
                  onClick={handleCopyBadge}
                  className="px-4 py-2 bg-white text-slate-700 border border-slate-300 hover:bg-slate-50 text-sm font-medium rounded-full transition-colors shrink-0"
                >
                  {badgeCopied ? 'Copied!' : 'Copy Markdown'}
                </button>
              </div>
            </div>

            {/* Scan Another Store */}
            <div className="bg-white border border-slate-200 rounded-2xl shadow-sm p-6">
              <div className="text-sm font-semibold text-slate-900 mb-4">Scan Another Store</div>
              <form onSubmit={handleScanAnother} className="flex flex-col sm:flex-row gap-3">
                <input
                  type="text"
                  value={newDomain}
                  onChange={e => setNewDomain(e.target.value)}
                  placeholder="Enter a Shopify store domain (e.g., allbirds.com)"
                  className="flex-1 px-4 py-2.5 bg-white border border-slate-300 rounded-lg text-slate-900 text-sm placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                />
                <button
                  type="submit"
                  disabled={!newDomain.trim()}
                  className="px-5 py-2.5 bg-blue-500 hover:bg-blue-600 disabled:opacity-60 disabled:cursor-not-allowed text-white font-semibold rounded-full transition-colors text-sm shrink-0"
                >
                  Scan
                </button>
              </form>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-200 px-6 py-6 mt-12 bg-white">
        <div className="max-w-4xl mx-auto text-center text-sm text-slate-400">
          &copy; {new Date().getFullYear()} MCPLens
        </div>
      </footer>
    </div>
  );
}
