import { useState, type FormEvent, useEffect } from 'react';
import { Navigate, Link, useNavigate } from 'react-router-dom';
import DOMPurify from 'dompurify';
import { motion } from 'framer-motion';
import { useBranding } from '../../contexts/BrandingContext';
import { useAuth } from '../../contexts/AuthContext';
import ScanDemo from '../../components/ScanDemo';

// ── Reduced motion ──────────────────────────────────────────────────────────
const prefersReducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// ── Animation variants ──────────────────────────────────────────────────────
const fadeInUp = prefersReducedMotion
  ? undefined
  : {
      hidden: { opacity: 0, y: 20 },
      visible: {
        opacity: 1,
        y: 0,
        transition: { duration: 0.5, ease: [0.25, 0.46, 0.45, 0.94] as const },
      },
    };

const staggerContainer = prefersReducedMotion
  ? undefined
  : {
      hidden: {},
      visible: { transition: { staggerChildren: 0.075 } },
    };

const cardHover = prefersReducedMotion
  ? {}
  : {
      whileHover: { y: -4, boxShadow: '0 10px 25px rgba(0,0,0,0.08)' },
      transition: { duration: 0.2 },
    };

// Shared props for scroll-triggered sections
function sectionMotionProps(): Record<string, unknown> {
  if (prefersReducedMotion) return {};
  return {
    initial: 'hidden' as const,
    whileInView: 'visible' as const,
    viewport: { once: true, amount: 0.2 as const },
    variants: fadeInUp,
  };
}

