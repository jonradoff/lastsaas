import { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Building2, Shield, Zap, Search, ChevronLeft, ChevronRight, ArrowUpDown, Download } from 'lucide-react';
import { toast } from 'sonner';
import { adminApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import { useTenant } from '../../contexts/TenantContext';
import type { TenantListItem } from '../../types';
import TableSkeleton from '../../components/TableSkeleton';
import ConfirmModal from '../../components/ConfirmModal';

const PAGE_SIZE = 25;

export default function TenantsPage() {
  const navigate = useNavigate();
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';
  const [searchParams, setSearchParams] = useSearchParams();

  const [tenants, setTenants] = useState<TenantListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [page, setPage] = useState(Number(searchParams.get('page')) || 1);
  const [sort, setSort] = useState(searchParams.get('sort') || '-createdAt');
  const [status, setStatus] = useState(searchParams.get('status') || '');
  const [billingStatus, setBillingStatus] = useState(searchParams.get('billingStatus') || '');
  const [statusTarget, setStatusTarget] = useState<TenantListItem | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const fetchTenants = useCallback(async (p: number, q: string, s: string, st: string, bs: string) => {
    setLoading(true);
    try {
      const data = await adminApi.listTenants({ page: p, limit: PAGE_SIZE, search: q || undefined, sort: s, status: st || undefined, billingStatus: bs || undefined });
      setTenants(data.tenants || []);
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
    if (billingStatus) params.billingStatus = billingStatus;
    setSearchParams(params, { replace: true });
  }, [page, search, sort, status, billingStatus, setSearchParams]);

  // Fetch on page/sort/filter change
  useEffect(() => {
    fetchTenants(page, search, sort, status, billingStatus);
  }, [page, sort, status, billingStatus, fetchTenants]); // eslint-disable-line react-hooks/exhaustive-deps

  // Debounced search
  const handleSearchChange = (value: string) => {
    setSearch(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setPage(1);
      fetchTenants(1, value, sort, status, billingStatus);
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

  const handleBillingStatusChange = (value: string) => {
    setBillingStatus(value);
    setPage(1);
  };

  const handleExport = async () => {
    try {
      const blob = await adminApi.exportTenantsCSV({ search: search || undefined, status: status || undefined, billingStatus: billingStatus || undefined });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'tenants.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // ignore
    }
  };

  const toggleStatus = async (tenant: TenantListItem) => {
    if (tenant.isRoot) return;
    setStatusLoading(true);
    try {
      await adminApi.updateTenantStatus(tenant.id, !tenant.isActive);
      setTenants(prev => prev.map(t => t.id === tenant.id ? { ...t, isActive: !t.isActive } : t));
      toast.success(`${tenant.name} ${tenant.isActive ? 'disabled' : 'enabled'}`);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setStatusLoading(false);
      setStatusTarget(null);
    }
  };

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Building2 className="w-7 h-7 text-accent-purple" />
            Tenants
          </h1>
          <p className="text-dark-400 mt-1">{total.toLocaleString()} total tenants</p>
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
            placeholder="Search by name or slug..."
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
        <select
          value={billingStatus}
          onChange={(e) => handleBillingStatusChange(e.target.value)}
          className="px-3 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
        >
          <option value="">All billing</option>
          <option value="active">Active</option>
          <option value="past_due">Past Due</option>
          <option value="canceled">Canceled</option>
          <option value="none">None</option>
        </select>
      </div>

      {/* Table */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
        {loading && tenants.length === 0 ? (
          <TableSkeleton rows={8} cols={7} />
        ) : tenants.length === 0 ? (
          <div className="py-16 text-center text-dark-400">
            {search ? 'No tenants match your search.' : 'No tenants yet.'}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-dark-800">
                    <th className="text-left px-6 py-3.5">
                      <button onClick={() => toggleSort('name')} className="flex items-center gap-1.5 text-sm font-medium text-dark-400 hover:text-white transition-colors">
                        Tenant
                        <ArrowUpDown className="w-3 h-3" />
                      </button>
                    </th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Plan</th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Credits</th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Members</th>
                    <th className="text-left px-6 py-3.5">
                      <button onClick={() => toggleSort('createdAt')} className="flex items-center gap-1.5 text-sm font-medium text-dark-400 hover:text-white transition-colors">
                        Created
                        <ArrowUpDown className="w-3 h-3" />
                      </button>
                    </th>
                    <th className="text-left px-6 py-3.5 text-sm font-medium text-dark-400">Status</th>
                    {canWrite && <th className="text-right px-6 py-3.5 text-sm font-medium text-dark-400">Actions</th>}
                  </tr>
                </thead>
                <tbody>
                  {tenants.map((tenant) => (
                    <tr
                      key={tenant.id}
                      onClick={() => navigate(`/last/tenants/${tenant.id}`)}
                      className="border-b border-dark-800/50 hover:bg-dark-800/30 transition-colors cursor-pointer"
                    >
                      <td className="px-6 py-3.5">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium text-white">{tenant.name}</p>
                          {tenant.isRoot && (
                            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-accent-purple/10 text-accent-purple">
                              <Shield className="w-3 h-3" />
                              Root
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-dark-500 font-mono">{tenant.slug}</p>
                      </td>
                      <td className="px-6 py-3.5 text-sm text-dark-300">{tenant.planName}</td>
                      <td className="px-6 py-3.5">
                        <div className="flex items-center gap-1 text-sm text-dark-300">
                          <Zap className="w-3.5 h-3.5 text-primary-400" />
                          {(tenant.subscriptionCredits + tenant.purchasedCredits).toLocaleString()}
                        </div>
                      </td>
                      <td className="px-6 py-3.5 text-sm text-dark-300">{tenant.memberCount}</td>
                      <td className="px-6 py-3.5 text-sm text-dark-400">
                        {new Date(tenant.createdAt).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-3.5">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          tenant.isActive
                            ? 'bg-accent-emerald/10 text-accent-emerald'
                            : 'bg-red-500/10 text-red-400'
                        }`}>
                          {tenant.isActive ? 'Active' : 'Disabled'}
                        </span>
                      </td>
                      {canWrite && (
                      <td className="px-6 py-3.5 text-right">
                        {!tenant.isRoot && (
                          <button
                            onClick={(e) => { e.stopPropagation(); setStatusTarget(tenant); }}
                            className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                              tenant.isActive
                                ? 'border-red-500/30 text-red-400 hover:bg-red-500/10'
                                : 'border-accent-emerald/30 text-accent-emerald hover:bg-accent-emerald/10'
                            }`}
                          >
                            {tenant.isActive ? 'Disable' : 'Enable'}
                          </button>
                        )}
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
        title={statusTarget?.isActive ? 'Disable Tenant' : 'Enable Tenant'}
        message={`Are you sure you want to ${statusTarget?.isActive ? 'disable' : 'enable'} ${statusTarget?.name}? ${statusTarget?.isActive ? 'All members will lose access.' : ''}`}
        confirmLabel={statusTarget?.isActive ? 'Disable' : 'Enable'}
        confirmVariant={statusTarget?.isActive ? 'danger' : 'primary'}
        loading={statusLoading}
      />
    </div>
  );
}
