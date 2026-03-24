import { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, RefreshCw, TrendingUp, TrendingDown, Minus, Calendar, Clock, AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import { trackedStoresApi, scanApi } from '../../api/client';
import type { TrackedStore, StoredScanEntry } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import LoadingSpinner from '../../components/LoadingSpinner';

function ScoreColor({ score }: { score: number }) {
  const cls =
    score >= 80 ? 'text-accent-emerald' :
    score >= 60 ? 'text-yellow-400' :
    score >= 40 ? 'text-orange-400' :
    'text-red-400';
  return <span className={cls}>{score}</span>;
}

function ScoreBadge({ score }: { score: number }) {
  const bg =
    score >= 80 ? 'bg-accent-emerald/20 text-accent-emerald' :
    score >= 60 ? 'bg-yellow-400/20 text-yellow-400' :
    score >= 40 ? 'bg-orange-400/20 text-orange-400' :
    'bg-red-400/20 text-red-400';
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-bold ${bg}`}>
      {score}
    </span>
  );
}

function ScoreTimelineChart({ scans }: { scans: StoredScanEntry[] }) {
  if (scans.length === 0) return null;

  const sorted = [...scans].sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());
  const scores = sorted.map(s => s.compositeScore);
  const minScore = Math.max(0, Math.min(...scores) - 10);
  const maxScore = Math.min(100, Math.max(...scores) + 10);
  const range = maxScore - minScore || 1;

  const WIDTH = 600;
  const HEIGHT = 160;
  const PADDING = { top: 12, right: 16, bottom: 28, left: 36 };
  const chartW = WIDTH - PADDING.left - PADDING.right;
  const chartH = HEIGHT - PADDING.top - PADDING.bottom;

  const toX = (i: number) => PADDING.left + (i / Math.max(scores.length - 1, 1)) * chartW;
  const toY = (score: number) => PADDING.top + chartH - ((score - minScore) / range) * chartH;

  const points = scores.map((s, i) => `${toX(i)},${toY(s)}`).join(' ');
  const areaPoints = [
    `${PADDING.left},${PADDING.top + chartH}`,
    ...scores.map((s, i) => `${toX(i)},${toY(s)}`),
    `${toX(scores.length - 1)},${PADDING.top + chartH}`,
  ].join(' ');

  const yLabels = [minScore, Math.round((minScore + maxScore) / 2), maxScore];

  return (
    <div className="overflow-x-auto">
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="w-full"
        style={{ minWidth: '320px', maxHeight: '180px' }}
      >
        {/* Grid lines */}
        {yLabels.map((label) => (
          <g key={label}>
            <line
              x1={PADDING.left}
              x2={PADDING.left + chartW}
              y1={toY(label)}
              y2={toY(label)}
              stroke="var(--color-dark-800)"
              strokeWidth="1"
            />
            <text
              x={PADDING.left - 6}
              y={toY(label) + 4}
              textAnchor="end"
              fill="var(--color-dark-500)"
              fontSize="10"
            >
              {label}
            </text>
          </g>
        ))}

        {/* Area fill */}
        <polygon
          points={areaPoints}
          fill="rgba(99,102,241,0.1)"
        />

        {/* Line */}
        <polyline
          points={points}
          fill="none"
          stroke="#6366f1"
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
        />

        {/* Data points */}
        {scores.map((s, i) => (
          <circle
            key={i}
            cx={toX(i)}
            cy={toY(s)}
            r="3.5"
            fill="#6366f1"
            stroke="#ffffff"
            strokeWidth="1.5"
          />
        ))}

        {/* X-axis date labels — show first, last, and a couple mid-points */}
        {sorted.filter((_, i) => {
          if (sorted.length <= 4) return true;
          return i === 0 || i === sorted.length - 1 || i === Math.floor(sorted.length / 2);
        }).map((scan) => {
          const origIndex = sorted.indexOf(scan);
          return (
            <text
              key={origIndex}
              x={toX(origIndex)}
              y={HEIGHT - 6}
              textAnchor="middle"
              fill="var(--color-dark-500)"
              fontSize="9"
            >
              {new Date(scan.createdAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
            </text>
          );
        })}
      </svg>
    </div>
  );
}

function CategoryBreakdown({ scan }: { scan: StoredScanEntry }) {
  if (!scan.categories || scan.categories.length === 0) return null;
  const tested = scan.categories.filter(c => c.tested);
  if (tested.length === 0) return null;

  const CATEGORY_LABELS: Record<string, string> = {
    'data-quality': 'Data Quality',
    'product-discovery': 'Product Discovery',
    'checkout-flow': 'Checkout Flow',
    'protocol-compliance': 'Protocol Compliance',
  };

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
      {tested.map(cat => {
        const displayScore = Math.round(cat.cappedScore ?? cat.score);
        const barColor =
          displayScore >= 80 ? 'bg-accent-emerald' :
          displayScore >= 60 ? 'bg-yellow-400' :
          displayScore >= 40 ? 'bg-orange-400' :
          'bg-red-400';

        return (
          <div key={cat.category} className="bg-dark-800/50 rounded-xl p-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm text-dark-300">
                {CATEGORY_LABELS[cat.category] || cat.category}
              </span>
              <span className="text-sm font-semibold text-dark-100">{displayScore}</span>
            </div>
            <div className="h-1.5 bg-dark-700 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full ${barColor} transition-all`}
                style={{ width: `${Math.min(100, displayScore)}%` }}
              />
            </div>
            {cat.scoreKillers && cat.scoreKillers.length > 0 && (
              <p className="text-xs text-red-400 mt-1.5 flex items-start gap-1">
                <AlertCircle className="w-3 h-3 shrink-0 mt-0.5" />
                Score capped at {cat.scoreKillers[0].cap} — {cat.scoreKillers[0].condition}
              </p>
            )}
          </div>
        );
      })}
    </div>
  );
}

