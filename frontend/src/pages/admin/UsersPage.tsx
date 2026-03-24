import { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Users, CheckCircle, XCircle, Search, ChevronLeft, ChevronRight, ArrowUpDown, UserCheck, Download } from 'lucide-react';
import { toast } from 'sonner';
import { adminApi, setAuthToken } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import { useAuth } from '../../contexts/AuthContext';
import { useTenant } from '../../contexts/TenantContext';
import type { UserListItem } from '../../types';
import TableSkeleton from '../../components/TableSkeleton';
import ConfirmModal from '../../components/ConfirmModal';

const PAGE_SIZE = 25;

function relativeTime(dateStr?: string): string {
  if (!dateStr) return 'Never';
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffSec = Math.floor((now - then) / 1000);
  if (diffSec < 60) return 'Just now';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  const diffMonth = Math.floor(diffDay / 30);
  if (diffMonth < 12) return `${diffMonth}mo ago`;
  return `${Math.floor(diffMonth / 12)}y ago`;
}

export default function UsersPage() {
  const navigate = useNavigate();
  const { user: currentUser, refreshUser } = useAuth();
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';
  const isOwner = role === 'owner';
  const [searchParams, setSearchParams] = useSearchParams();

  const [users, setUsers] = useState<UserListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [page, setPage] = useState(Number(searchParams.get('page')) || 1);
  const [sort, setSort] = useState(searchParams.get('sort') || '-createdAt');
  const [status, setStatus] = useState(searchParams.get('status') || '');
  const [statusTarget, setStatusTarget] = useState<UserListItem | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const fetchUsers = useCallback(async (p: number, q: string, s: string, st: string) => {
    setLoading(true);
    try {
      const data = await adminApi.listUsers({ page: p, limit: PAGE_SIZE, search: q || undefined, sort: s, status: st || undefined });
      setUsers(data.users || []);
      setTotal(data.total);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  // Sync URL params
  useEffect(() => {
    const params: Record<string, string> = {};
    if (page > 1) params.page = String(page);
    if (search) params.search = search;
    if (sort && sort !== '-createdAt') params.sort = sort;
    if (status) params.status = status;
    setSearchParams(params, { replace: true });
  }, [page, search, sort, status, setSearchParams]);

  // Fetch on page/sort/status change
  useEffect(() => {
    fetchUsers(page, search, sort, status);
  }, [page, sort, status, fetchUsers]); // eslint-disable-line react-hooks/exhaustive-deps

  // Debounced search
  const handleSearchChange = (value: string) => {
    setSearch(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setPage(1);
      fetchUsers(1, value, sort, status);
    }, 300);
  };

  const toggleSort = (field: string) => {
    setSort(prev => prev === field ? `-${field}` : prev === `-${field}` ? field : field);
    setPage(1);
  };

  const handleStatusChange = (value: string) => {
    setStatus(value);
    setPage(1);
  };

  const handleExport = async () => {
    try {
      const blob = await adminApi.exportUsersCSV({ search: search || undefined, status: status || undefined });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'users.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // ignore
    }
  };

  const toggleStatus = async (user: UserListItem) => {
    setStatusLoading(true);
    try {
      await adminApi.updateUserStatus(user.id, !user.isActive);
      setUsers(prev => prev.map(u => u.id === user.id ? { ...u, isActive: !u.isActive } : u));
      toast.success(`${user.displayName} ${user.isActive ? 'disabled' : 'enabled'}`);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setStatusLoading(false);
      setStatusTarget(null);
    }
  };

  const handleImpersonate = async (userId: string) => {
    try {
      const data = await adminApi.impersonateUser(userId);
      localStorage.setItem('mcplens_access_token', data.accessToken);
      localStorage.removeItem('mcplens_refresh_token');
      localStorage.setItem('mcplens_impersonating', 'true');
      setAuthToken(data.accessToken);
      await refreshUser();
      navigate('/dashboard');
    } catch (err) {
      toast.error(getErrorMessage(err));
    }
  };

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Users className="w-7 h-7 text-primary-400" />
            Users
          </h1>
          <p className="text-dark-400 mt-1">{total.toLocaleString()} total users</p>
        </div>
        <button
          onClick={handleExport}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white transition-colors"
          title="Download CSV"
        >
          <Download className="w-3.5 h-3.5" />
          CSV
        </button>
      </div>

      {/* Search + Filters */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-500" />
          <input
            type="text"
            placeholder="Search by name or email..."
            value={search}
            onChange={(e) => handleSearchChange(e.target.value)}
            className="w-full pl-10 pr-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors text-sm"
          />
        </div>
        <select
          value={status}
          onChange={(e) => handleStatusChange(e.target.value)}
          className="px-3 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
        >
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="disabled">Disabled</option>
        </select>
      </div>

      {/* Table */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
        {loading && users.length === 0 ? (
          <TableSkeleton rows={8} cols={7} />
        ) : users.length === 0 ? (
          <div className="py-16 text-center text-dark-400">
            {search ? 'No users match your search.' : 'No users yet.'}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-dark-800">
                    <th className="text-left px-6 py-3.5">
                      <button onClick={() => toggleSort('displayName')} className="flex items-center gap-1.5 text-sm font-medium text-dark-400 hover:text-white transition-colors">
                        User
                        <ArrowUpDown className="w-3 h-3" />
                      </button>
                    </th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Verified</th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Tenants</th>
                    <th className="text-left px-6 py-3.5">
                      <button onClick={() => toggleSort('createdAt')} className="flex items-center gap-1.5 text-sm font-medium text-dark-400 hover:text-white transition-colors">
                        Joined
                        <ArrowUpDown className="w-3 h-3" />
                      </button>
                    </th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Last Login</th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Status</th>
                    {canWrite && <th className="text-right px-6 py-3.5 text-sm font-medium text-dark-400">Actions</th>}
                  </tr>
                </thead>
                <tbody>
                  {users.map((user) => (
                    <tr
                      key={user.id}
                      onClick={() => navigate(`/last/users/${user.id}`)}
                      className="border-b border-dark-800/50 hover:bg-dark-800/30 transition-colors cursor-pointer"
                    >
                      <td className="px-6 py-3.5">
                        <p className="text-sm font-medium text-white">{user.displayName}</p>
                        <p className="text-xs text-dark-500">{user.email}</p>
                      </td>
                      <td className="px-6 py-3.5">
                        {user.emailVerified ? (
                          <CheckCircle className="w-4 h-4 text-accent-emerald" />
                        ) : (
                          <XCircle className="w-4 h-4 text-dark-500" />
                        )}
                      </td>
                      <td className="px-6 py-3.5 text-sm text-dark-300">{user.tenantCount}</td>
                      <td className="px-6 py-3.5 text-sm text-dark-400">
                        {new Date(user.createdAt).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-3.5 text-sm text-dark-400" title={user.lastLoginAt ? new Date(user.lastLoginAt).toLocaleString() : undefined}>
                        {relativeTime(user.lastLoginAt)}
                      </td>
                      <td className="px-6 py-3.5">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          user.isActive
                            ? 'bg-accent-emerald/10 text-accent-emerald'
                            : 'bg-red-500/10 text-red-400'
                        }`}>
                          {user.isActive ? 'Active' : 'Disabled'}
                        </span>
                      </td>
                      {canWrite && (
                      <td className="px-6 py-3.5 text-right">
                        <div className="flex items-center justify-end gap-1">
                          {isOwner && currentUser && user.id !== currentUser.id && (
                            <button
                              onClick={(e) => { e.stopPropagation(); handleImpersonate(user.id); }}
                              className="text-xs px-3 py-1.5 rounded-lg border border-primary-500/30 text-primary-400 hover:bg-primary-500/10 transition-colors"
                              title="Impersonate user"
                            >
                              <UserCheck className="w-3.5 h-3.5" />
                            </button>
                          )}
                          <button
                            onClick={(e) => { e.stopPropagation(); setStatusTarget(user); }}
                            className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                              user.isActive
                                ? 'border-red-500/30 text-red-400 hover:bg-red-500/10'
                                : 'border-accent-emerald/30 text-accent-emerald hover:bg-accent-emerald/10'
                            }`}
                          >
                            {user.isActive ? 'Disable' : 'Enable'}
                          </button>
                        </div>
                      </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-between px-6 py-3 border-t border-dark-800">
                <p className="text-sm text-dark-400">
                  Showing {((page - 1) * PAGE_SIZE) + 1}–{Math.min(page * PAGE_SIZE, total)} of {total.toLocaleString()}
                </p>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => setPage(p => Math.max(1, p - 1))}
                    disabled={page <= 1}
                    className="p-1.5 rounded-lg text-dark-400 hover:text-white hover:bg-dark-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => {
                    let p: number;
                    if (totalPages <= 7) {
                      p = i + 1;
                    } else if (page <= 4) {
                      p = i + 1;
                    } else if (page >= totalPages - 3) {
                      p = totalPages - 6 + i;
                    } else {
                      p = page - 3 + i;
                    }
                    return (
                      <button
                        key={p}
                        onClick={() => setPage(p)}
                        className={`min-w-[32px] h-8 rounded-lg text-sm font-medium transition-colors ${
                          p === page
                            ? 'bg-primary-500 text-white'
                            : 'text-dark-400 hover:text-white hover:bg-dark-800'
                        }`}
                      >
                        {p}
                      </button>
                    );
                  })}
                  <button
                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                    disabled={page >= totalPages}
                    className="p-1.5 rounded-lg text-dark-400 hover:text-white hover:bg-dark-800 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>

      <ConfirmModal
        open={statusTarget !== null}
        onClose={() => setStatusTarget(null)}
        onConfirm={() => statusTarget && toggleStatus(statusTarget)}
        title={statusTarget?.isActive ? 'Disable User' : 'Enable User'}
        message={`Are you sure you want to ${statusTarget?.isActive ? 'disable' : 'enable'} ${statusTarget?.displayName}?`}
        confirmLabel={statusTarget?.isActive ? 'Disable' : 'Enable'}
        confirmVariant={statusTarget?.isActive ? 'danger' : 'primary'}
        loading={statusLoading}
      />
    </div>
  );
}
