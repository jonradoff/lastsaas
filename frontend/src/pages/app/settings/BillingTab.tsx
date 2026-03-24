import { useEffect, useState } from 'react';
import { CreditCard, Receipt, ExternalLink } from 'lucide-react';
import { toast } from 'sonner';
import { billingApi, plansApi } from '../../../api/client';
import { getErrorMessage } from '../../../utils/errors';
import type { FinancialTransaction, BillingStatus } from '../../../types';
import LoadingSpinner from '../../../components/LoadingSpinner';
import InvoiceModal from './InvoiceModal';

export default function BillingTab() {
  const [billingStatus, setBillingStatus] = useState<BillingStatus>('none');
  const [billingInterval, setBillingInterval] = useState('');
  const [currentPeriodEnd, setCurrentPeriodEnd] = useState('');
  const [currentPlanName, setCurrentPlanName] = useState('');
  const [tenantName] = useState('');
  const [transactions, setTransactions] = useState<FinancialTransaction[]>([]);
  const [txTotal, setTxTotal] = useState(0);
  const [txPage, setTxPage] = useState(1);
  const [billingLoading, setBillingLoading] = useState(false);
  const [portalLoading, setPortalLoading] = useState(false);
  const [selectedTx, setSelectedTx] = useState<FinancialTransaction | null>(null);

  const loadBillingData = () => {
    setBillingLoading(true);
    Promise.all([
      plansApi.list(),
      billingApi.listTransactions({ page: txPage, perPage: 10 }),
    ])
      .then(([planData, txData]) => {
        setBillingStatus(planData.billingStatus || 'none');
        setBillingInterval(planData.billingInterval || '');
        setCurrentPeriodEnd(planData.currentPeriodEnd || '');
        const plan = planData.plans.find(p => p.id === planData.currentPlanId);
        setCurrentPlanName(plan?.name || 'Free');
        setTransactions(txData.transactions);
        setTxTotal(txData.total);
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setBillingLoading(false));
  };

  useEffect(() => { loadBillingData(); }, [txPage]);

  const handlePortal = async () => {
    setPortalLoading(true);
    try {
      const result = await billingApi.portal();
      window.location.href = result.portalUrl;
    } catch {
      // Error handled by interceptor
    } finally {
      setPortalLoading(false);
    }
  };

  const totalPages = Math.ceil(txTotal / 10);

  if (billingLoading) {
    return <LoadingSpinner size="lg" className="py-20" />;
  }

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Subscription Summary */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-dark-50 flex items-center gap-2 mb-4">
          <CreditCard className="w-5 h-5 text-dark-400" />
          Subscription
        </h2>
        <div className="space-y-3">
          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-dark-400">Plan</span>
            <span className="text-sm text-dark-50 font-medium">{currentPlanName}</span>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Status</span>
            <span className={`text-sm font-medium ${
              billingStatus === 'active' ? 'text-accent-emerald' :
              billingStatus === 'past_due' ? 'text-red-400' :
              billingStatus === 'canceled' ? 'text-yellow-400' :
              'text-dark-400'
            }`}>
              {billingStatus === 'active' ? 'Active' :
               billingStatus === 'past_due' ? 'Past Due' :
               billingStatus === 'canceled' ? 'Canceled' :
               'None'}
            </span>
          </div>
          {billingInterval && (
            <div className="flex items-center justify-between py-2 border-t border-dark-800">
              <span className="text-sm text-dark-400">Billing Interval</span>
              <span className="text-sm text-dark-50 capitalize">{billingInterval}ly</span>
            </div>
          )}
          {currentPeriodEnd && (
            <div className="flex items-center justify-between py-2 border-t border-dark-800">
              <span className="text-sm text-dark-400">
                {billingStatus === 'canceled' ? 'Benefits Until' : 'Next Billing'}
              </span>
              <span className="text-sm text-dark-50">{new Date(currentPeriodEnd).toLocaleDateString()}</span>
            </div>
          )}
        </div>

        {billingStatus !== 'none' && (
          <div className="mt-4 pt-4 border-t border-dark-800">
            <button
              onClick={handlePortal}
              disabled={portalLoading}
              className="inline-flex items-center gap-2 px-4 py-2 text-sm bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:border-dark-600 hover:text-dark-50 transition-colors disabled:opacity-60"
            >
              {portalLoading ? <LoadingSpinner size="sm" /> : <><ExternalLink className="w-4 h-4" /> Update Payment Method</>}
            </button>
          </div>
        )}
      </div>

      {/* Transaction History */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
        <div className="px-6 py-4 border-b border-dark-800">
          <h2 className="text-lg font-semibold text-dark-50 flex items-center gap-2">
            <Receipt className="w-5 h-5 text-dark-400" />
            Transaction History
          </h2>
        </div>

        {transactions.length === 0 ? (
          <div className="text-center py-12 text-dark-400">No transactions yet</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-dark-800">
                    <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Date</th>
                    <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Description</th>
                    <th className="text-right px-6 py-3 text-xs font-medium text-dark-400 uppercase">Amount</th>
                    <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Invoice</th>
                    <th className="px-6 py-3"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-dark-800/50">
                  {transactions.map(tx => (
                    <tr key={tx.id} className="hover:bg-dark-800/20">
                      <td className="px-6 py-3 text-sm text-dark-300 whitespace-nowrap">
                        {new Date(tx.createdAt).toLocaleDateString()}
                      </td>
                      <td className="px-6 py-3 text-sm text-dark-50">{tx.description}</td>
                      <td className="px-6 py-3 text-sm text-dark-50 text-right font-mono">
                        ${(tx.amountCents / 100).toFixed(2)}
                        {tx.taxAmountCents > 0 && (
                          <span className="block text-xs text-dark-500">incl. ${(tx.taxAmountCents / 100).toFixed(2)} tax</span>
                        )}
                      </td>
                      <td className="px-6 py-3 text-sm text-dark-400 font-mono">{tx.invoiceNumber}</td>
                      <td className="px-6 py-3">
                        <button
                          onClick={() => setSelectedTx(tx)}
                          className="text-xs text-primary-400 hover:text-primary-300 transition-colors"
                        >
                          View
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {totalPages > 1 && (
              <div className="px-6 py-3 border-t border-dark-800 flex items-center justify-between">
                <span className="text-xs text-dark-400">{txTotal} total</span>
                <div className="flex gap-2">
                  <button
                    onClick={() => setTxPage(p => Math.max(1, p - 1))}
                    disabled={txPage === 1}
                    className="px-3 py-1 text-xs bg-dark-800 text-dark-300 rounded disabled:opacity-40"
                  >
                    Prev
                  </button>
                  <span className="px-3 py-1 text-xs text-dark-400">{txPage} / {totalPages}</span>
                  <button
                    onClick={() => setTxPage(p => Math.min(totalPages, p + 1))}
                    disabled={txPage === totalPages}
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

      {selectedTx && (
        <InvoiceModal tx={selectedTx} tenantName={tenantName || 'Your Organization'} onClose={() => setSelectedTx(null)} />
      )}
    </div>
  );
}
