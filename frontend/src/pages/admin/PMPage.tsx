import { useState, useRef, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { pmApi } from '../../api/client';
import type { FunnelData, CohortRow, EngagementData, KPIData, EventTypeSummary } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { Card } from '../../components/ui';
import {
  ResponsiveContainer, AreaChart, Area, BarChart, Bar, LineChart, Line,
  XAxis, YAxis, Tooltip, CartesianGrid,
} from 'recharts';
import {
  TrendingUp, Users, DollarSign, Percent, Clock, UserCheck, BarChart3, Zap,
} from 'lucide-react';

type Tab = 'funnel' | 'kpis' | 'retention' | 'engagement' | 'events';
type Range = '7d' | '30d' | '90d' | '1y';

const tooltipStyle = { backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px' };
const tooltipLabelStyle = { color: '#94a3b8' };

function RangeSelector({ value, onChange }: { value: Range; onChange: (r: Range) => void }) {
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const debouncedChange = useCallback((r: Range) => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => onChange(r), 300);
  }, [onChange]);

  return (
    <div className="flex gap-1 bg-dark-900/50 border border-dark-800 rounded-lg p-1">
      {(['7d', '30d', '90d', '1y'] as const).map(r => (
        <button
          key={r}
          onClick={() => debouncedChange(r)}
          className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
            value === r ? 'bg-dark-700 text-white' : 'text-dark-400 hover:text-dark-300'
          }`}
        >
          {r}
        </button>
      ))}
    </div>
  );
}

// Bin daily data points into weekly or monthly buckets for large date ranges.
function binChartData(points: { date: string; value: number }[], range: Range): { date: string; value: number }[] {
  if (!points || points.length === 0) return points;
  if (range === '7d' || range === '30d') return points;

  const buckets = new Map<string, number>();
  for (const p of points) {
    let key: string;
    if (range === '1y') {
      // Monthly: YYYY-MM
      key = p.date.slice(0, 7);
    } else {
      // Weekly (90d): ISO week start (Monday)
      const d = new Date(p.date + 'T00:00:00Z');
      const day = d.getUTCDay();
      const diff = d.getUTCDate() - day + (day === 0 ? -6 : 1);
      const monday = new Date(d);
      monday.setUTCDate(diff);
      key = monday.toISOString().slice(0, 10);
    }
    buckets.set(key, (buckets.get(key) || 0) + p.value);
  }

  return Array.from(buckets.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, value]) => ({ date, value }));
}

function formatCents(v: number) {
  return `$${(v / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function formatNum(v: number) {
  return v.toLocaleString();
}

function formatPct(v: number) {
  return `${v.toFixed(1)}%`;
}

// --- Funnel Tab ---

function FunnelTab() {
  const [range, setRange] = useState<Range>('30d');
  const { data, isLoading } = useQuery({
    queryKey: ['pm', 'funnel', range],
    queryFn: () => pmApi.getFunnel({ range }),
  });

  if (isLoading) return <LoadingSpinner size="lg" className="py-20" />;
  const funnel = data as FunnelData;
  if (!funnel) return null;

  const maxCount = Math.max(...funnel.steps.map(s => s.count), 1);

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-lg font-semibold text-white">Conversion Funnel</h2>
        <RangeSelector value={range} onChange={setRange} />
      </div>

      <Card className="p-6">
        <div className="space-y-4">
          {funnel.steps.map((step, i) => (
            <div key={step.name}>
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-dark-300">{step.name}</span>
                <div className="flex items-center gap-3">
                  <span className="text-sm font-medium text-white">{formatNum(step.count)}</span>
                  {i > 0 && (
                    <span className={`text-xs px-2 py-0.5 rounded-full ${
                      step.conversion >= 50 ? 'bg-accent-emerald/20 text-accent-emerald' :
                      step.conversion >= 20 ? 'bg-yellow-500/20 text-yellow-400' :
                      'bg-red-500/20 text-red-400'
                    }`}>
                      {formatPct(step.conversion)}
                    </span>
                  )}
                </div>
              </div>
              <div className="h-8 bg-dark-800 rounded-lg overflow-hidden">
                <div
                  className="h-full rounded-lg transition-all duration-500"
                  style={{
                    width: `${Math.max((step.count / maxCount) * 100, 1)}%`,
                    background: `linear-gradient(90deg, ${
                      i === 0 ? '#6366f1' : i <= 2 ? '#8b5cf6' : i <= 4 ? '#10b981' : '#eab308'
                    }, ${
                      i === 0 ? '#818cf8' : i <= 2 ? '#a78bfa' : i <= 4 ? '#34d399' : '#facc15'
                    })`,
                  }}
                />
              </div>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

// --- KPIs Tab ---

function KPICard({ icon: Icon, label, value, color }: {
  icon: typeof DollarSign; label: string; value: string; color: string;
}) {
  return (
    <Card>
      <div className="flex items-center gap-4">
        <div className={`w-12 h-12 rounded-xl flex items-center justify-center`} style={{ backgroundColor: `${color}20` }}>
          <Icon className="w-6 h-6" style={{ color }} />
        </div>
        <div>
          <p className="text-sm text-dark-400">{label}</p>
          <p className="text-2xl font-bold text-white">{value}</p>
        </div>
      </div>
    </Card>
  );
}

function KPIsTab() {
  const { data, isLoading } = useQuery({
    queryKey: ['pm', 'kpis'],
    queryFn: () => pmApi.getKPIs(),
  });

  if (isLoading) return <LoadingSpinner size="lg" className="py-20" />;
  const kpi = data as KPIData;
  if (!kpi) return null;

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold text-white">Key Performance Indicators</h2>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <KPICard icon={DollarSign} label="MRR" value={formatCents(kpi.mrr)} color="#10b981" />
        <KPICard icon={TrendingUp} label="ARR" value={formatCents(kpi.arr)} color="#6366f1" />
        <KPICard icon={Users} label="Active Subscribers" value={formatNum(kpi.activeSubscribers)} color="#8b5cf6" />
        <KPICard icon={UserCheck} label="Total Registrations" value={formatNum(kpi.totalRegistrations)} color="#3b82f6" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <KPICard icon={DollarSign} label="ARPU" value={formatCents(kpi.arpu)} color="#14b8a6" />
        <KPICard icon={Zap} label="LTV" value={formatCents(kpi.ltv)} color="#f59e0b" />
        <KPICard icon={Percent} label="Churn Rate" value={formatPct(kpi.churnRate)} color="#ef4444" />
        <KPICard icon={Percent} label="Trial Conversion" value={formatPct(kpi.trialConversionRate)} color="#22c55e" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <KPICard icon={Clock} label="Median Days to First Purchase" value={kpi.timeToFirstPurchase.toFixed(1)} color="#a855f7" />
      </div>

      {/* MRR Trend */}
      {kpi.mrrTrend.length > 0 && (
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">MRR Trend (30d)</h3>
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={kpi.mrrTrend} margin={{ top: 5, right: 5, bottom: 5, left: 5 }}>
              <defs>
                <linearGradient id="mrrGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <YAxis hide />
              <Tooltip contentStyle={tooltipStyle} labelStyle={tooltipLabelStyle} formatter={(v) => [formatCents(v as number), 'MRR']} />
              <Area type="monotone" dataKey="value" stroke="#10b981" fill="url(#mrrGrad)" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        </Card>
      )}

      {/* Plan Distribution */}
      {kpi.planDistribution.length > 0 && (
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-4">Plan Distribution</h3>
          <div className="space-y-3">
            {kpi.planDistribution.map(p => (
              <div key={p.planName}>
                <div className="flex justify-between text-sm mb-1">
                  <span className="text-dark-300">{p.planName}</span>
                  <span className="text-white">{p.subscribers} ({formatPct(p.percentage)}) &middot; {formatCents(p.mrr)}/mo</span>
                </div>
                <div className="h-2 bg-dark-800 rounded-full overflow-hidden">
                  <div className="h-full bg-primary-500 rounded-full" style={{ width: `${p.percentage}%` }} />
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}

// --- Retention Tab ---

function RetentionTab() {
  const [granularity, setGranularity] = useState<'weekly' | 'monthly'>('weekly');
  const { data, isLoading } = useQuery({
    queryKey: ['pm', 'retention', granularity],
    queryFn: () => pmApi.getRetention({ granularity, periods: 12 }),
  });

  if (isLoading) return <LoadingSpinner size="lg" className="py-20" />;
  const cohorts = (data?.cohorts ?? []) as CohortRow[];

  const cellColor = (pct: number) => {
    if (pct >= 80) return 'bg-accent-emerald/30 text-accent-emerald';
    if (pct >= 60) return 'bg-accent-emerald/20 text-accent-emerald';
    if (pct >= 40) return 'bg-yellow-500/20 text-yellow-400';
    if (pct >= 20) return 'bg-orange-500/20 text-orange-400';
    return 'bg-red-500/20 text-red-400';
  };

  const maxPeriods = Math.max(...cohorts.map(c => c.retention.length), 0);

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-lg font-semibold text-white">Cohort Retention</h2>
        <div className="flex gap-1 bg-dark-900/50 border border-dark-800 rounded-lg p-1">
          {(['weekly', 'monthly'] as const).map(g => (
            <button
              key={g}
              onClick={() => setGranularity(g)}
              className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                granularity === g ? 'bg-dark-700 text-white' : 'text-dark-400 hover:text-dark-300'
              }`}
            >
              {g}
            </button>
          ))}
        </div>
      </div>

      {cohorts.length === 0 ? (
        <Card className="p-8 text-center text-dark-500">No retention data yet</Card>
      ) : (
        <Card className="p-4 overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr>
                <th className="text-left text-dark-400 pb-2 pr-4">Cohort</th>
                <th className="text-center text-dark-400 pb-2 px-1">Size</th>
                {Array.from({ length: maxPeriods }, (_, i) => (
                  <th key={i} className="text-center text-dark-400 pb-2 px-1">P{i}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {cohorts.map(c => (
                <tr key={c.cohortLabel}>
                  <td className="text-dark-300 py-1 pr-4 whitespace-nowrap">{c.cohortLabel}</td>
                  <td className="text-center text-dark-400 py-1 px-1">{c.cohortSize}</td>
                  {c.retention.map((pct, i) => (
                    <td key={i} className={`text-center py-1 px-1 rounded ${cellColor(pct)}`}>
                      {formatPct(pct)}
                    </td>
                  ))}
                  {Array.from({ length: maxPeriods - c.retention.length }, (_, i) => (
                    <td key={`empty-${i}`} className="py-1 px-1" />
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

// --- Engagement Tab ---

function EngagementTab() {
  const [range, setRange] = useState<Range>('30d');
  const { data, isLoading } = useQuery({
    queryKey: ['pm', 'engagement', range],
    queryFn: () => pmApi.getEngagement({ range }),
  });

  if (isLoading) return <LoadingSpinner size="lg" className="py-20" />;
  const eng = data as EngagementData;
  if (!eng) return null;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold text-white">Engagement (Paying Subscribers)</h2>
        <RangeSelector value={range} onChange={setRange} />
      </div>

      <Card>
        <div className="flex items-center gap-4 p-4">
          <div className="w-12 h-12 rounded-xl bg-primary-500/20 flex items-center justify-center">
            <BarChart3 className="w-6 h-6 text-primary-400" />
          </div>
          <div>
            <p className="text-sm text-dark-400">Avg Sessions / User / Week</p>
            <p className="text-2xl font-bold text-white">{eng.avgSessions.toFixed(2)}</p>
          </div>
        </div>
      </Card>

      {/* DAU/WAU/MAU Chart */}
      {(eng.dau?.length > 0 || eng.wau?.length > 0 || eng.mau?.length > 0) && (
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">Active Users</h3>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart margin={{ top: 5, right: 5, bottom: 5, left: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
              <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <YAxis tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <Tooltip contentStyle={tooltipStyle} labelStyle={tooltipLabelStyle} />
              {eng.dau?.length > 0 && <Line data={binChartData(eng.dau, range)} type="monotone" dataKey="value" name="DAU" stroke="#6366f1" strokeWidth={2} dot={false} />}
              {eng.wau?.length > 0 && <Line data={binChartData(eng.wau, range)} type="monotone" dataKey="value" name="WAU" stroke="#10b981" strokeWidth={2} dot={false} />}
              {eng.mau?.length > 0 && <Line data={binChartData(eng.mau, range)} type="monotone" dataKey="value" name="MAU" stroke="#eab308" strokeWidth={2} dot={false} />}
            </LineChart>
          </ResponsiveContainer>
        </Card>
      )}

      {/* Top Features */}
      {eng.topFeatures?.length > 0 && (
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">Top Custom Events</h3>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={eng.topFeatures} layout="vertical" margin={{ top: 5, right: 5, bottom: 5, left: 100 }}>
              <XAxis type="number" tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <YAxis type="category" dataKey="name" tick={{ fontSize: 11, fill: '#94a3b8' }} tickLine={false} axisLine={false} width={95} />
              <Tooltip contentStyle={tooltipStyle} labelStyle={tooltipLabelStyle} />
              <Bar dataKey="count" fill="#6366f1" radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>
      )}

      {/* Credit Consumption */}
      {eng.creditTrend?.length > 0 && (
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">Credit Consumption</h3>
          <ResponsiveContainer width="100%" height={160}>
            <AreaChart data={binChartData(eng.creditTrend, range)} margin={{ top: 5, right: 5, bottom: 5, left: 5 }}>
              <defs>
                <linearGradient id="creditGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
              <YAxis hide />
              <Tooltip contentStyle={tooltipStyle} labelStyle={tooltipLabelStyle} formatter={(v) => [formatNum(v as number), 'Credits']} />
              <Area type="monotone" dataKey="value" stroke="#8b5cf6" fill="url(#creditGrad)" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        </Card>
      )}
    </div>
  );
}

// --- Events Tab ---

function EventsTab() {
  const [range, setRange] = useState<Range>('30d');
  const [selectedEvent, setSelectedEvent] = useState('');

  const { data: typesData } = useQuery({
    queryKey: ['pm', 'event-types'],
    queryFn: () => pmApi.listEventTypes(),
  });

  const eventTypes = (typesData?.eventTypes ?? []) as EventTypeSummary[];

  const { data: eventData, isLoading } = useQuery({
    queryKey: ['pm', 'events', selectedEvent, range],
    queryFn: () => pmApi.getCustomEvents({ name: selectedEvent || undefined, range }),
  });

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold text-white">Event Explorer</h2>
        <RangeSelector value={range} onChange={setRange} />
      </div>

      <div className="flex gap-4 items-center">
        <select
          value={selectedEvent}
          onChange={e => setSelectedEvent(e.target.value)}
          className="bg-dark-800 border border-dark-700 text-white rounded-lg px-3 py-2 text-sm focus:ring-primary-500 focus:border-primary-500"
        >
          <option value="">All Events</option>
          {eventTypes.map(et => (
            <option key={et.eventName} value={et.eventName}>{et.eventName} ({et.count})</option>
          ))}
        </select>
      </div>

      {isLoading ? (
        <LoadingSpinner size="lg" className="py-10" />
      ) : eventData ? (
        <>
          <Card>
            <div className="flex items-center gap-4 p-4">
              <div className="w-12 h-12 rounded-xl bg-accent-emerald/20 flex items-center justify-center">
                <Zap className="w-6 h-6 text-accent-emerald" />
              </div>
              <div>
                <p className="text-sm text-dark-400">Total Events</p>
                <p className="text-2xl font-bold text-white">{formatNum(eventData.totalCount)}</p>
              </div>
            </div>
          </Card>

          {eventData.trend?.length > 0 && (
            <Card className="p-4">
              <h3 className="text-sm font-medium text-dark-400 mb-2">
                {eventData.eventName ? `Trend: ${eventData.eventName}` : 'Event Trend'}
              </h3>
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={binChartData(eventData.trend, range)} margin={{ top: 5, right: 5, bottom: 5, left: 5 }}>
                  <defs>
                    <linearGradient id="eventGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
                  <YAxis hide />
                  <Tooltip contentStyle={tooltipStyle} labelStyle={tooltipLabelStyle} formatter={(v) => [formatNum(v as number), 'Events']} />
                  <Area type="monotone" dataKey="value" stroke="#10b981" fill="url(#eventGrad)" strokeWidth={2} />
                </AreaChart>
              </ResponsiveContainer>
            </Card>
          )}
        </>
      ) : null}

      {/* Event Types Table */}
      {eventTypes.length > 0 && (
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-4">All Event Types</h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-dark-400 text-left">
                <th className="pb-2">Event Name</th>
                <th className="pb-2">Category</th>
                <th className="pb-2 text-right">Count</th>
                <th className="pb-2 text-right">Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {eventTypes.map(et => (
                <tr key={et.eventName} className="border-t border-dark-800">
                  <td className="py-2 text-white">{et.eventName}</td>
                  <td className="py-2">
                    <span className={`text-xs px-2 py-0.5 rounded-full ${
                      et.category === 'funnel' ? 'bg-primary-500/20 text-primary-400' :
                      et.category === 'engagement' ? 'bg-accent-emerald/20 text-accent-emerald' :
                      'bg-yellow-500/20 text-yellow-400'
                    }`}>{et.category}</span>
                  </td>
                  <td className="py-2 text-right text-dark-300">{formatNum(et.count)}</td>
                  <td className="py-2 text-right text-dark-400">{new Date(et.lastSeen).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

// --- Main Page ---

const tabs: { key: Tab; label: string }[] = [
  { key: 'funnel', label: 'Funnel' },
  { key: 'kpis', label: 'KPIs' },
  { key: 'retention', label: 'Retention' },
  { key: 'engagement', label: 'Engagement' },
  { key: 'events', label: 'Events' },
];

export default function PMPage() {
  const [activeTab, setActiveTab] = useState<Tab>('funnel');

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Product Analytics</h1>
        <p className="text-dark-400 mt-1">Customer journey, KPIs, retention, and engagement</p>
      </div>

      {/* Tab Bar */}
      <div className="flex gap-1 mb-8 bg-dark-900/50 border border-dark-800 rounded-lg p-1 w-fit">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-4 py-2 text-sm font-medium rounded-md transition-colors ${
              activeTab === tab.key
                ? 'bg-dark-700 text-white'
                : 'text-dark-400 hover:text-dark-300'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === 'funnel' && <FunnelTab />}
      {activeTab === 'kpis' && <KPIsTab />}
      {activeTab === 'retention' && <RetentionTab />}
      {activeTab === 'engagement' && <EngagementTab />}
      {activeTab === 'events' && <EventsTab />}
    </div>
  );
}
