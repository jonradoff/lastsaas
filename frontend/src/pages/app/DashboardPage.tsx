import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, TrendingUp, TrendingDown, Minus, Zap, X, Trash2, Store } from 'lucide-react';
import { toast } from 'sonner';
import DOMPurify from 'dompurify';
import { trackedStoresApi } from '../../api/client';
import type { TrackedStore } from '../../api/client';
import { useBranding } from '../../contexts/BrandingContext';
import { getErrorMessage } from '../../utils/errors';
import LoadingSpinner from '../../components/LoadingSpinner';

function ScoreRing({ score }: { score: number }) {
  const r = 22;
  const circumference = 2 * Math.PI * r;
  const progress = Math.min(100, Math.max(0, score));
  const offset = circumference - (progress / 100) * circumference;
  const color =
    score >= 80 ? '#10b981' :
    score >= 60 ? '#facc15' :
    score >= 40 ? '#fb923c' :
    '#f87171';

  return (
    <svg className="w-14 h-14 -rotate-90" viewBox="0 0 52 52">
      <circle cx="26" cy="26" r={r} fill="none" stroke="rgba(0,0,0,0.08)" strokeWidth="4" />
      <circle
        cx="26" cy="26" r={r}
        fill="none"
        stroke={color}
        strokeWidth="4"
        strokeDasharray={circumference}
        strokeDashoffset={offset}
        strokeLinecap="round"
        style={{ transition: 'stroke-dashoffset 0.4s ease' }}
      />
      <text
        x="26" y="26"
        textAnchor="middle"
        dominantBaseline="central"
        fill="currentColor"
        fontSize="12"
        fontWeight="bold"
        style={{ transform: 'rotate(90deg)', transformOrigin: '26px 26px' }}
      >
        {score}
      </text>
    </svg>
  );
}

function TrendBadge({ trend }: { trend: TrackedStore['trend'] }) {
  if (trend === 'up') return <TrendingUp className="w-4 h-4 text-accent-emerald" />;
  if (trend === 'down') return <TrendingDown className="w-4 h-4 text-red-400" />;
  return <Minus className="w-4 h-4 text-dark-500" />;
}

