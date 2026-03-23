import { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';

function formatRevenue(value: number): string {
  if (value >= 1_000_000) {
    const m = value / 1_000_000;
    return m % 1 === 0 ? `$${m}M` : `$${m.toFixed(1)}M`;
  }
  if (value >= 1_000) {
    const k = value / 1_000;
    return k % 1 === 0 ? `$${k}K` : `$${k.toFixed(0)}K`;
  }
  return `$${value.toLocaleString()}`;
}

function formatDollar(value: number): string {
  if (value >= 1_000_000) {
    return `$${(value / 1_000_000).toFixed(2)}M`;
  }
  if (value >= 1_000) {
    return `$${Math.round(value / 1_000)}K`;
  }
  return `$${Math.round(value).toLocaleString()}`;
}

function useAnimatedNumber(target: number, duration = 400): number {
  const [displayed, setDisplayed] = useState(target);
  const rafRef = useRef<number | null>(null);
  const startRef = useRef<number | null>(null);
  const fromRef = useRef(target);

  useEffect(() => {
    const from = fromRef.current;
    const to = target;
    if (from === to) return;

    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
    }

    startRef.current = null;

    const step = (timestamp: number) => {
      if (startRef.current === null) startRef.current = timestamp;
      const elapsed = timestamp - startRef.current;
      const progress = Math.min(elapsed / duration, 1);
      // ease-out cubic
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = from + (to - from) * eased;
      setDisplayed(current);

      if (progress < 1) {
        rafRef.current = requestAnimationFrame(step);
      } else {
        fromRef.current = to;
      }
    };

    rafRef.current = requestAnimationFrame(step);
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    };
  }, [target, duration]);

  return displayed;
}

export default function RevenueCalculator() {
  const [revenue, setRevenue] = useState(100_000);
  const [score, setScore] = useState(50);
  const [aov, setAov] = useState(75);

  const agentCommerceShare = 0.05;
  const scoreGap = (100 - score) / 100;
  const agentConversionMultiplier = 5;

  const monthlyLoss = revenue * agentCommerceShare * scoreGap * agentConversionMultiplier;
  const annualLoss = monthlyLoss * 12;

  const agentEligibleRevenue = revenue * agentCommerceShare;
  const scorePenaltyPct = scoreGap * 100;
  const monthlyMissed = agentEligibleRevenue * scoreGap * agentConversionMultiplier;

  const animatedAnnual = useAnimatedNumber(annualLoss);

  return (
    <section className="px-6 py-16 border-t border-slate-200">
      <div className="max-w-4xl mx-auto">
        <h2 className="text-2xl sm:text-3xl font-bold text-slate-900 text-center tracking-tight mb-3">
          How much is a low score costing you?
        </h2>
        <p className="text-center text-slate-500 mb-10 max-w-xl mx-auto">
          Adjust the sliders to estimate your annual revenue at risk from agent commerce.
        </p>

        <div className="bg-white rounded-2xl border border-slate-200 shadow-md p-8">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-10">
            {/* Inputs column */}
            <div className="flex flex-col gap-8">
              {/* Slider 1: Monthly revenue */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-semibold text-slate-700">
                    Monthly store revenue
                  </label>
                  <span className="text-sm font-mono font-bold text-blue-600">
                    {formatRevenue(revenue)}
                  </span>
                </div>
                <input
                  type="range"
                  min={10_000}
                  max={10_000_000}
                  step={10_000}
                  value={revenue}
                  onChange={(e) => setRevenue(Number(e.target.value))}
                  className="w-full h-2 rounded-full appearance-none cursor-pointer accent-blue-500 bg-slate-200"
                />
                <div className="flex justify-between text-xs text-slate-400 mt-1">
                  <span>$10K</span>
                  <span>$10M</span>
                </div>
              </div>

              {/* Slider 2: Agent readiness score */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-semibold text-slate-700">
                    Your agent readiness score
                  </label>
                  <span className="text-sm font-mono font-bold text-blue-600">
                    {score}/100
                  </span>
                </div>
                <input
                  type="range"
                  min={0}
                  max={100}
                  step={1}
                  value={score}
                  onChange={(e) => setScore(Number(e.target.value))}
                  className="w-full h-2 rounded-full appearance-none cursor-pointer accent-blue-500 bg-slate-200"
                />
                <div className="flex justify-between text-xs text-slate-400 mt-1">
                  <span>0</span>
                  <span className="text-center">
                    Don't know?{' '}
                    <Link to="/scan" className="text-blue-500 hover:underline">
                      Scan your store above.
                    </Link>
                  </span>
                  <span>100</span>
                </div>
              </div>

              {/* Slider 3: Average order value */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-semibold text-slate-700">
                    Average order value
                  </label>
                  <span className="text-sm font-mono font-bold text-blue-600">
                    ${aov}
                  </span>
                </div>
                <input
                  type="range"
                  min={20}
                  max={500}
                  step={5}
                  value={aov}
                  onChange={(e) => setAov(Number(e.target.value))}
                  className="w-full h-2 rounded-full appearance-none cursor-pointer accent-blue-500 bg-slate-200"
                />
                <div className="flex justify-between text-xs text-slate-400 mt-1">
                  <span>$20</span>
                  <span>$500</span>
                </div>
              </div>
            </div>

            {/* Output column */}
            <div className="flex flex-col justify-center items-center text-center gap-6">
              <div>
                <p className="text-sm text-slate-500 mb-2">
                  You're leaving an estimated
                </p>
                <div
                  className="text-4xl sm:text-5xl font-bold text-blue-600 tabular-nums"
                  style={{ transition: 'color 0.2s' }}
                >
                  {formatDollar(animatedAnnual)}/yr
                </div>
                <p className="text-sm text-slate-500 mt-2">on the table</p>
              </div>

              {/* Breakdown */}
              <div className="w-full bg-slate-50 rounded-xl border border-slate-200 p-4 text-left space-y-2">
                <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">
                  Breakdown
                </p>
                <div className="flex justify-between text-sm text-slate-600">
                  <span>Agent-eligible revenue</span>
                  <span className="font-mono font-medium text-slate-800">
                    {formatDollar(agentEligibleRevenue)}/mo
                    <span className="text-slate-400 font-normal"> (5%)</span>
                  </span>
                </div>
                <div className="flex justify-between text-sm text-slate-600">
                  <span>Score gap penalty</span>
                  <span className="font-mono font-medium text-slate-800">
                    {Math.round(scorePenaltyPct)}% lost
                  </span>
                </div>
                <div className="flex justify-between text-sm text-slate-600">
                  <span>5x agent conversion edge</span>
                  <span className="font-mono font-medium text-red-600">
                    −{formatDollar(monthlyMissed)}/mo missed
                  </span>
                </div>
                <div className="border-t border-slate-200 pt-2 flex justify-between text-sm font-semibold text-slate-800">
                  <span>Annual opportunity cost</span>
                  <span className="text-blue-600">{formatDollar(annualLoss)}</span>
                </div>
              </div>

              <Link
                to="/scan"
                className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-blue-500 hover:bg-blue-600 text-white font-semibold rounded-full transition-colors text-sm w-full sm:w-auto"
              >
                Scan your store to get your real score &rarr;
              </Link>

              <p className="text-xs text-slate-400 max-w-xs">
                Estimate based on industry benchmarks.
                Sources: McKinsey 2026, UCP Hub.
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
