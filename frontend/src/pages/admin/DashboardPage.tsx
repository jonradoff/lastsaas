import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Users, Building2, Activity, AlertTriangle, DollarSign, TrendingUp, UserCheck, Plug } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { adminApi } from '../../api/client';
import type { DailyMetricPoint, IntegrationCheck } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip } from 'recharts';
import { Card } from '../../components/ui';

function MetricChart({ data, color, formatter }: { data: DailyMetricPoint[]; color: string; formatter: (v: number) => string }) {
  if (data.length === 0) {
    return <div className="h-40 flex items-center justify-center text-dark-500 text-sm">No data yet</div>;
  }

  return (
    <ResponsiveContainer width="100%" height={160}>
      <AreaChart data={data} margin={{ top: 5, right: 5, bottom: 5, left: 5 }}>
        <defs>
          <linearGradient id={`grad-${color.replace('#', '')}`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor={color} stopOpacity={0.3} />
            <stop offset="95%" stopColor={color} stopOpacity={0} />
          </linearGradient>
        </defs>
        <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#64748b' }} tickLine={false} axisLine={false} />
        <YAxis hide />
        <Tooltip
          contentStyle={{ backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px' }}
          labelStyle={{ color: '#94a3b8' }}
          formatter={(value: number | undefined) => [formatter(value ?? 0), '']}
        />
        <Area type="monotone" dataKey="value" stroke={color} fill={`url(#grad-${color.replace('#', '')})`} strokeWidth={2} />
      </AreaChart>
    </ResponsiveContainer>
  );
}

export default function AdminDashboardPage() {
  const [chartRange, setChartRange] = useState<'7d' | '30d' | '1y'>('30d');

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'dashboard'],
    queryFn: async () => {
      const [dashData, intData] = await Promise.all([
        adminApi.getDashboard(),
        adminApi.getHealthIntegrations(),
      ]);
      const unconfigured = (intData.integrations || [])
        .filter((i: IntegrationCheck) => i.status === 'not_configured')
        .map((i: IntegrationCheck) => i.name);
      return { ...dashData, unconfiguredIntegrations: unconfigured };
    },
  });

  const { data: chartsData } = useQuery({
    queryKey: ['admin', 'dashboard-charts', chartRange],
    queryFn: async () => {
      const [rev, arr, dau] = await Promise.all([
        adminApi.getFinancialMetrics({ range: chartRange, metric: 'revenue' }),
        adminApi.getFinancialMetrics({ range: chartRange, metric: 'arr' }),
        adminApi.getFinancialMetrics({ range: chartRange, metric: 'dau' }),
      ]);
      return { revenue: rev.data, arr: arr.data, dau: dau.data };
    },
  });

  if (isLoading) return <LoadingSpinner size="lg" className="py-20" />;

  const healthy = data?.health?.healthy ?? true;
  const issues = data?.health?.issues ?? [];
  const unconfiguredIntegrations = data?.unconfiguredIntegrations ?? [];
  const revenueData = chartsData?.revenue ?? [];
  const arrData = chartsData?.arr ?? [];
  const dauData = chartsData?.dau ?? [];

  const latestRevenue = revenueData.length > 0 ? revenueData[revenueData.length - 1].value : 0;
  const latestArr = arrData.length > 0 ? arrData[arrData.length - 1].value : 0;
  const latestDau = dauData.length > 0 ? dauData[dauData.length - 1].value : 0;

  const formatCents = (v: number) => `$${(v / 100).toFixed(2)}`;
  const formatNum = (v: number) => v.toLocaleString();

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-dark-50">Admin Dashboard</h1>
        <p className="text-dark-400 mt-1">System overview and management</p>
      </div>

      {/* Unconfigured Integrations Warning */}
      {unconfiguredIntegrations.length > 0 && (
        <Link
          to="/last/health#integrations"
          className="flex items-center gap-3 mb-6 p-4 bg-yellow-500/5 border border-yellow-500/20 rounded-2xl hover:border-yellow-500/30 transition-colors"
        >
          <div className="w-10 h-10 rounded-xl bg-yellow-500/10 flex items-center justify-center flex-shrink-0">
            <Plug className="w-5 h-5 text-yellow-400" />
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-yellow-400">
              {unconfiguredIntegrations.length} integration{unconfiguredIntegrations.length > 1 ? 's' : ''} not configured
            </p>
            <p className="text-xs text-dark-400 mt-0.5">
              {unconfiguredIntegrations.map((n: string) => ({ stripe: 'Stripe', resend: 'Resend', mongodb: 'MongoDB', google_oauth: 'Google Login', github_oauth: 'GitHub Login', microsoft_oauth: 'Microsoft Login', webauthn: 'Passkeys', saml_sso: 'SSO/SAML' }[n] || n.charAt(0).toUpperCase() + n.slice(1))).join(', ')} {unconfiguredIntegrations.length > 1 ? 'need' : 'needs'} setup. Click to view details.
            </p>
          </div>
        </Link>
      )}

      {/* Top Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
        <Link to="/last/users">
          <Card className="hover:border-dark-700 transition-colors">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 rounded-xl bg-primary-500/20 flex items-center justify-center">
                <Users className="w-6 h-6 text-primary-400" />
              </div>
              <div>
                <p className="text-sm text-dark-400">Total Users</p>
                <p className="text-2xl font-bold text-dark-50">{data?.users ?? 0}</p>
              </div>
            </div>
          </Card>
        </Link>

        <Link to="/last/tenants">
          <Card className="hover:border-dark-700 transition-colors">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 rounded-xl bg-accent-purple/20 flex items-center justify-center">
                <Building2 className="w-6 h-6 text-accent-purple" />
              </div>
              <div>
                <p className="text-sm text-dark-400">Tenants</p>
                <p className="text-2xl font-bold text-dark-50">{data?.tenants ?? 0}</p>
              </div>
            </div>
          </Card>
        </Link>

        <Link to="/last/health">
          <Card className={`hover:border-dark-700 transition-colors ${!healthy ? 'border-red-500/30' : ''}`}>
            <div className="flex items-center gap-4">
              <div className={`w-12 h-12 rounded-xl flex items-center justify-center ${
                healthy ? 'bg-accent-emerald/20' : 'bg-red-500/20'
              }`}>
                {healthy ? (
                  <Activity className="w-6 h-6 text-accent-emerald" />
                ) : (
                  <AlertTriangle className="w-6 h-6 text-red-400" />
                )}
              </div>
              <div>
                <p className="text-sm text-dark-400">System Status</p>
                {healthy ? (
                  <p className="text-lg font-semibold text-accent-emerald">Healthy</p>
                ) : (
                  <p className="text-lg font-semibold text-red-400">Unhealthy</p>
                )}
              </div>
            </div>
            {!healthy && issues.length > 0 && (
              <div className="mt-3 space-y-1">
                {issues.map((issue, i) => (
                  <p key={i} className="text-xs text-red-400/80">{issue}</p>
                ))}
              </div>
            )}
          </Card>
        </Link>
      </div>

      {/* Business Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <Link to="/last/financial">
          <Card className="hover:border-dark-700 transition-colors">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 rounded-xl bg-accent-emerald/20 flex items-center justify-center">
                <DollarSign className="w-6 h-6 text-accent-emerald" />
              </div>
              <div>
                <p className="text-sm text-dark-400">Revenue Today</p>
                <p className="text-2xl font-bold text-dark-50">{formatCents(latestRevenue)}</p>
              </div>
            </div>
          </Card>
        </Link>

        <Card>
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-primary-500/20 flex items-center justify-center">
              <TrendingUp className="w-6 h-6 text-primary-400" />
            </div>
            <div>
              <p className="text-sm text-dark-400">ARR</p>
              <p className="text-2xl font-bold text-dark-50">{formatCents(latestArr)}</p>
            </div>
          </div>
        </Card>

        <Card>
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-yellow-500/20 flex items-center justify-center">
              <UserCheck className="w-6 h-6 text-yellow-400" />
            </div>
            <div>
              <p className="text-sm text-dark-400">DAU</p>
              <p className="text-2xl font-bold text-dark-50">{formatNum(latestDau)}</p>
            </div>
          </div>
        </Card>
      </div>

      {/* Charts */}
      <div className="mb-6 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-dark-50">Business Metrics</h2>
        <div className="flex gap-1 bg-dark-900/50 border border-dark-800 rounded-lg p-1">
          {(['7d', '30d', '1y'] as const).map(range => (
            <button
              key={range}
              onClick={() => setChartRange(range)}
              className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                chartRange === range ? 'bg-dark-700 text-dark-50' : 'text-dark-400 hover:text-dark-300'
              }`}
            >
              {range}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">Revenue</h3>
          <MetricChart data={revenueData} color="#10b981" formatter={formatCents} />
        </Card>
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">ARR</h3>
          <MetricChart data={arrData} color="#6366f1" formatter={formatCents} />
        </Card>
        <Card className="p-4">
          <h3 className="text-sm font-medium text-dark-400 mb-2">DAU</h3>
          <MetricChart data={dauData} color="#eab308" formatter={formatNum} />
        </Card>
      </div>
    </div>
  );
}
