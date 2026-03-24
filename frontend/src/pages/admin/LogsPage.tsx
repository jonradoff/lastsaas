import { useEffect, useState, useCallback, useRef } from 'react';
import { FileText, Search, ChevronLeft, ChevronRight, X, RefreshCw, Download, ChevronDown, ChevronUp, Calendar } from 'lucide-react';
import { useSearchParams, Link } from 'react-router-dom';
import { adminApi } from '../../api/client';
import type { SystemLog, LogSeverity, LogCategory } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';

const severityConfig: Record<LogSeverity, { label: string; color: string; bg: string }> = {
  critical: { label: 'Critical', color: 'text-red-400', bg: 'bg-red-500/10' },
  high:     { label: 'High',     color: 'text-orange-400', bg: 'bg-orange-500/10' },
  medium:   { label: 'Medium',   color: 'text-yellow-400', bg: 'bg-yellow-500/10' },
  low:      { label: 'Low',      color: 'text-blue-400', bg: 'bg-blue-500/10' },
  debug:    { label: 'Debug',    color: 'text-dark-400', bg: 'bg-dark-700/50' },
};

const categoryLabels: Record<LogCategory, string> = {
  auth: 'Auth',
  billing: 'Billing',
  admin: 'Admin',
  system: 'System',
  security: 'Security',
  tenant: 'Tenant',
};

const DATE_PRESETS = [
  { label: 'Last hour', hours: 1 },
  { label: 'Last 24h', hours: 24 },
  { label: 'Last 7d', hours: 168 },
  { label: 'Last 30d', hours: 720 },
];

const ALL_SEVERITIES: LogSeverity[] = ['critical', 'high', 'medium', 'low', 'debug'];

const PER_PAGE_OPTIONS = [25, 50, 100];

