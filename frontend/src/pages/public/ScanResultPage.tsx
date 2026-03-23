import { useState, useEffect, type FormEvent } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { scanApi, type ScanResult, type ScanCategoryResult } from '../../api/client';

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
];

function scoreColor(score: number): string {
  if (score >= 80) return '#22c55e';
  if (score >= 50) return '#f59e0b';
  return '#ef4444';
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
            stroke="#1e293b"
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
          <span className="text-4xl font-bold text-white">{Math.round(score)}</span>
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
    <div className="bg-[#1e293b] border border-slate-700/50 rounded-xl p-5 flex flex-col gap-3">
      <div className="flex items-start justify-between gap-2">
        <h3 className="font-semibold text-white text-sm">{label}</h3>
        {cat.tested ? (
          <span className="px-2 py-0.5 text-xs rounded-full bg-slate-700 text-slate-300 shrink-0">Tested</span>
        ) : (
          <span className="px-2 py-0.5 text-xs rounded-full bg-slate-800 text-slate-500 shrink-0">Not Tested</span>
        )}
      </div>

      <div className="flex items-end justify-between">
        <div>
          <div className="text-3xl font-bold" style={{ color }}>
            {cat.tested ? Math.round(cat.cappedScore) : '—'}
          </div>
          {cat.tested && cat.score !== cat.cappedScore && (
            <div className="text-xs text-slate-500 mt-0.5">
              Raw: {Math.round(cat.score)} (capped)
            </div>
          )}
        </div>
        <div className="text-right">
          <div className="text-xs text-slate-400">Weight</div>
          <div className="text-sm font-medium text-slate-300">{Math.round(cat.weight * 100)}%</div>
        </div>
      </div>

      {cat.tested && (
        <div className="w-full h-1.5 bg-slate-700 rounded-full overflow-hidden">
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
    <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-4">
      <div className="flex items-start gap-3">
        <div className="text-red-400 text-lg mt-0.5">!</div>
        <div>
          <div className="text-red-400 font-semibold text-sm mb-1">
            Score Cap Active — {label}
          </div>
          {cat.scoreKillers.map((sk, i) => (
            <div key={i} className="text-red-300/80 text-xs">
              {sk.condition} — capped at {sk.cap}
            </div>
          ))}
        </div>
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
    <div className="min-h-screen bg-[#0f172a] text-slate-200">
      {/* Nav */}
      <header className="border-b border-slate-800 px-6 py-4">
        <div className="max-w-4xl mx-auto flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-sky-400 to-sky-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-white text-lg">MCPLens</span>
          </Link>
          <Link to="/scan" className="text-sm text-slate-400 hover:text-white transition-colors">
            New Scan
          </Link>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-12">
        {/* Domain header */}
        <div className="mb-8">
          <div className="text-xs text-slate-500 uppercase tracking-widest mb-1">Scan Results</div>
          <h1 className="text-2xl font-bold text-white">{domain}</h1>
        </div>

        {/* Loading state */}
        {loading && (
          <div className="flex flex-col items-center justify-center py-24 gap-6">
            <div className="w-12 h-12 border-2 border-sky-500/30 border-t-sky-500 rounded-full animate-spin" />
            <div className="text-slate-400 text-sm animate-pulse">
              {LOADING_MESSAGES[loadingMsgIdx]}
            </div>
            <div className="text-xs text-slate-600">This may take up to 30 seconds</div>
          </div>
        )}

        {/* Error state */}
        {!loading && error && (
          <div className="flex flex-col items-center justify-center py-16 gap-6">
            <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-6 max-w-lg w-full text-center">
              <div className="text-red-400 font-semibold mb-2">Scan Failed</div>
              <div className="text-red-300/80 text-sm">{error}</div>
            </div>
            <button
              onClick={() => navigate('/scan')}
              className="px-5 py-2.5 bg-sky-500 hover:bg-sky-400 text-white font-semibold rounded-lg transition-colors text-sm"
            >
              Try Another Store
            </button>
          </div>
        )}

        {/* Results */}
        {!loading && result && (
          <div className="flex flex-col gap-8">
            {/* Score Hero */}
            <div className="bg-[#1e293b] border border-slate-700/50 rounded-2xl p-8 flex flex-col sm:flex-row items-center gap-8">
              <ScoreCircle score={result.compositeScore} />
              <div className="flex-1 text-center sm:text-left">
                <h2 className="text-xl font-bold text-white mb-2">
                  AI Agent Readiness Score
                </h2>
                <p className="text-slate-400 text-sm mb-4 leading-relaxed">
                  Composite score across {result.testedCategories.length} tested categories
                  from {result.scenarioCount} scenario{result.scenarioCount !== 1 ? 's' : ''}.
                </p>
                <div className="flex flex-wrap gap-4 justify-center sm:justify-start text-xs text-slate-500">
                  <span>Version: {result.version}</span>
                  <span>Duration: {(result.durationMs / 1000).toFixed(1)}s</span>
                  {result.partialResults && (
                    <span className="text-amber-400">Partial scan</span>
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

            {/* Partial scan notice */}
            {result.partialResults && result.partialReason && (
              <div className="bg-amber-500/10 border border-amber-500/20 rounded-xl p-4">
                <div className="text-amber-400 font-semibold text-sm mb-1">Partial Scan</div>
                <div className="text-amber-300/80 text-xs">{result.partialReason}</div>
              </div>
            )}

            {/* Share section */}
            <div className="bg-[#1e293b] border border-slate-700/50 rounded-2xl p-6 flex flex-col sm:flex-row items-center gap-4">
              <div className="flex-1">
                <div className="text-sm font-semibold text-white mb-1">Share this result</div>
                <div className="text-xs text-slate-400 truncate">{window.location.href}</div>
              </div>
              <button
                onClick={handleCopyUrl}
                className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white text-sm font-medium rounded-lg transition-colors shrink-0"
              >
                {copied ? 'Copied!' : 'Copy URL'}
              </button>
            </div>

            {/* Badge section */}
            <div className="bg-[#1e293b] border border-slate-700/50 rounded-2xl p-6 flex flex-col gap-4">
              <div>
                <div className="text-sm font-semibold text-white mb-1">README Badge</div>
                <div className="text-xs text-slate-400 mb-3">
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
                <code className="flex-1 text-xs text-slate-300 bg-[#0f172a] border border-slate-700 rounded-lg px-3 py-2 font-mono break-all">
                  {`![Agent Readiness](https://img.shields.io/endpoint?url=${window.location.origin}/api/badge/${domain})`}
                </code>
                <button
                  onClick={handleCopyBadge}
                  className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white text-sm font-medium rounded-lg transition-colors shrink-0"
                >
                  {badgeCopied ? 'Copied!' : 'Copy Markdown'}
                </button>
              </div>
            </div>

            {/* Scan Another Store */}
            <div className="bg-[#1e293b] border border-slate-700/50 rounded-2xl p-6">
              <div className="text-sm font-semibold text-white mb-4">Scan Another Store</div>
              <form onSubmit={handleScanAnother} className="flex flex-col sm:flex-row gap-3">
                <input
                  type="text"
                  value={newDomain}
                  onChange={e => setNewDomain(e.target.value)}
                  placeholder="Enter a Shopify store domain (e.g., allbirds.com)"
                  className="flex-1 px-4 py-2.5 bg-[#0f172a] border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:outline-none focus:border-sky-500 transition-colors"
                />
                <button
                  type="submit"
                  disabled={!newDomain.trim()}
                  className="px-5 py-2.5 bg-sky-500 hover:bg-sky-400 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold rounded-lg transition-colors text-sm shrink-0"
                >
                  Scan
                </button>
              </form>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800 px-6 py-6 mt-12">
        <div className="max-w-4xl mx-auto text-center text-sm text-slate-500">
          © {new Date().getFullYear()} MCPLens
        </div>
      </footer>
    </div>
  );
}
