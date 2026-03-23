import { useEffect, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';

// Reduced motion check — evaluated once at module load so it's stable
const prefersReducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

type Phase = 'input' | 'scanning' | 'results' | 'fade';

const SCAN_MESSAGES = [
  'Connecting to MCP endpoint...',
  'Running scenarios...',
  'Generating report...',
];

// Each demo store has different scores and a contextual finding
const DEMO_STORES = [
  {
    domain: 'gymshark.com',
    score: 97,
    badge: 'Excellent',
    badgeColor: 'bg-emerald-50 text-emerald-700',
    categories: [
      { label: 'Data Quality', score: 100, color: '#059669' },
      { label: 'Product Discovery', score: 95, color: '#059669' },
      { label: 'Checkout Flow', score: 92, color: '#059669' },
      { label: 'Protocol', score: 100, color: '#059669' },
    ],
    finding: { type: 'pass', icon: '✓', color: 'bg-emerald-50 border-emerald-200 text-emerald-600', text: 'All products have complete price data and structured attributes — agents can compare effectively.' },
  },
  {
    domain: 'colourpop.com',
    score: 11,
    badge: 'Critical',
    badgeColor: 'bg-red-50 text-red-700',
    categories: [
      { label: 'Data Quality', score: 8, color: '#dc2626' },
      { label: 'Product Discovery', score: 15, color: '#dc2626' },
      { label: 'Checkout Flow', score: 0, color: '#dc2626' },
      { label: 'Protocol', score: 22, color: '#dc2626' },
    ],
    finding: { type: 'fail', icon: '⚠', color: 'bg-red-50 border-red-200 text-red-600', text: 'Fix: Add price_range to search results — agents skip products they can\'t price-compare.' },
  },
  {
    domain: 'allbirds.com',
    score: 100,
    badge: 'Perfect',
    badgeColor: 'bg-emerald-50 text-emerald-700',
    categories: [
      { label: 'Data Quality', score: 100, color: '#059669' },
      { label: 'Product Discovery', score: 100, color: '#059669' },
      { label: 'Checkout Flow', score: 100, color: '#059669' },
      { label: 'Protocol', score: 100, color: '#059669' },
    ],
    finding: { type: 'pass', icon: '✓', color: 'bg-emerald-50 border-emerald-200 text-emerald-600', text: 'Perfect score — every product has rich descriptions, variants, and structured attributes.' },
  },
  {
    domain: 'ridgewallet.com',
    score: 36,
    badge: 'Needs Work',
    badgeColor: 'bg-amber-50 text-amber-700',
    categories: [
      { label: 'Data Quality', score: 42, color: '#d97706' },
      { label: 'Product Discovery', score: 28, color: '#dc2626' },
      { label: 'Checkout Flow', score: 35, color: '#dc2626' },
      { label: 'Protocol', score: 40, color: '#d97706' },
    ],
    finding: { type: 'fail', icon: '⚠', color: 'bg-amber-50 border-amber-200 text-amber-600', text: 'Fix: 38% of products have descriptions under 50 chars — agents can\'t differentiate them.' },
  },
  {
    domain: 'tentree.com',
    score: 86,
    badge: 'Good',
    badgeColor: 'bg-emerald-50 text-emerald-700',
    categories: [
      { label: 'Data Quality', score: 90, color: '#059669' },
      { label: 'Product Discovery', score: 82, color: '#059669' },
      { label: 'Checkout Flow', score: 80, color: '#059669' },
      { label: 'Protocol', score: 95, color: '#059669' },
    ],
    finding: { type: 'pass', icon: '✓', color: 'bg-emerald-50 border-emerald-200 text-emerald-600', text: 'Strong agent readiness — consider adding structured size/material attributes to reach 95+.' },
  },
];

/* ── Typewriter ── */
function Typewriter({ text, onDone }: { text: string; onDone?: () => void }) {
  const [displayed, setDisplayed] = useState('');
  const doneRef = useRef(false);

  useEffect(() => {
    doneRef.current = false;
    setDisplayed('');

    if (prefersReducedMotion) {
      setDisplayed(text);
      onDone?.();
      return;
    }

    let i = 0;
    const id = setInterval(() => {
      i += 1;
      setDisplayed(text.slice(0, i));
      if (i >= text.length) {
        clearInterval(id);
        if (!doneRef.current) {
          doneRef.current = true;
          onDone?.();
        }
      }
    }, 55);

    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  return <>{displayed}</>;
}

/* ── Animated score circle ── */
function DemoScoreCircle({ score, animate }: { score: number; animate: boolean }) {
  const [current, setCurrent] = useState(0);
  const radius = 40;
  const circumference = 2 * Math.PI * radius;

  useEffect(() => {
    if (!animate) return;
    if (prefersReducedMotion) { setCurrent(score); return; }
    let start: number | null = null;
    const duration = 1200;
    function step(ts: number) {
      if (start === null) start = ts;
      const progress = Math.min((ts - start) / duration, 1);
      setCurrent(Math.round(progress * score));
      if (progress < 1) requestAnimationFrame(step);
    }
    requestAnimationFrame(step);
  }, [animate, score]);

  const offset = circumference - (current / 100) * circumference;

  return (
    <div className="relative w-28 h-28 mx-auto">
      <svg className="w-full h-full -rotate-90" viewBox="0 0 96 96">
        <circle cx="48" cy="48" r={radius} stroke="#e2e8f0" strokeWidth="9" fill="none" />
        <circle
          cx="48"
          cy="48"
          r={radius}
          stroke="#059669"
          strokeWidth="9"
          fill="none"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-2xl font-bold text-slate-900">{current}</span>
        <span className="text-[10px] text-slate-400">/ 100</span>
      </div>
    </div>
  );
}

/* ── Phase: Input ── */
function PhaseInput({ domain, onTyped }: { domain: string; onTyped: () => void }) {
  const [typed, setTyped] = useState(false);

  function handleDone() {
    setTyped(true);
    setTimeout(onTyped, 700);
  }

  return (
    <motion.div
      key={`input-${domain}`}
      initial={prefersReducedMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.3 }}
      className="p-6"
    >
      <div className="text-xs font-mono text-slate-400 uppercase tracking-widest mb-4">
        MCPLens — AI Agent Readiness Scanner
      </div>
      <div className="flex gap-2 items-center">
        <div className="flex-1 px-3 py-2.5 bg-white border border-slate-300 rounded-lg text-slate-900 text-sm font-mono min-h-[40px]">
          <Typewriter text={domain} onDone={handleDone} />
          {!typed && (
            <span className="inline-block w-0.5 h-4 bg-slate-500 ml-0.5 animate-pulse align-middle" />
          )}
        </div>
        <motion.button
          initial={{ opacity: 0, scale: 0.9 }}
          animate={typed ? { opacity: 1, scale: 1 } : { opacity: 0, scale: 0.9 }}
          transition={{ duration: 0.2 }}
          className="px-4 py-2.5 bg-blue-500 text-white text-sm font-semibold rounded-full whitespace-nowrap"
        >
          Scan Free →
        </motion.button>
      </div>
      <p className="mt-3 text-xs text-slate-400">Scanning public endpoints — no credentials needed.</p>
    </motion.div>
  );
}

/* ── Phase: Scanning ── */
function PhaseScanning({ onDone }: { onDone: () => void }) {
  const [msgIdx, setMsgIdx] = useState(0);

  useEffect(() => {
    const msgs = SCAN_MESSAGES.length;
    let i = 0;
    const id = setInterval(() => {
      i += 1;
      if (i < msgs) {
        setMsgIdx(i);
      } else {
        clearInterval(id);
        onDone();
      }
    }, 650);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <motion.div
      key="scanning"
      initial={prefersReducedMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.3 }}
      className="p-6"
    >
      {/* Shimmer placeholder rows */}
      <div className="space-y-3 mb-5">
        {[0.9, 0.7, 0.5].map((w, i) => (
          <div
            key={i}
            className="h-3 rounded-full bg-slate-200 overflow-hidden"
            style={{ width: `${w * 100}%` }}
          >
            <div className="h-full w-full shimmer-bar" />
          </div>
        ))}
      </div>

      <div className="flex items-center gap-3">
        <div className="flex gap-1">
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              className="w-1.5 h-1.5 rounded-full bg-blue-400"
              style={{
                animation: `bounce 0.9s ${i * 0.15}s infinite`,
              }}
            />
          ))}
        </div>
        <AnimatePresence mode="wait">
          <motion.span
            key={msgIdx}
            initial={prefersReducedMotion ? false : { opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.25 }}
            className="text-sm text-slate-500"
          >
            {SCAN_MESSAGES[msgIdx]}
          </motion.span>
        </AnimatePresence>
      </div>
    </motion.div>
  );
}