// ── Component ───────────────────────────────────────────────────────────────
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
        className="min-h-screen bg-slate-50"
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
    <div className="min-h-screen bg-slate-50 text-slate-700 flex flex-col">
      {/* ── Nav ── */}
      <header className="border-b border-slate-200 px-6 py-4 sticky top-0 z-50 bg-white/80 backdrop-blur-lg">
        <div className="max-w-5xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-slate-900 text-lg">MCPLens</span>
          </div>
          <nav className="flex items-center gap-6">
            <a href="#pricing" className="text-sm text-slate-500 hover:text-slate-900 transition-colors hidden sm:block">
              Pricing
            </a>
            <a
              href="https://github.com/reesthomas212/mcplens"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-slate-500 hover:text-slate-900 transition-colors hidden sm:block"
            >
              GitHub
            </a>
            <Link
              to="/login"
              className="text-sm text-slate-500 hover:text-slate-900 transition-colors hidden sm:block"
            >
              Sign in
            </Link>
            <Link
              to="/signup"
              className="text-sm px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white font-semibold rounded-full transition-colors"
            >
              Get Started
            </Link>
          </nav>
        </div>
      </header>

      <main className="flex-1">
        {/* ── Hero ── */}
        <section className="px-6 py-20 sm:py-28">
          <div className="max-w-5xl mx-auto">
            <div className="flex flex-col lg:flex-row lg:items-center lg:gap-16">
              {/* Left: copy + form */}
              <div className="flex-1 text-center lg:text-left">
                <motion.div
                  initial={prefersReducedMotion ? false : { opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.5, delay: 0 }}
                  className="inline-flex items-center gap-2 bg-blue-50 border border-blue-200 rounded-full px-4 py-1.5 text-sm text-blue-600 mb-8"
                >
                  AI Agent Readiness Scanner
                </motion.div>

                <motion.h1
                  initial={prefersReducedMotion ? false : { opacity: 0, y: 15 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.6, delay: 0.1 }}
                  className="text-4xl sm:text-5xl font-bold text-slate-900 leading-tight tracking-tight mb-6"
                >
                  Are AI agents skipping your store?
                </motion.h1>

                <motion.p
                  initial={prefersReducedMotion ? false : { opacity: 0, y: 15 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.6, delay: 0.3 }}
                  className="text-lg text-slate-500 leading-relaxed mb-10"
                >
                  If AI agents can&apos;t find, compare, or purchase your products, you&apos;re
                  already losing sales you&apos;ll never see.
                </motion.p>

                <motion.form
                  initial={prefersReducedMotion ? false : { opacity: 0, y: 15 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.6, delay: 0.45 }}
                  onSubmit={handleHeroSubmit}
                  className="flex flex-col sm:flex-row gap-3 w-full max-w-xl mx-auto lg:mx-0"
                >
                  <input
                    type="text"
                    value={heroDomain}
                    onChange={(e) => setHeroDomain(e.target.value)}
                    placeholder="allbirds.com"
                    className="flex-1 px-4 py-3 bg-white border border-slate-300 rounded-lg text-slate-900 text-sm placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                    autoFocus
                  />
                  <button
                    type="submit"
                    disabled={!heroDomain.trim()}
                    className="px-6 py-3 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold rounded-full transition-colors whitespace-nowrap"
                  >
                    Scan Free &rarr;
                  </button>
                </motion.form>

                <motion.p
                  initial={prefersReducedMotion ? false : { opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.5, delay: 0.6 }}
                  className="mt-4 text-sm text-slate-400"
                >
                  Get a scored report instantly. No credit card. No follow-up emails.
                </motion.p>
                <motion.p
                  initial={prefersReducedMotion ? false : { opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.5, delay: 0.65 }}
                  className="mt-1 text-sm text-slate-400"
                >
                  We scan only public endpoints &mdash; no store credentials needed.
                </motion.p>
              </div>

              {/* Right: animated product demo */}
              <motion.div
                initial={prefersReducedMotion ? false : { opacity: 0, x: 24 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.7, delay: 0.5 }}
                className="mt-14 lg:mt-0 lg:w-[340px] shrink-0"
              >
                <ScanDemo />
              </motion.div>
            </div>
          </div>
        </section>

        {/* ── Why This Matters Now ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-3xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 text-center tracking-tight mb-10">
              Agent commerce is here. Most stores aren&apos;t ready.
            </h2>

            <div className="space-y-6 text-slate-600 leading-relaxed">
              <p>
                AI-assisted shopping converts at 15&ndash;30%, compared to 3% for traditional
                browsing{' '}
                <span className="text-slate-400">(UCP Hub, 2026; McKinsey, 2026)</span>.
                Stores that work well with AI agents don&apos;t just keep up &mdash; they
                pull ahead.
              </p>
              <p>
                Every Shopify store now has a public MCP endpoint &mdash; the interface AI
                agents use to shop your store. Agents from ChatGPT, Gemini, and others are
                already using these endpoints to browse, compare, and buy.
              </p>
              <p>
                But most stores have critical gaps: missing price data, incomplete
                descriptions, broken checkout flows. These gaps make your store invisible to
                agents &mdash; or worse, make agents choose a competitor instead.
              </p>
              <p className="text-blue-600 font-semibold">
                MCPLens finds these gaps in seconds.
              </p>
            </div>
          </div>
        </motion.section>

        {/* ── How It Works ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-5xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 text-center tracking-tight mb-12">
              How it works
            </h2>
            <motion.div
              className="grid grid-cols-1 sm:grid-cols-3 gap-8 relative"
              initial="hidden"
              whileInView="visible"
              viewport={{ once: true, amount: 0.2 }}
              variants={staggerContainer}
            >
              {[
                {
                  step: '01',
                  icon: (
                    <svg className="w-7 h-7 text-blue-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35M17 11A6 6 0 1 1 5 11a6 6 0 0 1 12 0z" />
                    </svg>
                  ),
                  title: 'Enter any Shopify domain',
                  desc: 'Paste in a store URL — no credentials or API keys needed.',
                },
                {
                  step: '02',
                  icon: (
                    <svg className="w-7 h-7 text-blue-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 0 1-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.3 24.3 0 0 1 4.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15M14.25 3.104c.251.023.501.05.75.082M19.8 15l-1.322.977A7.5 7.5 0 0 1 12 18a7.5 7.5 0 0 1-6.478-2.023L4.2 15m15.6 0-4.2-3.073" />
                    </svg>
                  ),
                  title: 'We scan the public endpoint',
                  desc: 'MCPLens runs 10 agent commerce scenarios against the live interface AI agents use to shop your store.',
                },
                {
                  step: '03',
                  icon: (
                    <svg className="w-7 h-7 text-blue-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
                    </svg>
                  ),
                  title: 'Get a scored report with fixes',
                  desc: 'See exactly where agents fail and what to fix — prioritized by impact.',
                },
              ].map(({ step, icon, title, desc }) => (
                <motion.div
                  key={step}
                  variants={fadeInUp}
                  className="flex flex-col items-center text-center gap-4"
                >
                  <div className="w-14 h-14 rounded-2xl bg-blue-50 border border-blue-200 flex items-center justify-center">
                    {icon}
                  </div>
                  <div className="text-xs font-mono text-blue-500 tracking-widest">{step}</div>
                  <h3 className="text-lg font-semibold text-slate-900">{title}</h3>
                  <p className="text-sm text-slate-500 leading-relaxed">{desc}</p>
                </motion.div>
              ))}
            </motion.div>
          </div>
        </motion.section>

        {/* ── Sample Report Preview ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-3xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 text-center tracking-tight mb-4">
              See what you get
            </h2>
            <p className="text-center text-slate-500 mb-10">
              Every scan produces a detailed report. Here&apos;s what one looks like.
            </p>

            <motion.div
              {...(prefersReducedMotion ? {} : cardHover)}
              className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden cursor-default"
            >
              <div className="px-6 py-5 border-b border-slate-200 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                  <div className="text-xs font-mono text-slate-400 uppercase tracking-wider mb-1">
                    Agent Readiness Report
                  </div>
                  <div className="text-slate-900 font-semibold text-lg">example-store.com</div>
                </div>
                <div className="flex items-center gap-3">
                  <div className="text-4xl font-bold text-amber-500">47</div>
                  <div className="text-sm text-slate-400 leading-tight">
                    <div>out of</div>
                    <div>100</div>
                  </div>
                </div>
              </div>

              <div className="px-6 py-5 grid grid-cols-2 sm:grid-cols-4 gap-4">
                {[
                  { name: 'Data Quality', score: 31, color: 'text-red-600' },
                  { name: 'Discovery', score: 62, color: 'text-blue-600' },
                  { name: 'Checkout', score: 45, color: 'text-amber-600' },
                  { name: 'Protocol', score: 89, color: 'text-emerald-600' },
                ].map(({ name, score, color }) => (
                  <div key={name} className="text-center">
                    <div className={`text-2xl font-bold ${color}`}>{score}</div>
                    <div className="text-xs text-slate-400 mt-1">{name}</div>
                  </div>
                ))}
              </div>

              <div className="px-6 py-4 border-t border-slate-200 bg-amber-50">
                <div className="flex items-start gap-3">
                  <span className="text-amber-500 text-lg leading-none mt-0.5">&#9888;</span>
                  <div>
                    <div className="text-sm text-slate-900 font-medium">
                      34% of products missing price data
                    </div>
                    <div className="text-sm text-slate-600 mt-1">
                      Agents can&apos;t compare or purchase these products. This directly
                      reduces your visibility in agent-assisted shopping.
                    </div>
                  </div>
                </div>
              </div>
            </motion.div>
          </div>
        </motion.section>

        {/* ── Trust / Real Data ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-5xl mx-auto">
            <motion.div
              className="grid grid-cols-1 sm:grid-cols-3 gap-6"
              initial="hidden"
              whileInView="visible"
              viewport={{ once: true, amount: 0.2 }}
              variants={staggerContainer}
            >
              {[
                { stat: '100+', label: 'Stores scanned' },
                { stat: '11 – 100', label: 'Score range across scans' },
                { stat: '~15 sec', label: 'Average scan time' },
              ].map(({ stat, label }) => (
                <motion.div
                  key={stat}
                  variants={fadeInUp}
                  {...(prefersReducedMotion ? {} : cardHover)}
                  className="bg-white rounded-xl p-6 text-center border border-slate-200 shadow-sm cursor-default"
                >
                  <div className="text-3xl font-bold text-blue-500 mb-2">{stat}</div>
                  <div className="text-sm text-slate-500">{label}</div>
                </motion.div>
              ))}
            </motion.div>
          </div>
        </motion.section>

        {/* ── Scoring Categories ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-5xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 text-center tracking-tight mb-4">
              What gets scored
            </h2>
            <p className="text-center text-slate-500 mb-12 max-w-xl mx-auto">
              Every scan produces a 0&ndash;100 score across four weighted categories.
            </p>
            <motion.div
              className="grid grid-cols-1 sm:grid-cols-2 gap-6"
              initial="hidden"
              whileInView="visible"
              viewport={{ once: true, amount: 0.2 }}
              variants={staggerContainer}
            >
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
                  color: 'bg-blue-500',
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
                  question: 'Does the public endpoint follow the spec correctly?',
                },
              ].map(({ name, weight, color, question }) => (
                <motion.div
                  key={name}
                  variants={fadeInUp}
                  {...(prefersReducedMotion ? {} : cardHover)}
                  className="bg-white rounded-xl p-6 border border-slate-200 shadow-sm flex flex-col gap-3 cursor-default"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-slate-900">{name}</span>
                    <span className="text-sm font-mono text-slate-400">{weight}</span>
                  </div>
                  <div className="w-full bg-slate-200 rounded-full h-1.5">
                    <div
                      className={`${color} h-1.5 rounded-full`}
                      style={{ width: weight }}
                    />
                  </div>
                  <p className="text-sm text-slate-500">{question}</p>
                </motion.div>
              ))}
            </motion.div>
          </div>
        </motion.section>

        {/* ── Before / After ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-3xl mx-auto">
            <motion.div
              className="grid grid-cols-1 sm:grid-cols-2 gap-6"
              initial="hidden"
              whileInView="visible"
              viewport={{ once: true, amount: 0.2 }}
              variants={staggerContainer}
            >
              <motion.div
                variants={fadeInUp}
                className="bg-white border border-slate-200 rounded-xl shadow-sm p-7"
              >
                <div className="text-xs font-mono text-red-600 uppercase tracking-widest mb-4">
                  Before MCPLens
                </div>
                <p className="text-slate-600 leading-relaxed">
                  You deployed your Shopify store. AI agents visit. Some buy. Most don&apos;t.
                  You have no idea why.
                </p>
              </motion.div>
              <motion.div
                variants={fadeInUp}
                className="bg-blue-50 border border-blue-200 rounded-xl p-7"
              >
                <div className="text-xs font-mono text-blue-600 uppercase tracking-widest mb-4">
                  After MCPLens
                </div>
                <p className="text-slate-700 leading-relaxed">
                  You scan your store. You see exactly where agents fail. You fix the gaps.
                  Your agent conversion rate climbs.
                </p>
              </motion.div>
            </motion.div>
          </div>
        </motion.section>

        {/* ── Agencies CTA ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-3xl mx-auto bg-gradient-to-br from-blue-50 to-violet-50 border border-blue-200 rounded-2xl p-10 text-center">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 tracking-tight mb-4">
              Win clients with data, not decks
            </h2>
            <p className="text-slate-600 mb-4 max-w-xl mx-auto">
              Scan any prospect&apos;s store before the first call. Show them exactly where
              they&apos;re losing to competitors. Close deals with evidence.
            </p>
            <p className="text-slate-700 mb-8 max-w-xl mx-auto font-medium">
              One agency scan can justify a $5,000/month retainer.
            </p>
            <Link
              to="/scan"
              className="inline-flex items-center justify-center gap-2 px-8 py-3 bg-blue-500 hover:bg-blue-600 text-white font-semibold rounded-full transition-colors"
            >
              Start scanning &rarr;
            </Link>
          </div>
        </motion.section>

        {/* ── Pricing ── */}
        <motion.section
          {...sectionMotionProps()}
          id="pricing"
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-5xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 text-center tracking-tight mb-4">
              Simple, transparent pricing
            </h2>
            <p className="text-center text-slate-500 mb-12">
              See the problems free. Pay for the solutions.
            </p>
            <motion.div
              className="grid grid-cols-1 sm:grid-cols-3 gap-6 items-start"
              initial="hidden"
              whileInView="visible"
              viewport={{ once: true, amount: 0.2 }}
              variants={staggerContainer}
            >
              {[
                {
                  name: 'Free',
                  price: '$0',
                  period: 'forever',
                  cta: 'Get Started',
                  highlight: false,
                  bestFor: 'Best for: seeing what is broken',
                  features: [
                    'Unlimited scans',
                    'Score + category breakdown',
                    'Issue detection',
                    'CLI access',
                  ],
                },
                {
                  name: 'Pro',
                  price: '$99',
                  period: 'per month',
                  cta: 'Start Pro',
                  highlight: true,
                  bestFor: 'Best for: store owners fixing their score',
                  features: [
                    'Fix instructions for every issue',
                    'CI/CD integration',
                    '5 tracked stores',
                    '90-day score history',
                    'Weekly automated scans',
                  ],
                },
                {
                  name: 'Agency',
                  price: '$199',
                  period: 'per month',
                  cta: 'Start Agency',
                  highlight: false,
                  bestFor: 'Best for: agencies landing clients with data',
                  features: [
                    '50 tracked stores',
                    'Daily scans',
                    'White-label reports',
                    'API access',
                    'Pitch deck + email swipes',
                  ],
                },
              ].map(({ name, price, period, cta, highlight, bestFor, features }) => (
                <motion.div
                  key={name}
                  variants={fadeInUp}
                  {...(prefersReducedMotion ? {} : cardHover)}
                  className={`rounded-xl p-7 border flex flex-col gap-6 cursor-default ${
                    highlight
                      ? 'bg-blue-50 border-blue-300 ring-1 ring-blue-300'
                      : 'bg-white border-slate-200 shadow-sm'
                  }`}
                >
                  {highlight && (
                    <div className="text-xs font-semibold text-blue-600 uppercase tracking-widest -mb-2">
                      Most popular
                    </div>
                  )}
                  <div>
                    <div className="text-lg font-bold text-slate-900 mb-1">{name}</div>
                    <div className="text-xs text-slate-500 mb-3">{bestFor}</div>
                    <div className="flex items-end gap-1">
                      <span className="text-4xl font-bold text-slate-900">{price}</span>
                      <span className="text-slate-500 mb-1 text-sm">/ {period}</span>
                    </div>
                  </div>
                  <ul className="flex flex-col gap-2">
                    {features.map((f) => (
                      <li key={f} className="flex items-start gap-2 text-sm text-slate-600">
                        <svg
                          className="w-4 h-4 mt-0.5 text-blue-500 shrink-0"
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
                    to="/signup"
                    className={`mt-auto inline-flex items-center justify-center px-5 py-2.5 rounded-full font-semibold text-sm transition-colors ${
                      highlight
                        ? 'bg-blue-500 hover:bg-blue-600 text-white'
                        : 'bg-white text-slate-700 border border-slate-300 hover:bg-slate-50'
                    }`}
                  >
                    {cta}
                  </Link>
                </motion.div>
              ))}
            </motion.div>
            <p className="text-center text-sm text-slate-400 mt-6">
              Cancel anytime. No lock-in. Start with Free &mdash; upgrade when you want to
              track scores over time.
            </p>
          </div>
        </motion.section>

        {/* ── CLI Section ── */}
        <motion.section
          {...sectionMotionProps()}
          className="px-6 py-16 border-t border-slate-200"
        >
          <div className="max-w-3xl mx-auto text-center">
            <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 tracking-tight mb-4">
              Works in your terminal too
            </h2>
            <p className="text-slate-500 mb-8">
              Open source. MIT licensed. Works in CI/CD pipelines.
            </p>

            <div className="bg-slate-900 border border-slate-700 rounded-xl px-6 py-5 text-left mb-8 font-mono text-sm">
              <span className="text-slate-500 select-none">$ </span>
              <span className="text-blue-400">npx</span>
              <span className="text-white"> mcplens scan allbirds.com</span>
            </div>

            <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
              <a
                href="https://github.com/reesthomas212/mcplens"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-white hover:bg-slate-50 border border-slate-300 text-slate-700 text-sm font-medium rounded-full transition-colors"
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
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-white hover:bg-slate-50 border border-slate-300 text-slate-700 text-sm font-medium rounded-full transition-colors"
              >
                <svg className="w-4 h-4 text-red-500" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M0 0v24h6.5V6H12v18h6V0H0z" />
                </svg>
                npm
              </a>
            </div>
          </div>
        </motion.section>
      </main>

      {/* ── Footer ── */}
      <footer className="border-t border-slate-200 px-6 py-10 bg-white">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center">
              <span className="text-white font-bold text-xs">ML</span>
            </div>
            <span className="font-semibold text-slate-900">MCPLens</span>
          </div>

          <nav className="flex flex-wrap items-center justify-center gap-5 text-sm text-slate-500">
            <Link to="/scan" className="hover:text-slate-900 transition-colors">
              Scan a Store
            </Link>
            <a href="#pricing" className="hover:text-slate-900 transition-colors">
              Pricing
            </a>
            <a
              href="https://github.com/reesthomas212/mcplens"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-slate-900 transition-colors"
            >
              GitHub
            </a>
            <a href="#" className="hover:text-slate-900 transition-colors">
              Docs
            </a>
            <Link to="/terms" className="hover:text-slate-900 transition-colors">
              Terms
            </Link>
            <Link to="/privacy" className="hover:text-slate-900 transition-colors">
              Privacy
            </Link>
          </nav>

          <div className="text-sm text-slate-400 text-center sm:text-right">
            <div>Free account, no credit card required</div>
            <div>&copy; {new Date().getFullYear()} MCPLens</div>
          </div>
        </div>
      </footer>
    </div>
  );
}
