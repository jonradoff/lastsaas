import { useEffect, useState, useCallback } from 'react';
import { Activity, ChevronLeft, ChevronRight, Search, X } from 'lucide-react';
import { toast } from 'sonner';
import { tenantApi } from '../../api/client';
import type { ActivityLogEntry } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-500/20 text-red-400',
  high: 'bg-orange-500/20 text-orange-400',
  medium: 'bg-yellow-500/20 text-yellow-400',
  low: 'bg-blue-500/20 text-blue-400',
  debug: 'bg-dark-700 text-dark-400',
};

const PAGE_SIZE = 25;

export default function ActivityPage() {
  const [logs, setLogs] = useState<ActivityLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [action, setAction] = useState('');
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');

  const fetchActivity = useCallback(() => {
    setLoading(true);
    const params: Record<string, string | number> = { page, perPage: PAGE_SIZE };
    if (action) params.action = action;
    if (search) params.search = search;
    tenantApi.getActivity(params)
      .then(data => {
        setLogs(data.logs || []);
        setTotal(data.total);
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, [page, action, search]);

  useEffect(() => { fetchActivity(); }, [fetchActivity]);

  const totalPages = Math.ceil(total / PAGE_SIZE);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setSearch(searchInput);
  };

  const clearSearch = () => {
    setSearchInput('');
    setSearch('');
    setPage(1);
  };

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
          <Activity className="w-7 h-7 text-primary-400" />
          Activity
        </h1>
        <p className="text-dark-400 mt-1">Recent activity in your organization</p>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3 mb-6">
        <form onSubmit={handleSearch} className="flex-1 flex gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-500" />
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search activity..."
              className="w-full pl-10 pr-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-50 placeholder-dark-400 focus:outline-none focus:border-primary-500"
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
              onClick={clearSearch}
              className="flex items-center gap-1 px-3 py-2 bg-dark-800 border border-dark-700 text-dark-300 text-sm rounded-lg hover:text-dark-50 transition-colors"
            >
              <X className="w-3.5 h-3.5" /> Clear
            </button>
          )}
        </form>
        <input
          type="text"
          value={action}
          onChange={(e) => { setAction(e.target.value); setPage(1); }}
          placeholder="Filter by action..."
          className="px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-50 placeholder-dark-400 focus:outline-none focus:border-primary-500 w-48"
        />
      </div>

      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
        {loading && logs.length === 0 ? (
          <div className="py-20"><LoadingSpinner size="lg" /></div>
        ) : logs.length === 0 ? (
          <div className="py-16 text-center text-dark-400">No activity recorded yet.</div>
        ) : (
          <>
            <div className="divide-y divide-dark-800/50">
              {logs.map(log => (
                <div key={log.id} className="px-6 py-4 hover:bg-dark-800/20 transition-colors">
                  <div className="flex items-start gap-3">
                    <span className={`mt-0.5 px-2 py-0.5 text-xs font-medium rounded-full ${SEVERITY_COLORS[log.severity] || SEVERITY_COLORS.low}`}>
                      {log.severity}
                    </span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-dark-50">{log.message}</p>
                      {log.action && (
                        <span className="inline-block mt-1 px-2 py-0.5 bg-dark-800 rounded text-xs text-dark-400 font-mono">{log.action}</span>
                      )}
                    </div>
                    <span className="text-xs text-dark-500 whitespace-nowrap shrink-0">
                      {new Date(log.createdAt).toLocaleString()}
                    </span>
                  </div>
                </div>
              ))}
            </div>

            {totalPages > 1 && (
              <div className="flex items-center justify-between px-6 py-3 border-t border-dark-800">
                <p className="text-sm text-dark-400">
                  Showing {((page - 1) * PAGE_SIZE) + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}
                </p>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => setPage(p => Math.max(1, p - 1))}
                    disabled={page <= 1}
                    className="p-1.5 rounded-lg text-dark-400 hover:text-dark-50 hover:bg-dark-800 disabled:opacity-30 transition-colors"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <span className="px-3 py-1 text-sm text-dark-400">{page} / {totalPages}</span>
                  <button
                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                    disabled={page >= totalPages}
                    className="p-1.5 rounded-lg text-dark-400 hover:text-dark-50 hover:bg-dark-800 disabled:opacity-30 transition-colors"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