/* ── Phase: Results ── */
function PhaseResults({ store }: { store: typeof DEMO_STORES[number] }) {
  const [scoreAnimate, setScoreAnimate] = useState(false);

  useEffect(() => {
    const id = setTimeout(() => setScoreAnimate(true), 100);
    return () => clearTimeout(id);
  }, []);

  const stagger = prefersReducedMotion ? 0 : 0.075;

  return (
    <motion.div
      key={`results-${store.domain}`}
      initial={prefersReducedMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.4 }}
      className="p-6"
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <div className="text-xs font-mono text-slate-400 uppercase tracking-widest mb-0.5">
            Agent Readiness Report
          </div>
          <div className="font-semibold text-slate-900 text-sm">{store.domain}</div>
        </div>
        <span className={`px-2 py-0.5 text-xs rounded-full font-medium ${store.badgeColor}`}>
          {store.badge}
        </span>
      </div>

      {/* Score circle */}
      <DemoScoreCircle score={store.score} animate={scoreAnimate} />

      {/* Category scores */}
      <motion.div
        className="mt-4 grid grid-cols-2 gap-2"
        initial="hidden"
        animate="visible"
        variants={{
          hidden: {},
          visible: { transition: { staggerChildren: stagger, delayChildren: 0.5 } },
        }}
      >
        {store.categories.map(({ label, score, color }) => (
          <motion.div
            key={label}
            variants={{
              hidden: prefersReducedMotion ? {} : { opacity: 0, y: 8 },
              visible: { opacity: 1, y: 0, transition: { duration: 0.35 } },
            }}
            className="bg-slate-50 border border-slate-200 rounded-lg px-3 py-2"
          >
            <div className="text-[11px] text-slate-500 mb-0.5">{label}</div>
            <div className="text-lg font-bold" style={{ color }}>
              {score}
            </div>
          </motion.div>
        ))}
      </motion.div>

      {/* Contextual finding — pass or fail with specific detail */}
      <motion.div
        initial={prefersReducedMotion ? false : { opacity: 0, x: -8 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ duration: 0.4, delay: 0.9 }}
        className={`mt-3 flex items-start gap-2 border rounded-lg px-3 py-2.5 ${store.finding.color}`}
      >
        <span className="font-bold text-sm flex-shrink-0">{store.finding.icon}</span>
        <span className="text-xs text-slate-700">{store.finding.text}</span>
      </motion.div>
    </motion.div>
  );
}