function AddStoreModal({ onAdd, onClose }: { onAdd: (domain: string) => Promise<void>; onClose: () => void }) {
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!domain.trim()) return;
    setLoading(true);
    try {
      await onAdd(domain.trim());
      onClose();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-dark-100">Track a Store</h3>
          <button
            onClick={onClose}
            className="p-1 rounded-lg text-dark-400 hover:text-dark-100 hover:bg-dark-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>
        <p className="text-sm text-dark-400 mb-4">
          Enter the domain of the MCP store you want to monitor.
        </p>
        <form onSubmit={handleSubmit}>
          <input
            type="text"
            value={domain}
            onChange={e => setDomain(e.target.value)}
            placeholder="e.g. allbirds.com"
            autoFocus
            className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-xl text-dark-100 placeholder-dark-400 focus:outline-none focus:border-primary-500 transition-colors mb-4"
          />
          <div className="flex gap-3 justify-end">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-dark-300 hover:text-dark-100 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !domain.trim()}
              className="px-4 py-2 text-sm bg-primary-500 hover:bg-primary-600 disabled:opacity-60 text-white font-medium rounded-lg transition-colors"
            >
              {loading ? 'Adding…' : 'Add Store'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function UpgradePrompt({ message, onUpgrade }: { message: string; onUpgrade: () => void }) {
  return (
    <div className="rounded-2xl border border-primary-500/30 bg-primary-500/5 p-6 text-center">
      <div className="w-12 h-12 rounded-xl bg-primary-500/20 flex items-center justify-center mx-auto mb-3">
        <Zap className="w-6 h-6 text-primary-400" />
      </div>
      <h3 className="text-base font-semibold text-dark-100 mb-1">Upgrade to Track Stores</h3>
      <p className="text-sm text-dark-400 mb-4">{message}</p>
      <button
        onClick={onUpgrade}
        className="px-5 py-2 bg-primary-500 hover:bg-primary-600 text-white text-sm font-medium rounded-lg transition-colors"
      >
        View Plans
      </button>
    </div>
  );
}

export default function DashboardPage() {
  const navigate = useNavigate();
  const { branding } = useBranding();

  const [stores, setStores] = useState<TrackedStore[]>([]);
  const [maxStores, setMaxStores] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const loadStores = () => {
    trackedStoresApi.list()
      .then(data => {
        setStores(data.stores);
        setMaxStores(data.maxStores);
      })
      .catch(err => {
        // If 402 (payment required), treat as no entitlement
        const status = (err as { response?: { status?: number } })?.response?.status;
        if (status === 400 || status === 402) {
          setMaxStores(0);
          setStores([]);
        } else {
          toast.error(getErrorMessage(err));
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadStores(); }, []);

  const handleAddStore = async (domain: string) => {
    try {
      const store = await trackedStoresApi.add(domain);
      setStores(prev => [store, ...prev]);
      toast.success(`Now tracking ${domain}`);
    } catch (err) {
      toast.error(getErrorMessage(err));
      throw err;
    }
  };

  const handleRemoveStore = async (store: TrackedStore) => {
    if (!window.confirm(`Stop tracking ${store.domain}?`)) return;
    setDeletingId(store.id);
    try {
      await trackedStoresApi.remove(store.id);
      setStores(prev => prev.filter(s => s.id !== store.id));
      toast.success(`Stopped tracking ${store.domain}`);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setDeletingId(null);
    }
  };

  const isAtMax = maxStores !== null && maxStores > 0 && stores.length >= maxStores;
  const hasEntitlement = maxStores === null || maxStores > 0;

  // Fetch plans for upgrade redirect
  const handleUpgradeClick = () => navigate('/plan');

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  return (
    <div>
      {branding.dashboardHtml && (
        <div className="mb-8" dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(branding.dashboardHtml) }} />
      )}

      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-dark-100 flex items-center gap-2">
            <Store className="w-6 h-6 text-primary-400" />
            Tracked Stores
          </h1>
          <p className="text-dark-400 text-sm mt-1">
            Monitor your MCP stores and track score changes over time.
          </p>
        </div>
        {hasEntitlement && (
          <div className="flex items-center gap-3">
            {maxStores !== null && maxStores > 0 && (
              <span className="text-sm text-dark-400">
                {stores.length} / {maxStores} stores
              </span>
            )}
            <button
              onClick={() => {
                if (isAtMax) {
                  toast.error('You have reached the maximum number of tracked stores. Upgrade to add more.');
                  return;
                }
                setShowAddModal(true);
              }}
              disabled={isAtMax}
              className="flex items-center gap-2 px-4 py-2 bg-primary-500 hover:bg-primary-600 disabled:opacity-60 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            >
              <Plus className="w-4 h-4" />
              Add Store
            </button>
          </div>
        )}
      </div>

      {/* Content */}
      {!hasEntitlement ? (
        <UpgradePrompt
          message="Your current plan doesn't include store tracking. Upgrade to start monitoring your MCP stores."
          onUpgrade={handleUpgradeClick}
        />
      ) : stores.length === 0 ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-12 text-center">
          <div className="w-16 h-16 rounded-2xl bg-primary-500/10 flex items-center justify-center mx-auto mb-4">
            <Store className="w-8 h-8 text-primary-400" />
          </div>
          <h3 className="text-lg font-semibold text-dark-100 mb-2">No stores tracked yet</h3>
          <p className="text-sm text-dark-400 mb-5">
            Add your first MCP store to start monitoring its score and trends.
          </p>
          <button
            onClick={() => setShowAddModal(true)}
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-primary-500 hover:bg-primary-600 text-white text-sm font-medium rounded-lg transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add Your First Store
          </button>
        </div>
      ) : (
        <>
          {isAtMax && (
            <div className="mb-4 flex items-center gap-3 px-4 py-3 rounded-xl border border-yellow-500/30 bg-yellow-500/5">
              <Zap className="w-4 h-4 text-yellow-400 shrink-0" />
              <p className="text-sm text-yellow-300 flex-1">
                You've reached your limit of {maxStores} tracked stores.{' '}
                <button onClick={handleUpgradeClick} className="underline hover:no-underline">
                  Upgrade your plan
                </button>{' '}
                to track more.
              </p>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {stores.map(store => (
              <div
                key={store.id}
                className="group bg-dark-900/50 border border-dark-800 hover:border-dark-700 rounded-2xl p-5 cursor-pointer transition-all"
                onClick={() => navigate(`/dashboard/store/${store.id}`)}
              >
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1 min-w-0">
                    <h3 className="text-sm font-semibold text-dark-100 truncate">{store.domain}</h3>
                    <p className="text-xs text-dark-500 mt-0.5">
                      {store.lastScannedAt
                        ? `Last scanned ${new Date(store.lastScannedAt).toLocaleDateString()}`
                        : 'Not yet scanned'}
                    </p>
                  </div>
                  <button
                    onClick={e => { e.stopPropagation(); handleRemoveStore(store); }}
                    disabled={deletingId === store.id}
                    className="p-1.5 rounded-lg text-dark-600 hover:text-red-400 hover:bg-red-400/10 opacity-0 group-hover:opacity-100 transition-all ml-2 shrink-0"
                    title="Stop tracking"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>

                <div className="flex items-end justify-between">
                  <div>
                    <ScoreRing score={store.currentScore} />
                  </div>
                  <div className="text-right">
                    <div className="flex items-center gap-1 justify-end">
                      <TrendBadge trend={store.trend} />
                      <span className={`text-xs font-medium ${
                        store.trend === 'up' ? 'text-accent-emerald' :
                        store.trend === 'down' ? 'text-red-400' :
                        'text-dark-500'
                      }`}>
                        {store.trend === 'up' ? '+' : store.trend === 'down' ? '' : ''}
                        {store.previousScore && store.currentScore !== store.previousScore
                          ? `${store.currentScore - store.previousScore}`
                          : store.trend}
                      </span>
                    </div>
                    <p className="text-xs text-dark-500 mt-1">vs previous</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {showAddModal && (
        <AddStoreModal
          onAdd={handleAddStore}
          onClose={() => setShowAddModal(false)}
        />
      )}
    </div>
  );
}
