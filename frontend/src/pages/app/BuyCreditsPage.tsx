import { useEffect, useState } from 'react';
import { Zap, ShoppingCart } from 'lucide-react';
import { toast } from 'sonner';
import { bundlesApi, plansApi, billingApi } from '../../api/client';
import type { CreditBundle } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';

function formatPrice(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function BuyCreditsPage() {
  const [bundles, setBundles] = useState<CreditBundle[]>([]);
  const [totalCredits, setTotalCredits] = useState(0);
  const [loading, setLoading] = useState(true);
  const [checkoutLoading, setCheckoutLoading] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      bundlesApi.list(),
      plansApi.list(),
    ])
      .then(([bundleData, planData]) => {
        setBundles(bundleData.bundles);
        setTotalCredits(planData.tenantSubscriptionCredits + planData.tenantPurchasedCredits);
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  const handleBuy = async (bundleId: string) => {
    setCheckoutLoading(bundleId);
    try {
      const result = await billingApi.checkout({ bundleId });
      if (result.checkoutUrl) {
        window.location.href = result.checkoutUrl;
      }
    } catch {
      // Error handled by interceptor
    } finally {
      setCheckoutLoading(null);
    }
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  return (
    <div>
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
          <Zap className="w-7 h-7 text-primary-400" />
          Buy Credits
        </h1>
        <p className="text-dark-400 mt-1">
          Purchase additional usage credits for your account
        </p>
        <div className="mt-3 inline-flex items-center gap-2 px-4 py-2 bg-dark-900/50 border border-dark-800 rounded-lg">
          <Zap className="w-4 h-4 text-primary-400" />
          <span className="text-sm text-dark-300">
            Current balance:{' '}
            <span className="text-dark-50 font-semibold">
              {totalCredits.toLocaleString()}
            </span>
            {' '}credits
          </span>
        </div>
      </div>

      {bundles.length === 0 ? (
        <div className="text-center py-20 text-dark-400">
          No credit bundles are available for purchase at this time.
        </div>
      ) : (
        <div className={`grid gap-6 ${
          bundles.length === 1 ? 'grid-cols-1' :
          bundles.length === 2 ? 'grid-cols-1 md:grid-cols-2' :
          bundles.length === 3 ? 'grid-cols-1 md:grid-cols-3' :
          'grid-cols-1 md:grid-cols-2 lg:grid-cols-3'
        }`}>
          {bundles.map((bundle) => (
            <div
              key={bundle.id}
              className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 hover:border-dark-700 transition-all"
            >
              <div className="mb-4">
                <h3 className="text-lg font-bold text-dark-50">{bundle.name}</h3>
              </div>

              <div className="flex items-baseline gap-2 mb-2">
                <Zap className="w-5 h-5 text-primary-400" />
                <span className="text-3xl font-bold text-dark-50">
                  {bundle.credits.toLocaleString()}
                </span>
                <span className="text-dark-400 text-sm">credits</span>
              </div>

              <div className="mb-6">
                <span className="text-2xl font-semibold text-primary-400">
                  {formatPrice(bundle.priceCents)}
                </span>
                <span className="text-dark-500 text-sm ml-1">
                  ({formatPrice(Math.round(bundle.priceCents / bundle.credits * 100))}/100 credits)
                </span>
              </div>

              <button
                onClick={() => handleBuy(bundle.id)}
                disabled={checkoutLoading !== null}
                className="w-full py-2.5 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-60 flex items-center justify-center gap-2"
              >
                {checkoutLoading === bundle.id ? (
                  <LoadingSpinner size="sm" />
                ) : (
                  <>
                    <ShoppingCart className="w-4 h-4" />
                    Buy Now
                  </>
                )}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