/* ── Main ScanDemo ── */
export default function ScanDemo() {
  const [phase, setPhase] = useState<Phase>('input');
  const [storeIdx, setStoreIdx] = useState(0);
  const currentStore = DEMO_STORES[storeIdx];

  useEffect(() => {
    if (phase === 'results') {
      const id = setTimeout(() => setPhase('fade'), 3500);
      return () => clearTimeout(id);
    }
    if (phase === 'fade') {
      const id = setTimeout(() => {
        setStoreIdx((prev) => (prev + 1) % DEMO_STORES.length);
        setPhase('input');
      }, 900);
      return () => clearTimeout(id);
    }
  }, [phase]);

  return (
    <div className="w-full max-w-sm mx-auto">
      <div className="bg-white rounded-2xl border border-slate-200 shadow-lg overflow-hidden">
        {/* Browser bar chrome */}
        <div className="flex items-center gap-1.5 px-4 py-3 border-b border-slate-100 bg-slate-50">
          <div className="w-2.5 h-2.5 rounded-full bg-red-400" />
          <div className="w-2.5 h-2.5 rounded-full bg-amber-400" />
          <div className="w-2.5 h-2.5 rounded-full bg-emerald-400" />
          <div className="flex-1 mx-3 h-5 bg-white border border-slate-200 rounded text-[10px] text-slate-400 flex items-center px-2">
            mcplens.dev
          </div>
        </div>

        <div className="min-h-[300px] relative">
          <AnimatePresence mode="wait">
            {phase === 'input' && (
              <PhaseInput domain={currentStore.domain} onTyped={() => setPhase('scanning')} />
            )}
            {phase === 'scanning' && (
              <PhaseScanning onDone={() => setPhase('results')} />
            )}
            {(phase === 'results' || phase === 'fade') && (
              <motion.div
                key={`results-wrap-${currentStore.domain}`}
                animate={phase === 'fade' ? { opacity: 0 } : { opacity: 1 }}
                transition={{ duration: 0.6 }}
              >
                <PhaseResults store={currentStore} />
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Store indicator dots */}
        <div className="flex justify-center gap-1.5 pb-3">
          {DEMO_STORES.map((_, i) => (
            <div
              key={i}
              className={`w-1.5 h-1.5 rounded-full transition-colors duration-300 ${
                i === storeIdx ? 'bg-blue-500' : 'bg-slate-200'
              }`}
            />
          ))}
        </div>
      </div>

      {/* Shimmer & bounce keyframes via inline style */}
      <style>{`
        @keyframes shimmer {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(200%); }
        }
        .shimmer-bar {
          background: linear-gradient(90deg, transparent 0%, rgba(148,163,184,0.4) 50%, transparent 100%);
          animation: shimmer 1.2s infinite;
        }
        @keyframes bounce {
          0%, 80%, 100% { transform: translateY(0); }
          40% { transform: translateY(-5px); }
        }
      `}</style>
    </div>
  );
}
