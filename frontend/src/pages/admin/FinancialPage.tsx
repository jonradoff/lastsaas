import { useEffect, useState, useCallback } from 'react';
import { DollarSign, Search } from 'lucide-react';
import { toast } from 'sonner';
import { adminApi } from '../../api/client';
import type { FinancialTransaction } from '../../types';
import TableSkeleton from '../../components/TableSkeleton';
import { getErrorMessage } from '../../utils/errors';

export default function AdminFinancialPage() {
  const [transactions, setTransactions] = useState<FinancialTransaction[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(true);

  const [debouncedSearch, setDebouncedSearch] = useState('');

  const loadData = useCallback(() => {
    setLoading(true);
    adminApi.listFinancialTransactions({ page, perPage: 50, search: debouncedSearch || undefined })
      .then((data) => {
        setTransactions(data.transactions);
        setTotal(data.total);
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, [page, debouncedSearch]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setDebouncedSearch(search);
  };

  const totalPages = Math.ceil(total / 50);

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white flex items-center gap-3">
          <DollarSign className="w-7 h-7 text-primary-400" />
          Financial
        </h1>
        <p className="text-dark-400 mt-1">All financial transactions across the platform</p>
      </div>

      {/* Search */}
      <form onSubmit={handleSearch} className="mb-6">
        <div className="relative max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-500" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by description, invoice #, plan..."
            className="w-full pl-10 pr-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 text-sm"
          />
        </div>
      </form>

      {loading ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
          <TableSkeleton rows={8} cols={5} />
        </div>
      ) : (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
          {transactions.length === 0 ? (
            <div className="text-center py-12 text-dark-400">No transactions found</div>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-dark-800">
                      <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Date</th>
                      <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Type</th>
                      <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Description</th>
                      <th className="text-right px-6 py-3 text-xs font-medium text-dark-400 uppercase">Amount</th>
                      <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Invoice #</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-dark-800/50">
                    {transactions.map(tx => (
                      <tr key={tx.id} className="hover:bg-dark-800/20">
                        <td className="px-6 py-3 text-sm text-dark-300 whitespace-nowrap">
                          {new Date(tx.createdAt).toLocaleDateString()}
                        </td>
                        <td className="px-6 py-3">
                          <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                            tx.type === 'subscription' ? 'bg-primary-500/10 text-primary-400' :
                            tx.type === 'credit_purchase' ? 'bg-accent-emerald/10 text-accent-emerald' :
                            'bg-yellow-500/10 text-yellow-400'
                          }`}>
                            {tx.type === 'subscription' ? 'Subscription' :
                             tx.type === 'credit_purchase' ? 'Credit Purchase' :
                             'Refund'}
                          </span>
                        </td>
                        <td className="px-6 py-3 text-sm text-white">{tx.description}</td>
                        <td className="px-6 py-3 text-sm text-white text-right font-mono">
                          {new Intl.NumberFormat(undefined, { style: 'currency', currency: tx.currency || 'usd' }).format(tx.amountCents / 100)}
                          {tx.taxAmountCents > 0 && (
                            <span className="block text-xs text-dark-500">
                              incl. {new Intl.NumberFormat(undefined, { style: 'currency', currency: tx.currency || 'usd' }).format(tx.taxAmountCents / 100)} tax
                            </span>
                          )}
                        </td>
                        <td className="px-6 py-3 text-sm text-dark-400 font-mono">{tx.invoiceNumber}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {totalPages > 1 && (
                <div className="px-6 py-3 border-t border-dark-800 flex items-center justify-between">
                  <span className="text-xs text-dark-400">{total} total</span>
                  <div className="flex gap-2">
                    <button
                      onClick={() => setPage(p => Math.max(1, p - 1))}
                      disabled={page === 1}
                      className="px-3 py-1 text-xs bg-dark-800 text-dark-300 rounded disabled:opacity-40"
                    >
                      Prev
                    </button>
                    <span className="px-3 py-1 text-xs text-dark-400">{page} / {totalPages}</span>
                    <button
                      onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                      disabled={page === totalPages}
                      className="px-3 py-1 text-xs bg-dark-800 text-dark-300 rounded disabled:opacity-40"
                    >
                      Next
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
