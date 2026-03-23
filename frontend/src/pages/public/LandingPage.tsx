import { useState, type FormEvent, useEffect } from 'react';
import { Navigate, Link, useNavigate } from 'react-router-dom';
import DOMPurify from 'dompurify';
import { useBranding } from '../../contexts/BrandingContext';
import { useAuth } from '../../contexts/AuthContext';

export default function LandingPage() {
  const { branding, loaded } = useBranding();
  const { isAuthenticated } = useAuth();
  const navigate = useNavigate();
  const [heroDomain, setHeroDomain] = useState('');

  useEffect(() => {
    if (!loaded) return;
    const prevTitle = document.title;
    const title =
      branding.landingEnabled && branding.landingTitle
        ? branding.landingTitle
        : 'MCPLens — AI Agent Readiness Scanner';
    document.title = title;

    let metaDesc = document.querySelector(
      'meta[name="description"]'
    ) as HTMLMetaElement | null;
    const meta =
      branding.landingEnabled && branding.landingMeta
        ? branding.landingMeta
        : "MCPLens scans your store's public MCP endpoint and shows exactly how well AI buyer agents can find, evaluate, and purchase your products.";
    if (meta) {
      if (!metaDesc) {
        metaDesc = document.createElement('meta');
        metaDesc.name = 'description';
        document.head.appendChild(metaDesc);
      }
      metaDesc.content = meta;
    }

    return () => {
      document.title = prevTitle;
      if (metaDesc) metaDesc.content = '';
    };
  }, [loaded, branding.landingEnabled, branding.landingTitle, branding.landingMeta]);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  if (!loaded) return null;

  if (branding.landingEnabled && branding.landingHtml) {
    return (
      <div
        className="min-h-screen bg-[#0f172a]"
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(branding.landingHtml) }}
      />
    );
  }

  function handleHeroSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = heroDomain.trim().replace(/^https?:\/\//, '').replace(/\/$/, '');
    if (!trimmed) return;
    navigate(`/scan/${trimmed}`);
  }

  return (
    <div className="min-h-screen bg-[#0f172a] text-slate-200 flex flex-col">
      {/* ── Nav ── */}
      <header className="border-b border-slate-800 px-6 py-4 sticky top-0 z-50 bg-[#0f172a]/90 backdrop-blur">
        <div className="max-w-5xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-sky-400 to-sky-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-white text-lg">MCPLens</span>
          </div>
          <nav className="flex items-center gap-6">
            <a href="#pricing" className="text-sm text-slate-400 hover:text-white transition-colors hidden sm:block">
              Pricing
            </a>
            <a
              href="https://github.com/reesthomas212/mcplens"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-slate-400 hover:text-white transition-colors hidden sm:block"
            >
              GitHub
            </a>
            <Link
              to="/login"
              className="text-sm text-slate-400 hover:text-white transition-colors"
            >
              Sign in
            </Link>
          </nav>
        </div>
      </header>

      <main className="flex-1">
        {/* ── Hero ── */}
        <section className="px-6 py-24 sm:py-32">
          <div className="max-w-2xl mx-auto text-center w-full">
            <div className="inline-flex items-center gap-2 bg-sky-500/10 border border-sky-500/20 rounded-full px-4 py-1.5 text-sm text-sky-400 mb-8">
              AI Agent Readiness Scanner
            </div>

            <h1 className="text-4xl sm:text-5xl font-bold text-white leading-tight mb-6">
              How agent-ready is your Shopify store?
            </h1>

            <p className="text-lg text-slate-400 leading-relaxed mb-10">
              MCPLens scans your store&apos;s public MCP endpoint and shows exactly how well AI
              buyer agents can find, evaluate, and purchase your products.
            </p>

            <form
              onSubmit={handleHeroSubmit}
              className="flex flex-col sm:flex-row gap-3 w-full max-w-xl mx-auto"
            >
              <input
                type="text"
                value={heroDomain}
                onChange={(e) => setHeroDomain(e.target.value)}
                placeholder="allbirds.com"
                className="flex-1 px-4 py-3 bg-[#1e293b] border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:outline-none focus:border-sky-500 transition-colors"
                autoFocus
              />
              <button
                type="submit"
                disabled={!heroDomain.trim()}
                className="px-6 py-3 bg-sky-500 hover:bg-sky-400 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold rounded-lg transition-colors whitespace-nowrap"
              >
                Scan Free →
              </button>
            </form>

            <p className="mt-4 text-sm text-slate-500">No signup required. Free forever.</p>
          </div>
        </section>

        {/* ── Social Proof / Stats ── */}
        <section className="px-6 py-16 border-t border-slate-800">
          <div className="max-w-5xl mx-auto">
            <p className="text-center text-sm text-slate-500 uppercase tracking-widest mb-10">
              Trusted by developers building agent-ready commerce
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
              {[
                {
                  stat: '5.5M+',
                  label: 'Shopify stores with MCP endpoints',
                },
                {
                  stat: '15–30%',
                  label: 'Agent-assisted conversion rate (vs 3% traditional)',
                },
                {
                  stat: '10 scenarios',
                  label: 'Across 4 categories, scored 0–100',
                },
              ].map(({ stat, label }) => (
                <div
                  key={stat}
                  className="bg-[#1e293b] rounded-xl p-6 text-center border border-slate-700"
                >
                  <div className="text-3xl font-bold text-sky-400 mb-2">{stat}</div>
                  <div className="text-sm text-slate-400">{label}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ── How It Works ── */}
        <section className="px-6 py-16 border-t border-slate-800">
          <div className="max-w-5xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-white text-center mb-12">
              How it works
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-8 relative">
              {[
                {
                  step: '01',
                  icon: (
                    <svg className="w-7 h-7 text-sky-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35M17 11A6 6 0 1 1 5 11a6 6 0 0 1 12 0z" />
                    </svg>
                  ),
                  title: 'Enter any Shopify domain',
                  desc: 'Paste in a store URL — no credentials or API keys needed.',
                },
                {
                  step: '02',
                  icon: (
                    <svg className="w-7 h-7 text-sky-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 0 1-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.3 24.3 0 0 1 4.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15M14.25 3.104c.251.023.501.05.75.082M19.8 15l-1.322.977A7.5 7.5 0 0 1 12 18a7.5 7.5 0 0 1-6.478-2.023L4.2 15m15.6 0-4.2-3.073" />
                    </svg>
                  ),
                  title: 'We scan the MCP endpoint',
                  desc: 'MCPLens runs 10 agent commerce scenarios against the live endpoint.',
                },
                {
                  step: '03',
                  icon: (
                    <svg className="w-7 h-7 text-sky-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
                    </svg>
                  ),
                  title: 'Get a scored report with fixes',
                  desc: 'See exactly where agents fail and what to fix — prioritized by impact.',
                },
              ].map(({ step, icon, title, desc }) => (
                <div key={step} className="flex flex-col items-center text-center gap-4">
                  <div className="w-14 h-14 rounded-2xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center">
                    {icon}
                  </div>
                  <div className="text-xs font-mono text-sky-500 tracking-widest">{step}</div>
                  <h3 className="text-lg font-semibold text-white">{title}</h3>
                  <p className="text-sm text-slate-400 leading-relaxed">{desc}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ── Scoring Categories ── */}
        <section className="px-6 py-16 border-t border-slate-800">
          <div className="max-w-5xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-white text-center mb-4">
              What gets scored
            </h2>
            <p className="text-center text-slate-400 mb-12 max-w-xl mx-auto">
              Every scan produces a 0–100 score across four weighted categories.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
              {[
                {
                  name: 'Data Quality',
                  weight: '35%',
                  color: 'bg-emerald-500',
                  question: 'Are your products described well enough for agents to compare?',
                },
                {
                  name: 'Product Discovery',
                  weight: '30%',
                  color: 'bg-sky-500',
                  question: 'Can agents search and filter your catalog effectively?',
                },
                {
                  name: 'Checkout Flow',
                  weight: '25%',
                  color: 'bg-violet-500',
                  question: 'Can agents add to cart and initiate purchase?',
                },
                {
                  name: 'Protocol Compliance',
                  weight: '10%',
                  color: 'bg-amber-500',
                  question: 'Does your MCP endpoint follow the spec?',
                },
              ].map(({ name, weight, color, question }) => (
                <div
                  key={name}
                  className="bg-[#1e293b] rounded-xl p-6 border border-slate-700 flex flex-col gap-3"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-white">{name}</span>
                    <span className="text-sm font-mono text-slate-400">{weight}</span>
                  </div>
                  <div className="w-full bg-slate-700 rounded-full h-1.5">
                    <div
                      className={`${color} h-1.5 rounded-full`}
                      style={{ width: weight }}
                    />
                  </div>
                  <p className="text-sm text-slate-400">{question}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ── Agencies CTA ── */}
        <section className="px-6 py-16 border-t border-slate-800">
          <div className="max-w-3xl mx-auto bg-gradient-to-br from-sky-500/10 to-violet-500/10 border border-sky-500/20 rounded-2xl p-10 text-center">
            <h2 className="text-2xl sm:text-3xl font-bold text-white mb-4">
              Agencies: Win clients with agent readiness audits
            </h2>
            <p className="text-slate-400 mb-8 max-w-xl mx-auto">
              Scan any prospect&apos;s store before the first call. Show them exactly where
              they&apos;re losing to competitors who have already optimized for AI buyer agents.
            </p>
            <Link
              to="/scan"
              className="inline-flex items-center justify-center gap-2 px-8 py-3 bg-sky-500 hover:bg-sky-400 text-white font-semibold rounded-lg transition-colors"
            >
              Start scanning →
            </Link>
          </div>
        </section>

        {/* ── Pricing ── */}
        <section id="pricing" className="px-6 py-16 border-t border-slate-800">
          <div className="max-w-5xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-white text-center mb-4">
              Simple, transparent pricing
            </h2>
            <p className="text-center text-slate-400 mb-12">
              Start free. Upgrade when you need history and tracking.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-6 items-start">
              {[
                {
                  name: 'Free',
                  price: '$0',
                  period: 'forever',
                  cta: 'Get Started',
                  highlight: false,
                  features: [
                    'Unlimited scans',
                    'HTML reports',
                    'CLI access',
                    'CI/CD integration',
                  ],
                },
                {
                  name: 'Starter',
                  price: '$19',
                  period: 'per month',
                  cta: 'Start Trial',
                  highlight: true,
                  features: [
                    '5 tracked stores',
                    'Score history',
                    'Weekly scheduled scans',
                    'Email alerts',
                  ],
                },
                {
                  name: 'Agency',
                  price: '$49',
                  period: 'per month',
                  cta: 'Start Trial',
                  highlight: false,
                  features: [
                    '25 tracked stores',
                    'Daily scans',
                    'White-label reports',
                    'API access',
                  ],
                },
              ].map(({ name, price, period, cta, highlight, features }) => (
                <div
                  key={name}
                  className={`rounded-xl p-7 border flex flex-col gap-6 ${
                    highlight
                      ? 'bg-sky-500/10 border-sky-500/40 ring-1 ring-sky-500/30'
                      : 'bg-[#1e293b] border-slate-700'
                  }`}
                >
                  {highlight && (
                    <div className="text-xs font-semibold text-sky-400 uppercase tracking-widest -mb-2">
                      Most popular
                    </div>
                  )}
                  <div>
                    <div className="text-lg font-bold text-white mb-1">{name}</div>
                    <div className="flex items-end gap-1">
                      <span className="text-4xl font-bold text-white">{price}</span>
                      <span className="text-slate-400 mb-1 text-sm">/ {period}</span>
                    </div>
                  </div>
                  <ul className="flex flex-col gap-2">
                    {features.map((f) => (
                      <li key={f} className="flex items-start gap-2 text-sm text-slate-300">
                        <svg
                          className="w-4 h-4 mt-0.5 text-sky-400 shrink-0"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth={2}
                          viewBox="0 0 24 24"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                        </svg>
                        {f}
                      </li>
                    ))}
                  </ul>
                  <Link
                    to="/scan"
                    className={`mt-auto inline-flex items-center justify-center px-5 py-2.5 rounded-lg font-semibold text-sm transition-colors ${
                      highlight
                        ? 'bg-sky-500 hover:bg-sky-400 text-white'
                        : 'bg-slate-700 hover:bg-slate-600 text-white'
                    }`}
                  >
                    {cta}
                  </Link>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ── CLI Section ── */}
        <section className="px-6 py-16 border-t border-slate-800">
          <div className="max-w-3xl mx-auto text-center">
            <h2 className="text-2xl sm:text-3xl font-bold text-white mb-4">
              Works in your terminal too
            </h2>
            <p className="text-slate-400 mb-8">
              Open source. MIT licensed. Works in CI/CD pipelines.
            </p>

            <div className="bg-[#1e293b] border border-slate-700 rounded-xl px-6 py-5 text-left mb-8 font-mono text-sm">
              <span className="text-slate-500 select-none">$ </span>
              <span className="text-sky-400">npx</span>
              <span className="text-white"> mcplens scan allbirds.com</span>
            </div>

            <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
              <a
                href="https://github.com/reesthomas212/mcplens"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-white text-sm font-medium rounded-lg transition-colors"
              >
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0 0 22 12.017C22 6.484 17.522 2 12 2Z" />
                </svg>
                GitHub
              </a>
              <a
                href="https://www.npmjs.com/package/mcplens"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-slate-800 hover:bg-slate-700 border border-slate-700 text-white text-sm font-medium rounded-lg transition-colors"
              >
                <svg className="w-4 h-4 text-red-400" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M0 0v24h6.5V6H12v18h6V0H0z" />
                </svg>
                npm
              </a>
            </div>
          </div>
        </section>
      </main>

      {/* ── Footer ── */}
      <footer className="border-t border-slate-800 px-6 py-10">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-sky-400 to-sky-600 flex items-center justify-center">
              <span className="text-white font-bold text-xs">ML</span>
            </div>
            <span className="font-semibold text-white">MCPLens</span>
          </div>

          <nav className="flex flex-wrap items-center justify-center gap-5 text-sm text-slate-500">
            <Link to="/scan" className="hover:text-white transition-colors">
              Scan a Store
            </Link>
            <a href="#pricing" className="hover:text-white transition-colors">
              Pricing
            </a>
            <a
              href="https://github.com/reesthomas212/mcplens"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-white transition-colors"
            >
              GitHub
            </a>
            <a href="#" className="hover:text-white transition-colors">
              Docs
            </a>
          </nav>

          <div className="text-sm text-slate-500 text-center sm:text-right">
            <div>Built with Claude Code</div>
            <div>© {new Date().getFullYear()} MCPLens</div>
          </div>
        </div>
      </footer>
    </div>
  );
}
