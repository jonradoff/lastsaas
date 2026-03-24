import { useState } from 'react';
import { Download } from 'lucide-react';
import { billingApi } from '../../../api/client';
import type { FinancialTransaction } from '../../../types';
import LoadingSpinner from '../../../components/LoadingSpinner';

interface InvoiceModalProps {
  tx: FinancialTransaction;
  tenantName: string;
  onClose: () => void;
}

export default function InvoiceModal({ tx, tenantName, onClose }: InvoiceModalProps) {
  const [downloading, setDownloading] = useState(false);

  const handleDownload = async () => {
    setDownloading(true);
    try {
      const blob = await billingApi.getInvoicePDF(tx.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `invoice-${tx.invoiceNumber}.pdf`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // Error handled by interceptor
    } finally {
      setDownloading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-lg mx-4 w-full" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold text-dark-50">Invoice</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-dark-50" aria-label="Close">&times;</button>
        </div>

        <div className="space-y-4 mb-6">
          <div className="flex justify-between">
            <span className="text-dark-400 text-sm">Invoice Number</span>
            <span className="text-dark-50 text-sm font-mono">{tx.invoiceNumber}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-dark-400 text-sm">Date</span>
            <span className="text-dark-50 text-sm">{new Date(tx.createdAt).toLocaleDateString()}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-dark-400 text-sm">Bill To</span>
            <span className="text-dark-50 text-sm">{tenantName}</span>
          </div>
          <hr className="border-dark-800" />
          <div className="flex justify-between">
            <span className="text-dark-300 text-sm">{tx.description}</span>
            <span className="text-dark-50 text-sm font-medium">
              ${((tx.taxAmountCents > 0 ? (tx.subtotalCents || tx.amountCents) : tx.amountCents) / 100).toFixed(2)}
            </span>
          </div>
          {tx.taxAmountCents > 0 && (
            <>
              <div className="flex justify-between">
                <span className="text-dark-400 text-sm">Subtotal</span>
                <span className="text-dark-300 text-sm">${((tx.subtotalCents || tx.amountCents) / 100).toFixed(2)}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-dark-400 text-sm">Tax</span>
                <span className="text-dark-300 text-sm">${(tx.taxAmountCents / 100).toFixed(2)}</span>
              </div>
            </>
          )}
          <hr className="border-dark-800" />
          <div className="flex justify-between">
            <span className="text-dark-50 font-semibold">Total</span>
            <span className="text-dark-50 font-semibold">${(tx.amountCents / 100).toFixed(2)} {tx.currency.toUpperCase()}</span>
          </div>
        </div>

        <button
          onClick={handleDownload}
          disabled={downloading}
          className="w-full py-2.5 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 disabled:opacity-60 transition-colors flex items-center justify-center gap-2"
        >
          {downloading ? <LoadingSpinner size="sm" /> : <><Download className="w-4 h-4" /> Download PDF</>}
        </button>
      </div>
    </div>
  );
}