export default function LogsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [logs, setLogs] = useState<SystemLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(50);
  const [activeSeverities, setActiveSeverities] = useState<Set<LogSeverity>>(() => new Set(ALL_SEVERITIES));
  const [category, setCategory] = useState('');
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [userId, setUserId] = useState(searchParams.get('userId') || '');
  const [fromDate, setFromDate] = useState('');
  const [toDate, setToDate] = useState('');
  const [loading, setLoading] = useState(true);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [severityCounts, setSeverityCounts] = useState<Record<string, number>>({});
  const [autoRefresh, setAutoRefresh] = useState(false);
  const autoRefreshRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const fetchLogs = useCallback(async () => {
    if (abortRef.current) abortRef.current.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    try {
      const params: Record<string, string | number> = { page, perPage };
      if (activeSeverities.size > 0 && activeSeverities.size < 5) {
        params.severity = Array.from(activeSeverities).join(',');
      }
      if (category) params.category = category;
      if (search) params.search = search;
      if (userId) params.userId = userId;
      if (fromDate) params.fromDate = fromDate;
      if (toDate) params.toDate = toDate;
      const data = await adminApi.listLogs(params);
      if (!controller.signal.aborted) {
        setLogs(data.logs);
        setTotal(data.total);
      }
    } catch {
      // ignore
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [page, perPage, activeSeverities, category, search, userId, fromDate, toDate]);

  const fetchSeverityCounts = useCallback(async () => {
    try {
      const params: Record<string, string> = {};
      if (category) params.category = category;
      if (fromDate) params.fromDate = fromDate;
      if (toDate) params.toDate = toDate;
      const data = await adminApi.logSeverityCounts(params);
      setSeverityCounts(data.counts);
    } catch {
      // ignore
    }
  }, [category, fromDate, toDate]);

  useEffect(() => { fetchLogs(); }, [fetchLogs]);
  useEffect(() => { fetchSeverityCounts(); }, [fetchSeverityCounts]);

  // Auto-refresh (pauses when tab is in background)
  useEffect(() => {
    if (!autoRefresh) return;

    const tick = () => {
      if (document.visibilityState === 'visible') {
        fetchLogs();
        fetchSeverityCounts();
      }
    };
    autoRefreshRef.current = setInterval(tick, 10000);
    return () => {
      if (autoRefreshRef.current) clearInterval(autoRefreshRef.current);
    };
  }, [autoRefresh, fetchLogs, fetchSeverityCounts]);

  const totalPages = Math.max(1, Math.ceil(total / perPage));

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setSearch(searchInput);
  };

  const toggleSeverity = (sev: LogSeverity) => {
    setPage(1);
    setActiveSeverities(prev => {
      const next = new Set(prev);
      if (next.has(sev)) {
        next.delete(sev);
        // Deselecting the last one re-selects all
        if (next.size === 0) return new Set(ALL_SEVERITIES);
      } else {
        next.add(sev);
      }
      return next;
    });
  };

  const handleCategoryChange = (cat: string) => {
    setPage(1);
    setCategory(cat);
  };

  const clearUserFilter = () => {
    setUserId('');
    setPage(1);
    setSearchParams({});
  };

  const applyDatePreset = (hours: number) => {
    const now = new Date();
    const from = new Date(now.getTime() - hours * 60 * 60 * 1000);
    setFromDate(from.toISOString());
    setToDate('');
    setPage(1);
  };

  const clearDateFilter = () => {
    setFromDate('');
    setToDate('');
    setPage(1);
  };

  const handleExport = async () => {
    try {
      const params: Record<string, string> = {};
      if (activeSeverities.size > 0 && activeSeverities.size < 5) {
        params.severity = Array.from(activeSeverities).join(',');
      }
      if (category) params.category = category;
      if (search) params.search = search;
      if (fromDate) params.fromDate = fromDate;
      if (toDate) params.toDate = toDate;
      const blob = await adminApi.exportLogsCSV(params);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'system_logs.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // ignore
    }
  };

  const handleRefresh = () => {
    fetchLogs();
    fetchSeverityCounts();
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <FileText className="w-7 h-7 text-primary-400" />
            System Logs
          </h1>
          <p className="text-dark-400 mt-1">{total.toLocaleString()} entries</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setAutoRefresh(a => !a)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm border transition-colors ${
              autoRefresh
                ? 'border-primary-500/50 bg-primary-500/10 text-primary-400'
                : 'border-dark-700 bg-dark-800 text-dark-300 hover:text-white'
            }`}
          >
            <RefreshCw className={`w-3.5 h-3.5 ${autoRefresh ? 'animate-spin' : ''}`} />
            {autoRefresh ? 'Auto' : 'Auto'}
          </button>
          <button
            onClick={handleRefresh}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white transition-colors"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            Refresh
          </button>
          <button
            onClick={handleExport}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white transition-colors"
          >
            <Download className="w-3.5 h-3.5" />
            CSV
          </button>
        </div>
      </div>

      {/* Severity multi-select toggles */}
      <div className="flex items-center gap-2 mb-4 flex-wrap">
        {ALL_SEVERITIES.map(sev => {
          const count = severityCounts[sev] || 0;
          const cfg = severityConfig[sev];
          const isActive = activeSeverities.has(sev);
          return (
            <button
              key={sev}
              onClick={() => toggleSeverity(sev)}
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium transition-colors border ${
                isActive
                  ? `${cfg.bg} ${cfg.color} border-current`
                  : 'bg-dark-800/50 text-dark-600 border-dark-700/50 line-through'
              }`}
            >
              {cfg.label}{count > 0 ? `: ${count.toLocaleString()}` : ''}
            </button>
          );
        })}
      </div>

      {/* Active user filter chip */}
      {userId && (
        <div className="mb-4 flex items-center gap-2">
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-primary-500/10 border border-primary-500/20 rounded-lg text-sm text-primary-400">
            Filtered by user: <span className="font-mono">{userId.slice(-8)}</span>
            <button onClick={clearUserFilter} className="ml-1 hover:text-white transition-colors">
              <X className="w-3.5 h-3.5" />
            </button>
          </span>
        </div>
      )}

      {/* Date range */}
      <div className="flex items-center gap-2 mb-4 flex-wrap">
        <Calendar className="w-4 h-4 text-dark-500" />
        {DATE_PRESETS.map(p => (
          <button
            key={p.hours}
            onClick={() => applyDatePreset(p.hours)}
            className={`px-2.5 py-1 text-xs rounded-lg border transition-colors ${
              fromDate && !toDate && Math.abs(new Date().getTime() - new Date(fromDate).getTime() - p.hours * 3600000) < 60000
                ? 'border-primary-500/50 bg-primary-500/10 text-primary-400'
                : 'border-dark-700 bg-dark-800 text-dark-400 hover:text-white'
            }`}
          >
            {p.label}
          </button>
        ))}
        {fromDate && (
          <button onClick={clearDateFilter} className="px-2.5 py-1 text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg transition-colors">
            Clear dates
          </button>
        )}
      </div>

      {/* Filters row */}
      <div className="flex flex-col sm:flex-row gap-3 mb-6">
        <form onSubmit={handleSearch} className="flex-1 flex gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-500" />
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search logs..."
              className="w-full pl-10 pr-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-400 focus:outline-none focus:border-primary-500"
            />
          </div>
          <button
            type="submit"
            className="px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 transition-colors"
          >
            Search
          </button>
          {search && (
            <button
              type="button"
              onClick={() => { setSearchInput(''); setSearch(''); setPage(1); }}
              className="px-4 py-2 bg-dark-800 border border-dark-700 text-dark-300 text-sm rounded-lg hover:text-white transition-colors"
            >
              Clear
            </button>
          )}
        </form>

        <select
          value={category}
          onChange={(e) => handleCategoryChange(e.target.value)}
          className="px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
        >
          <option value="">All categories</option>
          {Object.entries(categoryLabels).map(([val, label]) => (
            <option key={val} value={val}>{label}</option>
          ))}
        </select>
      </div>

      {loading ? (
        <LoadingSpinner size="lg" className="py-20" />
      ) : logs.length === 0 ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-12 text-center">
          <FileText className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <p className="text-dark-400">No log entries found</p>
        </div>
      ) : (
        <>
          <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-dark-800">
                  <th className="text-left px-4 py-3 text-dark-400 font-medium w-10"></th>
                  <th className="text-left px-4 py-3 text-dark-400 font-medium w-44">Timestamp</th>
                  <th className="text-left px-4 py-3 text-dark-400 font-medium w-24">Severity</th>
                  <th className="text-left px-4 py-3 text-dark-400 font-medium w-24">Category</th>
                  <th className="text-left px-4 py-3 text-dark-400 font-medium w-20">User</th>
                  <th className="text-left px-4 py-3 text-dark-400 font-medium">Message</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((log) => {
                  const sev = severityConfig[log.severity] || severityConfig.debug;
                  const isExpanded = expandedRow === log.id;
                  return (
                    <>
                      <tr
                        key={log.id}
                        className="border-b border-dark-800/50 hover:bg-dark-800/30 cursor-pointer"
                        onClick={() => setExpandedRow(isExpanded ? null : log.id)}
                      >
                        <td className="px-4 py-3 text-dark-500">
                          {isExpanded ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                        </td>
                        <td className="px-4 py-3 text-dark-400 whitespace-nowrap font-mono text-xs">
                          {new Date(log.createdAt).toLocaleString()}
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${sev.color} ${sev.bg}`}>
                            {sev.label}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          {log.category ? (
                            <span className="inline-block px-2 py-0.5 rounded text-xs font-medium bg-dark-700/50 text-dark-300">
                              {categoryLabels[log.category] || log.category}
                            </span>
                          ) : (
                            <span className="text-dark-600 text-xs">—</span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          {log.userId ? (
                            <Link
                              to={`/last/users/${log.userId}`}
                              className="text-primary-400 hover:text-primary-300 text-xs font-mono transition-colors"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {log.userId.slice(-8)}
                            </Link>
                          ) : (
                            <span className="text-dark-500 text-xs">System</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-dark-200 truncate max-w-md">
                          {log.message}
                        </td>
                      </tr>
                      {isExpanded && (
                        <tr key={`${log.id}-detail`} className="border-b border-dark-800/50 bg-dark-800/20">
                          <td colSpan={6} className="px-8 py-4">
                            <div className="space-y-2 text-sm">
                              <div>
                                <span className="text-dark-400">Full message: </span>
                                <span className="text-dark-200 break-all">{log.message}</span>
                              </div>
                              {log.action && (
                                <div>
                                  <span className="text-dark-400">Action: </span>
                                  <span className="text-dark-200 font-mono">{log.action}</span>
                                </div>
                              )}
                              {log.tenantId && (
                                <div>
                                  <span className="text-dark-400">Tenant: </span>
                                  <Link to={`/last/tenants/${log.tenantId}`} className="text-primary-400 hover:text-primary-300 font-mono text-xs">
                                    {log.tenantId}
                                  </Link>
                                </div>
                              )}
                              {log.userId && (
                                <div>
                                  <span className="text-dark-400">User: </span>
                                  <Link to={`/last/users/${log.userId}`} className="text-primary-400 hover:text-primary-300 font-mono text-xs">
                                    {log.userId}
                                  </Link>
                                </div>
                              )}
                              {log.metadata && Object.keys(log.metadata).length > 0 && (
                                <div>
                                  <span className="text-dark-400">Metadata: </span>
                                  <pre className="mt-1 p-2 bg-dark-900 rounded text-xs text-dark-300 overflow-x-auto">
                                    {JSON.stringify(log.metadata, null, 2)}
                                  </pre>
                                </div>
                              )}
                            </div>
                          </td>
                        </tr>
                      )}
                    </>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          <div className="flex items-center justify-between mt-4">
            <div className="flex items-center gap-3">
              <p className="text-sm text-dark-400">
                Showing {((page - 1) * perPage) + 1}–{Math.min(page * perPage, total)} of {total.toLocaleString()}
              </p>
              <select
                value={perPage}
                onChange={(e) => { setPerPage(Number(e.target.value)); setPage(1); }}
                className="px-2 py-1 bg-dark-800 border border-dark-700 rounded text-xs text-dark-300 focus:outline-none"
              >
                {PER_PAGE_OPTIONS.map(n => (
                  <option key={n} value={n}>{n} / page</option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
                className="flex items-center gap-1 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white disabled:opacity-40 disabled:hover:text-dark-300 transition-colors"
              >
                <ChevronLeft className="w-4 h-4" /> Prev
              </button>
              <span className="text-sm text-dark-400">
                {page} / {totalPages}
              </span>
              <button
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="flex items-center gap-1 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white disabled:opacity-40 disabled:hover:text-dark-300 transition-colors"
              >
                Next <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
