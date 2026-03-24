import { useEffect, useState, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { CreditCard, Check, Minus, Crown, Sparkles, Zap, XCircle, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
import { plansApi, billingApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import { useTenant } from '../../contexts/TenantContext';
import { useTelemetry } from '../../hooks/useTelemetry';
import type { Plan, EntitlementValue, BillingStatus } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';

const currencySymbols: Record<string, string> = {
  usd: '$', eur: '€', gbp: '£', jpy: '¥', cad: 'CA$', aud: 'A$',
};

function getCurrencySymbol(currency: string): string {
  return currencySymbols[currency?.toLowerCase()] || currency?.toUpperCase() + ' ';
}

function formatPrice(cents: number, currency = 'usd'): string {
  if (cents === 0) return 'Free';
  return `${getCurrencySymbol(currency)}${(cents / 100).toFixed(2)}`;
}

function annualPrice(cents: number, discountPct: number, currency = 'usd'): string {
  const monthly = (cents / 100) * (1 - discountPct / 100);
  return `${getCurrencySymbol(currency)}${monthly.toFixed(2)}`;
}

function annualTotal(cents: number, discountPct: number): number {
  const annual = cents * 12;
  return Math.round(annual * (1 - discountPct / 100));
}

export default function PlanPage() {
  const { activeTenant } = useTenant();
  const { trackPageView } = useTelemetry();
  useEffect(() => { trackPageView('/plan'); }, [trackPageView]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [currentPlanId, setCurrentPlanId] = useState('');
  const [billingWaived, setBillingWaived] = useState(false);
  const [billingStatus, setBillingStatus] = useState<BillingStatus>('none');
  const [billingInterval, setBillingInterval] = useState<string>('');
  const [currentPeriodEnd, setCurrentPeriodEnd] = useState<string>('');
  const [, setCanceledAt] = useState<string>('');
  const [subscriptionCredits, setSubscriptionCredits] = useState(0);
  const [purchasedCredits, setPurchasedCredits] = useState(0);
  const [maxPlanUserLimit, setMaxPlanUserLimit] = useState(0);
  const [currency, setCurrency] = useState('usd');
  const [loading, setLoading] = useState(true);
  const [checkoutLoading, setCheckoutLoading] = useState<string | null>(null);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [showCancelModal, setShowCancelModal] = useState(false);
  const [showWaiverModal, setShowWaiverModal] = useState(false);
  const [pendingWaiverPlan, setPendingWaiverPlan] = useState<Plan | null>(null);
  const [selectedInterval, setSelectedInterval] = useState<'month' | 'year'>('year');
  const [searchParams] = useSearchParams();
  const upgradePlanId = searchParams.get('upgrade');
  const [highlightId, setHighlightId] = useState<string | null>(null);
  const cardRefs = useRef<Record<string, HTMLDivElement | null>>({});

  useEffect(() => {
    if (!activeTenant) return;
    setLoading(true);
    plansApi.list()
      .then((data) => {
        setPlans(data.plans);
        setCurrentPlanId(data.currentPlanId);
        setBillingWaived(data.billingWaived);
        setBillingStatus(data.billingStatus || 'none');
        setBillingInterval(data.billingInterval || '');
        setCurrentPeriodEnd(data.currentPeriodEnd || '');
        setCanceledAt(data.canceledAt || '');
        setSubscriptionCredits(data.tenantSubscriptionCredits);
        setPurchasedCredits(data.tenantPurchasedCredits);
        setMaxPlanUserLimit(data.maxPlanUserLimit);
        if (data.currency) setCurrency(data.currency);
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, [activeTenant]);

  useEffect(() => {
    if (!loading && upgradePlanId && cardRefs.current[upgradePlanId]) {
      setHighlightId(upgradePlanId);
      cardRefs.current[upgradePlanId]?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      const timer = setTimeout(() => setHighlightId(null), 3000);
      return () => clearTimeout(timer);
    }
  }, [loading, upgradePlanId]);

  const handleCheckout = async (plan: Plan) => {
    if (plan.id === currentPlanId) return;

    // If billing is waived and this is a paid plan, show confirmation modal
    if (billingWaived && (plan.monthlyPriceCents > 0 || plan.perSeatPriceCents > 0)) {
      setPendingWaiverPlan(plan);
      setShowWaiverModal(true);
      return;
    }

    await doCheckout(plan, false);
  };

  const doCheckout = async (plan: Plan, removeBillingWaiver: boolean) => {
    setCheckoutLoading(plan.id);
    try {
      const result = await billingApi.checkout({
        planId: plan.id,
        billingInterval: selectedInterval,
        removeBillingWaiver,
      });
      if (result.waived) {
        window.location.reload();
      } else if (result.checkoutUrl) {
        window.location.href = result.checkoutUrl;
      }
    } catch {
      // Error handled by interceptor
    } finally {
      setCheckoutLoading(null);
    }
  };

  const handleWaiverConfirm = async () => {
    if (!pendingWaiverPlan) return;
    setShowWaiverModal(false);
    await doCheckout(pendingWaiverPlan, true);
    setPendingWaiverPlan(null);
  };

  const handleCancel = async () => {
    setCancelLoading(true);
    try {
      await billingApi.cancel();
      window.location.reload();
    } catch {
      // Error handled by interceptor
    } finally {
      setCancelLoading(false);
      setShowCancelModal(false);
    }
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  const currentPlan = plans.find(p => p.id === currentPlanId);
  const hasCredits = plans.some(p => p.usageCreditsPerMonth > 0);
  const hasBonusCredits = plans.some(p => p.bonusCredits > 0);
  const hasAnnual = plans.some(p => p.annualDiscountPct > 0);
  const showUserLimits = maxPlanUserLimit !== 1;

  // Collect all unique entitlement keys with descriptions
  const entitlementKeys: { key: string; description: string }[] = [];
  const seenKeys = new Set<string>();
  for (const plan of plans) {
    for (const [key, val] of Object.entries(plan.entitlements || {})) {
      if (!seenKeys.has(key)) {
        seenKeys.add(key);
        entitlementKeys.push({ key, description: val.description || key });
      }
    }
  }

  // Sort plans by price for display
  const sortedPlans = [...plans].sort((a, b) => a.monthlyPriceCents - b.monthlyPriceCents);
  const currentPlanIndex = sortedPlans.findIndex(p => p.id === currentPlanId);
  const isActiveSubscription = billingStatus === 'active' || billingStatus === 'canceled';
  const isCanceled = billingStatus === 'canceled';

  return (
    <div>
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
          <CreditCard className="w-7 h-7 text-primary-400" />
          Your Plan
        </h1>
        <p className="text-dark-400 mt-1">
          Manage your subscription and compare available plans
        </p>
      </div>

      {/* Billing Interval Toggle */}
      {hasAnnual && !isActiveSubscription && (
        <div className="flex items-center justify-center gap-3 mb-6">
          <button
            onClick={() => setSelectedInterval('month')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              selectedInterval === 'month'
                ? 'bg-primary-500 text-white'
                : 'bg-dark-800 text-dark-400 hover:text-dark-300'
            }`}
          >
            Monthly
          </button>
          <button
            onClick={() => setSelectedInterval('year')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              selectedInterval === 'year'
                ? 'bg-primary-500 text-white'
                : 'bg-dark-800 text-dark-400 hover:text-dark-300'
            }`}
          >
            Annual
            <span className="ml-1 text-xs opacity-75">Save up to {Math.max(...plans.map(p => p.annualDiscountPct))}%</span>
          </button>
        </div>
      )}

      {/* Current Plan Banner */}
      {currentPlan && (
        <div className="bg-gradient-to-r from-primary-500/10 via-accent-purple/10 to-primary-500/10 border border-primary-500/20 rounded-2xl p-6 mb-8">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <Crown className="w-5 h-5 text-primary-400" />
                <span className="text-sm font-medium text-primary-400">Current Plan</span>
                {billingWaived && (
                  <span className="px-2 py-0.5 bg-accent-emerald/10 text-accent-emerald text-xs font-medium rounded-full">
                    Billing Waived
                  </span>
                )}
                {billingStatus === 'active' && (
                  <span className="px-2 py-0.5 bg-accent-emerald/10 text-accent-emerald text-xs font-medium rounded-full">
                    Active
                  </span>
                )}
                {billingStatus === 'past_due' && (
                  <span className="px-2 py-0.5 bg-red-500/10 text-red-400 text-xs font-medium rounded-full">
                    Past Due
                  </span>
                )}
                {isCanceled && (
                  <span className="px-2 py-0.5 bg-yellow-500/10 text-yellow-400 text-xs font-medium rounded-full">
                    Canceled
                  </span>
                )}
              </div>
              <h2 className="text-2xl font-bold text-dark-50">{currentPlan.name}</h2>
              {currentPlan.description && (
                <p className="text-dark-300 mt-1">{currentPlan.description}</p>
              )}
              {billingInterval && (
                <p className="text-dark-400 text-sm mt-1">
                  Billed {billingInterval}ly
                </p>
              )}
              {currentPeriodEnd && isActiveSubscription && (
                <p className="text-dark-400 text-sm mt-1">
                  {isCanceled
                    ? `Benefits until ${new Date(currentPeriodEnd).toLocaleDateString()}`
                    : `Next billing: ${new Date(currentPeriodEnd).toLocaleDateString()}`
                  }
                </p>
              )}
              {(hasCredits || hasBonusCredits) && (
                <div className="mt-3 space-y-1">
                  <div className="flex items-center gap-2">
                    <Zap className="w-4 h-4 text-primary-400" />
                    <span className="text-sm text-dark-300">
                      <span className="text-dark-50 font-semibold">{(subscriptionCredits + purchasedCredits).toLocaleString()}</span>
                      {' '}credits total
                    </span>
                  </div>
                  <div className="flex items-center gap-4 text-xs text-dark-400 ml-6">
                    <span>{subscriptionCredits.toLocaleString()} from monthly plan</span>
                    <span>{purchasedCredits.toLocaleString()} from purchases &amp; bonuses</span>
                  </div>
                </div>
              )}
            </div>
            <div className="text-right">
              <div className="text-3xl font-bold text-dark-50">
                {formatPrice(currentPlan.monthlyPriceCents, currency)}
              </div>
              {currentPlan.monthlyPriceCents > 0 && (
                <span className="text-dark-400 text-sm">/month</span>
              )}
            </div>
          </div>

          {/* Cancel button */}
          {billingStatus === 'active' && currentPlan.monthlyPriceCents > 0 && !billingWaived && (
            <div className="mt-4 pt-4 border-t border-primary-500/10">
              <button
                onClick={() => setShowCancelModal(true)}
                className="text-sm text-dark-400 hover:text-red-400 transition-colors"
              >
                Cancel Subscription
              </button>
            </div>
          )}
        </div>
      )}

      {/* Past due warning */}
      {billingStatus === 'past_due' && (
        <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-4 mb-6 flex items-start gap-3">
          <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
          <div>
            <p className="text-red-400 font-medium">Payment Failed</p>
            <p className="text-dark-400 text-sm mt-1">
              Your most recent payment was unsuccessful. Please update your billing information in Settings to avoid service interruption.
            </p>
          </div>
        </div>
      )}

      {/* Plan Cards */}
      <div className={`grid gap-6 mb-10 ${
        sortedPlans.length <= 3 ? `grid-cols-1 md:grid-cols-${sortedPlans.length}` : 'grid-cols-1 md:grid-cols-2 lg:grid-cols-3'
      }`}>
        {sortedPlans.map((plan, idx) => {
          const isCurrent = plan.id === currentPlanId;
          const isUpgrade = idx > currentPlanIndex;
          const isPopular = idx === 1 && sortedPlans.length >= 3;
          const displayPrice = selectedInterval === 'year' && plan.annualDiscountPct > 0
            ? annualTotal(plan.monthlyPriceCents, plan.annualDiscountPct)
            : plan.monthlyPriceCents;

          const isHighlighted = highlightId === plan.id;

          return (
            <div
              key={plan.id}
              ref={(el) => { cardRefs.current[plan.id] = el; }}
              className={`relative rounded-2xl border p-6 transition-all ${
                isCurrent
                  ? 'bg-primary-500/5 border-primary-500/30 ring-1 ring-primary-500/20'
                  : isHighlighted
                    ? 'bg-primary-500/10 border-primary-400 ring-2 ring-primary-500 animate-pulse'
                    : 'bg-dark-900/50 border-dark-800 hover:border-dark-700'
              }`}
            >
              {isCurrent && (
                <div className="absolute -top-3 left-6 px-3 py-0.5 bg-primary-500 text-white text-xs font-medium rounded-full">
                  Current Plan
                </div>
              )}
              {isPopular && !isCurrent && (
                <div className="absolute -top-3 left-6 px-3 py-0.5 bg-accent-purple text-white text-xs font-medium rounded-full flex items-center gap-1">
                  <Sparkles className="w-3 h-3" /> Popular
                </div>
              )}

              <div className="mb-4 pt-2">
                <h3 className="text-lg font-bold text-dark-50">{plan.name}</h3>
                {plan.description && (
                  <p className="text-dark-400 text-sm mt-1">{plan.description}</p>
                )}
              </div>

              <div className="mb-6">
                {selectedInterval === 'year' && plan.annualDiscountPct > 0 ? (
                  <>
                    <div className="flex items-baseline gap-1">
                      <span className="text-3xl font-bold text-dark-50">{annualPrice(plan.monthlyPriceCents, plan.annualDiscountPct, currency)}</span>
                      <span className="text-dark-400 text-sm">/mo</span>
                    </div>
                    <p className="text-sm text-accent-emerald mt-1">
                      {formatPrice(displayPrice, currency)}/year ({plan.annualDiscountPct}% off)
                    </p>
                  </>
                ) : (
                  <>
                    <div className="flex items-baseline gap-1">
                      <span className="text-3xl font-bold text-dark-50">{formatPrice(plan.monthlyPriceCents, currency)}</span>
                      {plan.monthlyPriceCents > 0 && <span className="text-dark-400 text-sm">/mo</span>}
                    </div>
                    {plan.annualDiscountPct > 0 && (
                      <p className="text-sm text-accent-emerald mt-1">
                        {annualPrice(plan.monthlyPriceCents, plan.annualDiscountPct, currency)}/mo billed annually ({plan.annualDiscountPct}% off)
                      </p>
                    )}
                  </>
                )}
              </div>

              {/* Trial badge */}
              {plan.trialDays > 0 && !isCurrent && (
                <div className="mb-4 px-3 py-1.5 bg-accent-emerald/10 border border-accent-emerald/20 rounded-lg text-center">
                  <span className="text-sm font-medium text-accent-emerald">{plan.trialDays}-day free trial</span>
                </div>
              )}

              {/* Key features list */}
              <div className="space-y-3 mb-6">
                {showUserLimits && (
                  <div className="flex items-center gap-2 text-sm">
                    <Check className="w-4 h-4 text-accent-emerald flex-shrink-0" />
                    <span className="text-dark-300">
                      {plan.userLimit === 0 ? 'Unlimited users' : `Up to ${plan.userLimit} user${plan.userLimit > 1 ? 's' : ''}`}
                    </span>
                  </div>
                )}
                {hasCredits && plan.usageCreditsPerMonth > 0 && (
                  <div className="flex items-center gap-2 text-sm">
                    <Check className="w-4 h-4 text-accent-emerald flex-shrink-0" />
                    <span className="text-dark-300">{plan.usageCreditsPerMonth.toLocaleString()} credits/month</span>
                  </div>
                )}
                {hasBonusCredits && plan.bonusCredits > 0 && (
                  <div className="flex items-center gap-2 text-sm">
                    <Check className="w-4 h-4 text-accent-emerald flex-shrink-0" />
                    <span className="text-dark-300">{plan.bonusCredits.toLocaleString()} bonus credits</span>
                  </div>
                )}
                {entitlementKeys.map(({ key, description }) => {
                  const ent = plan.entitlements?.[key];
                  if (!ent) return null;
                  if (ent.type === 'bool' && !ent.boolValue) return null;
                  return (
                    <div key={key} className="flex items-center gap-2 text-sm">
                      <Check className="w-4 h-4 text-accent-emerald flex-shrink-0" />
                      <span className="text-dark-300">
                        {ent.type === 'bool' ? description : `${ent.numericValue} ${description}`}
                      </span>
                    </div>
                  );
                })}
              </div>

              {/* CTA Button */}
              {isCurrent ? (
                <div className="w-full py-2.5 text-center text-sm font-medium text-primary-400 bg-primary-500/10 rounded-lg border border-primary-500/20">
                  Your Plan
                </div>
              ) : (
                <button
                  onClick={() => handleCheckout(plan)}
                  disabled={checkoutLoading !== null}
                  className={`w-full py-2.5 text-sm font-medium rounded-lg transition-colors ${
                    isUpgrade
                      ? 'bg-primary-500 text-white hover:bg-primary-600 disabled:opacity-60'
                      : 'bg-dark-800 text-dark-200 border border-dark-700 hover:border-dark-600 disabled:opacity-60 disabled:text-dark-400'
                  }`}
                >
                  {checkoutLoading === plan.id ? (
                    <LoadingSpinner size="sm" />
                  ) : (
                    isUpgrade ? 'Upgrade' : 'Switch Plan'
                  )}
                </button>
              )}
            </div>
          );
        })}
      </div>

      {/* Comparison Table */}
      <div className="bg-dark-900/50 rounded-2xl border border-dark-800 overflow-hidden">
        <div className="px-6 py-4 border-b border-dark-800">
          <h3 className="text-lg font-semibold text-dark-50">Plan Comparison</h3>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-dark-800">
                <th className="text-left px-6 py-3 text-sm font-medium text-dark-400 min-w-[200px]">Feature</th>
                {sortedPlans.map(plan => (
                  <th key={plan.id} className={`text-center px-6 py-3 text-sm font-medium min-w-[140px] ${
                    plan.id === currentPlanId ? 'text-primary-400' : 'text-dark-400'
                  }`}>
                    {plan.name}
                    {plan.id === currentPlanId && (
                      <span className="block text-xs text-primary-400/60 font-normal mt-0.5">Current</span>
                    )}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-dark-800/50">
              {/* Price Row */}
              <tr className="hover:bg-dark-800/20">
                <td className="px-6 py-3 text-sm text-dark-300">Monthly Price</td>
                {sortedPlans.map(plan => (
                  <td key={plan.id} className={`px-6 py-3 text-sm text-center ${plan.id === currentPlanId ? 'text-dark-50 font-medium' : 'text-dark-300'}`}>
                    {formatPrice(plan.monthlyPriceCents, currency)}{plan.monthlyPriceCents > 0 ? '/mo' : ''}
                  </td>
                ))}
              </tr>

              {/* Annual Price Row */}
              {hasAnnual && (
                <tr className="hover:bg-dark-800/20">
                  <td className="px-6 py-3 text-sm text-dark-300">Annual Price</td>
                  {sortedPlans.map(plan => (
                    <td key={plan.id} className={`px-6 py-3 text-sm text-center ${plan.id === currentPlanId ? 'text-dark-50 font-medium' : 'text-dark-300'}`}>
                      {plan.annualDiscountPct > 0 ? (
                        <span>{annualPrice(plan.monthlyPriceCents, plan.annualDiscountPct, currency)}/mo <span className="text-accent-emerald text-xs">({plan.annualDiscountPct}% off)</span></span>
                      ) : (
                        <span className="text-dark-500">—</span>
                      )}
                    </td>
                  ))}
                </tr>
              )}

              {/* User Limit Row */}
              {showUserLimits && (
                <tr className="hover:bg-dark-800/20">
                  <td className="px-6 py-3 text-sm text-dark-300">Users</td>
                  {sortedPlans.map(plan => (
                    <td key={plan.id} className={`px-6 py-3 text-sm text-center ${plan.id === currentPlanId ? 'text-dark-50 font-medium' : 'text-dark-300'}`}>
                      {plan.userLimit === 0 ? 'Unlimited' : plan.userLimit}
                    </td>
                  ))}
                </tr>
              )}

              {/* Usage Credits Row */}
              {hasCredits && (
                <tr className="hover:bg-dark-800/20">
                  <td className="px-6 py-3 text-sm text-dark-300">Usage Credits / Month</td>
                  {sortedPlans.map(plan => (
                    <td key={plan.id} className={`px-6 py-3 text-sm text-center ${plan.id === currentPlanId ? 'text-dark-50 font-medium' : 'text-dark-300'}`}>
                      {plan.usageCreditsPerMonth > 0 ? plan.usageCreditsPerMonth.toLocaleString() : <span className="text-dark-500">—</span>}
                    </td>
                  ))}
                </tr>
              )}

              {/* Bonus Credits Row */}
              {hasBonusCredits && (
                <tr className="hover:bg-dark-800/20">
                  <td className="px-6 py-3 text-sm text-dark-300">Bonus Credits (one-time)</td>
                  {sortedPlans.map(plan => (
                    <td key={plan.id} className={`px-6 py-3 text-sm text-center ${plan.id === currentPlanId ? 'text-dark-50 font-medium' : 'text-dark-300'}`}>
                      {plan.bonusCredits > 0 ? plan.bonusCredits.toLocaleString() : <span className="text-dark-500">—</span>}
                    </td>
                  ))}
                </tr>
              )}

              {/* Entitlement Rows */}
              {entitlementKeys.map(({ key, description }) => (
                <tr key={key} className="hover:bg-dark-800/20">
                  <td className="px-6 py-3 text-sm text-dark-300">{description}</td>
                  {sortedPlans.map(plan => {
                    const ent: EntitlementValue | undefined = plan.entitlements?.[key];
                    return (
                      <td key={plan.id} className={`px-6 py-3 text-center ${plan.id === currentPlanId ? 'text-dark-50' : 'text-dark-300'}`}>
                        {!ent ? (
                          <Minus className="w-4 h-4 text-dark-600 mx-auto" />
                        ) : ent.type === 'bool' ? (
                          ent.boolValue ? (
                            <Check className="w-5 h-5 text-accent-emerald mx-auto" />
                          ) : (
                            <Minus className="w-4 h-4 text-dark-600 mx-auto" />
                          )
                        ) : (
                          <span className="text-sm font-medium">{ent.numericValue > 0 ? ent.numericValue.toLocaleString() : <Minus className="w-4 h-4 text-dark-600 mx-auto inline-block" />}</span>
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Billing Waiver Confirmation Modal */}
      {showWaiverModal && pendingWaiverPlan && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full">
            <div className="flex items-center gap-3 mb-4">
              <AlertTriangle className="w-6 h-6 text-yellow-400" />
              <h3 className="text-lg font-semibold text-dark-50">Billing Waiver Active</h3>
            </div>
            <p className="text-dark-300 mb-2">
              Your account currently has billing waived. Switching to <span className="text-dark-50 font-medium">{pendingWaiverPlan.name}</span> will start a paid subscription.
            </p>
            <p className="text-dark-400 text-sm mb-6">
              Your billing waiver will be removed and you'll be redirected to complete payment.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => { setShowWaiverModal(false); setPendingWaiverPlan(null); }}
                className="px-4 py-2 text-sm text-dark-300 hover:text-dark-50 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleWaiverConfirm}
                className="px-4 py-2 text-sm bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
              >
                Switch to Paid Plan
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Cancel Subscription Modal */}
      {showCancelModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full">
            <div className="flex items-center gap-3 mb-4">
              <XCircle className="w-6 h-6 text-red-400" />
              <h3 className="text-lg font-semibold text-dark-50">Cancel Subscription</h3>
            </div>
            <p className="text-dark-300 mb-2">
              Are you sure you want to cancel your subscription?
            </p>
            {currentPeriodEnd && (
              <p className="text-dark-400 text-sm mb-6">
                You'll keep your benefits until <span className="text-dark-50 font-medium">{new Date(currentPeriodEnd).toLocaleDateString()}</span>.
                After that, you'll be downgraded to the Free plan.
              </p>
            )}
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setShowCancelModal(false)}
                className="px-4 py-2 text-sm text-dark-300 hover:text-dark-50 transition-colors"
              >
                Keep Subscription
              </button>
              <button
                onClick={handleCancel}
                disabled={cancelLoading}
                className="px-4 py-2 text-sm bg-red-500 text-white rounded-lg hover:bg-red-600 disabled:opacity-60 transition-colors"
              >
                {cancelLoading ? <LoadingSpinner size="sm" /> : 'Cancel Subscription'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