export default function StoreDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [store, setStore] = useState<TrackedStore | null>(null);
  const [history, setHistory] = useState<StoredScanEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);

  const load = () => {
    if (!id) return;
    setLoading(true);
    trackedStoresApi.history(id, 30)
      .then(data => {
        setStore(data.store);
        setHistory(data.history);
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [id]);

  const handleScanNow = async () => {
    if (!store) return;
    setScanning(true);
    try {
      await scanApi.trigger(store.domain);
      toast.success('Scan started — refreshing history');
      load();
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setScanning(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!store) {
    return (
      <div className="text-center py-20">
        <p className="text-dark-400">Store not found.</p>
        <Link to="/dashboard" className="text-primary-400 hover:underline mt-2 inline-block">
          Back to Dashboard
        </Link>
      </div>
    );
  }

  const latestScan = history[0] ?? null;
  const TrendIcon = store.trend === 'up' ? TrendingUp : store.trend === 'down' ? TrendingDown : Minus;
  const trendColor = store.trend === 'up' ? 'text-accent-emerald' : store.trend === 'down' ? 'text-red-400' : 'text-dark-400';

  return (
    <div>
      {/* Header */}
      <div className="flex items-center gap-4 mb-6">
        <button
          onClick={() => navigate('/dashboard')}
          className="p-2 rounded-lg text-dark-400 hover:text-dark-100 hover:bg-dark-800 transition-colors"
        >
          <ArrowLeft className="w-5 h-5" />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-bold text-dark-100 truncate">{store.domain}</h1>
          <p className="text-dark-400 text-sm mt-0.5">
            Added {new Date(store.addedAt).toLocaleDateString()}
            {store.lastScannedAt && (
              <> &middot; Last scanned {new Date(store.lastScannedAt).toLocaleDateString()}</>
            )}
          </p>
        </div>
        <button
          onClick={handleScanNow}
          disabled={scanning}
          className="flex items-center gap-2 px-4 py-2 bg-primary-500 hover:bg-primary-600 disabled:opacity-60 text-white text-sm font-medium rounded-lg transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${scanning ? 'animate-spin' : ''}`} />
          {scanning ? 'Scanning…' : 'Scan Now'}
        </button>
      </div>

      {/* Score Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-4">
          <p className="text-xs text-dark-400 uppercase tracking-wider mb-1">Current Score</p>
          <p className="text-4xl font-bold">
            <ScoreColor score={store.currentScore} />
          </p>
        </div>
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-4">
          <p className="text-xs text-dark-400 uppercase tracking-wider mb-1">Trend</p>
          <p className={`text-2xl font-semibold flex items-center gap-2 ${trendColor}`}>
            <TrendIcon className="w-6 h-6" />
            <span className="capitalize">{store.trend}</span>
          </p>
          {store.previousScore > 0 && store.previousScore !== store.currentScore && (
            <p className="text-xs text-dark-500 mt-1">
              Previous: {store.previousScore}
            </p>
          )}
        </div>
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-4">
          <p className="text-xs text-dark-400 uppercase tracking-wider mb-1">Total Scans</p>
          <p className="text-4xl font-bold text-dark-100">{history.length}</p>
        </div>
      </div>

      {/* Score Timeline */}
      {history.length > 1 && (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-5 mb-6">
          <h2 className="text-base font-semibold text-dark-100 mb-4">Score Over Time</h2>
          <ScoreTimelineChart scans={history} />
        </div>
      )}

      {/* Latest Scan Category Breakdown */}
      {latestScan && (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-5 mb-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-base font-semibold text-dark-100">Latest Scan Breakdown</h2>
            <div className="flex items-center gap-2 text-sm text-dark-400">
              <Calendar className="w-4 h-4" />
              {new Date(latestScan.createdAt).toLocaleString()}
              <Clock className="w-4 h-4 ml-2" />
              {(latestScan.durationMs / 1000).toFixed(1)}s
            </div>
          </div>
          <CategoryBreakdown scan={latestScan} />
        </div>
      )}

      {/* Scan History List */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
        <div className="px-5 py-4 border-b border-dark-800">
          <h2 className="text-base font-semibold text-dark-100">Scan History</h2>
          <p className="text-sm text-dark-400 mt-0.5">Most recent {history.length} scans</p>
        </div>
        {history.length === 0 ? (
          <div className="px-5 py-12 text-center">
            <p className="text-dark-400 text-sm">No scans yet. Run the first scan above.</p>
          </div>
        ) : (
          <div className="divide-y divide-dark-800/50">
            {history.map((scan, i) => {
              const prev = history[i + 1];
              const delta = prev ? scan.compositeScore - prev.compositeScore : null;
              return (
                <div key={scan.id} className="flex items-center justify-between px-5 py-3.5">
                  <div className="flex items-center gap-3">
                    <ScoreBadge score={scan.compositeScore} />
                    <div>
                      <p className="text-sm text-dark-100">
                        {new Date(scan.createdAt).toLocaleString()}
                      </p>
                      <p className="text-xs text-dark-500">
                        {scan.scenarioCount} scenarios &middot; {(scan.durationMs / 1000).toFixed(1)}s
                        {scan.partialResults && (
                          <span className="text-orange-400 ml-1">&middot; partial</span>
                        )}
                      </p>
                    </div>
                  </div>
                  {delta !== null && (
                    <span className={`text-sm font-medium ${
                      delta > 0 ? 'text-accent-emerald' :
                      delta < 0 ? 'text-red-400' :
                      'text-dark-500'
                    }`}>
                      {delta > 0 ? `+${delta}` : delta === 0 ? '—' : delta}
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
