import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Save, Building2, Zap, Users, CreditCard, XCircle, AlertTriangle } from 'lucide-react';
import { adminApi } from '../../api/client';
import { useTenant } from '../../contexts/TenantContext';
import type { TenantDetail, TenantMember, Plan } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';

function formatPrice(cents: number): string {
  if (cents === 0) return 'Free';
  return `$${(cents / 100).toFixed(2)}`;
}

export default function TenantProfilePage() {
  const { tenantId } = useParams<{ tenantId: string }>();
  const navigate = useNavigate();
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';
  const isOwner = role === 'owner';

  const [tenant, setTenant] = useState<TenantDetail | null>(null);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');

  // Edit fields
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [saveSuccess, setSaveSuccess] = useState('');

  // Plan & billing fields
  const [plans, setPlans] = useState<Plan[]>([]);
  const [selectedPlanId, setSelectedPlanId] = useState<string>('');
  const [billingWaived, setBillingWaived] = useState(false);
  const [savingPlan, setSavingPlan] = useState(false);
  const [planError, setPlanError] = useState('');
  const [planSuccess, setPlanSuccess] = useState('');

  // Credit fields
  const [subscriptionCredits, setSubscriptionCredits] = useState(0);
  const [purchasedCredits, setPurchasedCredits] = useState(0);
  const [savingCredits, setSavingCredits] = useState(false);
  const [creditError, setCreditError] = useState('');
  const [creditSuccess, setCreditSuccess] = useState('');

  // Billing
  const [cancellingSubscription, setCancellingSubscription] = useState(false);
  const [showCancelModal, setShowCancelModal] = useState(false);

  const fetchTenant = useCallback(async () => {
    if (!tenantId) return;
    setLoading(true);
    try {
      const [tenantData, plansData] = await Promise.all([
        adminApi.getTenant(tenantId),
        adminApi.listPlans(),
      ]);
      setTenant(tenantData.tenant);
      setMembers(tenantData.members || []);
      setName(tenantData.tenant.name);
      setBillingWaived(tenantData.tenant.billingWaived);
      setSelectedPlanId(tenantData.tenant.planId || '');
      setSubscriptionCredits(tenantData.tenant.subscriptionCredits);
      setPurchasedCredits(tenantData.tenant.purchasedCredits);
      setPlans(plansData.plans || []);
    } catch {
      setFetchError('Failed to load tenant');
    } finally {
      setLoading(false);
    }
  }, [tenantId]);

  useEffect(() => { fetchTenant(); }, [fetchTenant]);

  const handleSave = async () => {
    if (!tenant) return;
    setSaving(true);
    setSaveError('');
    setSaveSuccess('');
    try {
      const updates: { name?: string } = {};
      if (name.trim() !== tenant.name) updates.name = name.trim();
      if (Object.keys(updates).length === 0) {
        setSaveSuccess('No changes to save');
        setSaving(false);
        return;
      }
      await adminApi.updateTenant(tenant.id, updates);
      setSaveSuccess('Tenant updated successfully');
      await fetchTenant();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to update tenant';
      setSaveError(msg);
    } finally {
      setSaving(false);
    }
  };

  const handleSavePlan = async () => {
    if (!tenant) return;
    const planChanged = selectedPlanId !== (tenant.planId || '');
    const waivedChanged = billingWaived !== tenant.billingWaived;
    if (!planChanged && !waivedChanged) {
      setPlanSuccess('No changes to save');
      return;
    }
    setSavingPlan(true);
    setPlanError('');
    setPlanSuccess('');
    try {
      await adminApi.assignTenantPlan(
        tenant.id,
        planChanged ? (selectedPlanId || null) : undefined as unknown as string | null,
        waivedChanged ? billingWaived : undefined,
      );
      setPlanSuccess('Plan updated successfully');
      await fetchTenant();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to update plan';
      setPlanError(msg);
    } finally {
      setSavingPlan(false);
    }
  };

  const handleSaveCredits = async () => {
    if (!tenant) return;
    setSavingCredits(true);
    setCreditError('');
    setCreditSuccess('');
    try {
      const updates: { subscriptionCredits?: number; purchasedCredits?: number } = {};
      if (subscriptionCredits !== tenant.subscriptionCredits) updates.subscriptionCredits = subscriptionCredits;
      if (purchasedCredits !== tenant.purchasedCredits) updates.purchasedCredits = purchasedCredits;
      if (Object.keys(updates).length === 0) {
        setCreditSuccess('No changes to save');
        setSavingCredits(false);
        return;
      }
      await adminApi.updateTenant(tenant.id, updates);
      setCreditSuccess('Credits updated successfully');
      await fetchTenant();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to update credits';
      setCreditError(msg);
    } finally {
      setSavingCredits(false);
    }
  };

  const handleToggleStatus = async () => {
    if (!tenant) return;
    try {
      await adminApi.updateTenantStatus(tenant.id, !tenant.isActive);
      await fetchTenant();
    } catch {
      // ignore
    }
  };

  const handleCancelSubscription = async (immediate: boolean) => {
    if (!tenant) return;
    setCancellingSubscription(true);
    try {
      await adminApi.adminCancelSubscription(tenant.id, immediate);
      setShowCancelModal(false);
      await fetchTenant();
    } catch {
      // ignore
    } finally {
      setCancellingSubscription(false);
    }
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  if (fetchError || !tenant) {
    return (
      <div className="text-center py-20">
        <p className="text-red-400 mb-4">{fetchError || 'Tenant not found'}</p>
        <button onClick={() => navigate('/last/tenants')} className="text-primary-400 hover:underline">
          Back to Tenants
        </button>
      </div>
    );
  }

  // Derive warnings for plan & billing section
  const selectedPlan = plans.find(p => p.id === selectedPlanId);
  const systemPlan = plans.find(p => p.isSystem);
  const currentPlanName = plans.find(p => p.id === tenant.planId)?.name || systemPlan?.name || 'Free';
  const selectedIsPaid = selectedPlan ? selectedPlan.monthlyPriceCents > 0 : false;
  const hasActiveSubscription = !!tenant.stripeSubscriptionId && (tenant.billingStatus === 'active' || tenant.billingStatus === 'past_due');

  // Warning: waiving billing while they have an active subscription
  const showWaiveWarning = billingWaived && !tenant.billingWaived && hasActiveSubscription;
  // Warning: removing waiver on a paid plan with no subscription
  const showUnwaiveWarning = !billingWaived && tenant.billingWaived && selectedIsPaid && !hasActiveSubscription;
  // Warning: assigning paid plan without waiver and no subscription
  const showPaidNoWaiverWarning = selectedIsPaid && !billingWaived && !hasActiveSubscription;

  return (
    <div>
      {/* Header */}
      <div className="mb-8">
        <Link to="/last/tenants" className="text-dark-400 hover:text-white text-sm flex items-center gap-1 mb-4">
          <ArrowLeft className="w-4 h-4" /> Back to Tenants
        </Link>
        <div className="flex items-center gap-3">
          <Building2 className="w-7 h-7 text-primary-400" />
          <div>
            <h1 className="text-2xl font-bold text-white">Tenant Profile</h1>
            <p className="text-dark-400 text-sm">{tenant.name} &middot; {tenant.slug}</p>
          </div>
          {tenant.isRoot && (
            <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-amber-500/20 text-amber-400 rounded">Root</span>
          )}
        </div>
      </div>

      {/* Tenant Information */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-4">Tenant Information</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm text-dark-400 mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          </div>
          <div>
            <label className="block text-sm text-dark-400 mb-1">Slug</label>
            <div className="px-3 py-2 bg-dark-800/50 border border-dark-700 rounded-lg text-dark-400 text-sm font-mono">
              {tenant.slug}
            </div>
          </div>
        </div>
        {canWrite && (
          <div className="flex items-center gap-3">
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-60 flex items-center gap-2"
            >
              <Save className="w-4 h-4" />
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
            {saveError && <span className="text-red-400 text-sm">{saveError}</span>}
            {saveSuccess && <span className="text-green-400 text-sm">{saveSuccess}</span>}
          </div>
        )}
      </div>

      {/* Plan & Billing */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <CreditCard className="w-5 h-5 text-dark-400" />
          Plan &amp; Billing
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm text-dark-400 mb-1">Plan</label>
            <select
              value={selectedPlanId}
              onChange={(e) => setSelectedPlanId(e.target.value)}
              disabled={!isOwner}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            >
              <option value="">{systemPlan ? `${systemPlan.name} (Default)` : 'System Default'}</option>
              {plans.filter(p => !p.isSystem).map(p => (
                <option key={p.id} value={p.id}>
                  {p.name}{p.isArchived ? ' (Archived)' : ''} — {formatPrice(p.monthlyPriceCents)}/mo
                </option>
              ))}
            </select>
            <p className="text-xs text-dark-500 mt-1">Currently: {currentPlanName}</p>
          </div>
          <div>
            <label className="block text-sm text-dark-400 mb-1">Billing Waived</label>
            <button
              onClick={() => setBillingWaived(!billingWaived)}
              disabled={!isOwner}
              className={`px-3 py-2 text-sm font-medium rounded-lg border transition-colors disabled:opacity-60 disabled:cursor-not-allowed ${
                billingWaived
                  ? 'bg-green-500/20 text-green-400 border-green-500/30'
                  : 'bg-dark-800 text-dark-400 border-dark-700'
              }`}
            >
              {billingWaived ? 'Yes' : 'No'}
            </button>
            <p className="text-xs text-dark-500 mt-1">
              {billingWaived ? 'Tenant uses paid features without being charged' : 'Tenant must pay via Stripe for paid plans'}
            </p>
          </div>
        </div>

        {/* Contextual warnings */}
        {showWaiveWarning && (
          <div className="mb-4 bg-amber-500/10 border border-amber-500/20 rounded-lg p-3 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
            <p className="text-sm text-amber-300">
              This tenant has an active Stripe subscription. Waiving billing will <strong>cancel their subscription immediately</strong> and they will no longer be charged.
            </p>
          </div>
        )}
        {showUnwaiveWarning && (
          <div className="mb-4 bg-amber-500/10 border border-amber-500/20 rounded-lg p-3 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
            <p className="text-sm text-amber-300">
              This tenant is on a paid plan with no Stripe subscription. Removing the billing waiver will <strong>downgrade them to {systemPlan?.name || 'the default plan'}</strong>. They can then subscribe to a paid plan through the normal checkout flow.
            </p>
          </div>
        )}
        {showPaidNoWaiverWarning && !showUnwaiveWarning && (
          <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
            <p className="text-sm text-red-300">
              You're assigning a paid plan without waiving billing. This tenant has no active subscription to cover the cost. Either <strong>enable billing waived</strong> or let the tenant subscribe through the checkout flow.
            </p>
          </div>
        )}

        {isOwner && (
          <div className="flex items-center gap-3">
            <button
              onClick={handleSavePlan}
              disabled={savingPlan}
              className="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-60 flex items-center gap-2"
            >
              <Save className="w-4 h-4" />
              {savingPlan ? 'Saving...' : 'Save Plan'}
            </button>
            {planError && <span className="text-red-400 text-sm">{planError}</span>}
            {planSuccess && <span className="text-green-400 text-sm">{planSuccess}</span>}
          </div>
        )}
      </div>

      {/* Usage Credits */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-1 flex items-center gap-2">
          <Zap className="w-5 h-5 text-primary-400" />
          Usage Credits
        </h2>
        <p className="text-dark-400 text-sm mb-4">
          Total balance: <span className="text-white font-semibold">{(subscriptionCredits + purchasedCredits).toLocaleString()}</span> credits
        </p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm text-dark-400 mb-1">Subscription Credits</label>
            <p className="text-xs text-dark-500 mb-1">From monthly plan allotment (resets monthly if policy is &ldquo;reset&rdquo;)</p>
            <input
              type="text"
              inputMode="numeric"
              value={subscriptionCredits}
              onChange={(e) => setSubscriptionCredits(parseInt(e.target.value) || 0)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          </div>
          <div>
            <label className="block text-sm text-dark-400 mb-1">Purchased &amp; Bonus Credits</label>
            <p className="text-xs text-dark-500 mb-1">From one-time purchases and bonuses (never reset)</p>
            <input
              type="text"
              inputMode="numeric"
              value={purchasedCredits}
              onChange={(e) => setPurchasedCredits(parseInt(e.target.value) || 0)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          </div>
        </div>
        {canWrite && (
          <div className="flex items-center gap-3">
            <button
              onClick={handleSaveCredits}
              disabled={savingCredits}
              className="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-60 flex items-center gap-2"
            >
              <Save className="w-4 h-4" />
              {savingCredits ? 'Saving...' : 'Save Credits'}
            </button>
            {creditError && <span className="text-red-400 text-sm">{creditError}</span>}
            {creditSuccess && <span className="text-green-400 text-sm">{creditSuccess}</span>}
          </div>
        )}
      </div>

      {/* Account Status */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-4">Account Status</h2>
        <div className="flex items-center gap-4">
          <span className={`px-2 py-1 text-xs font-medium rounded ${
            tenant.isActive ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
          }`}>
            {tenant.isActive ? 'Active' : 'Disabled'}
          </span>
          {!tenant.isRoot && canWrite && (
            <button
              onClick={handleToggleStatus}
              className={`px-3 py-1.5 text-sm font-medium rounded-lg border transition-colors ${
                tenant.isActive
                  ? 'border-red-500/30 text-red-400 hover:bg-red-500/10'
                  : 'border-green-500/30 text-green-400 hover:bg-green-500/10'
              }`}
            >
              {tenant.isActive ? 'Disable Tenant' : 'Enable Tenant'}
            </button>
          )}
        </div>
      </div>

      {/* Billing Info */}
      {tenant.billingStatus && tenant.billingStatus !== 'none' && (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
          <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <CreditCard className="w-5 h-5 text-dark-400" />
            Stripe Subscription
          </h2>
          <div className="space-y-3 mb-4">
            <div className="flex items-center justify-between py-2">
              <span className="text-sm text-dark-400">Status</span>
              <span className={`text-sm font-medium ${
                tenant.billingStatus === 'active' ? 'text-accent-emerald' :
                tenant.billingStatus === 'past_due' ? 'text-red-400' :
                tenant.billingStatus === 'canceled' ? 'text-yellow-400' :
                'text-dark-400'
              }`}>
                {tenant.billingStatus === 'active' ? 'Active' :
                 tenant.billingStatus === 'past_due' ? 'Past Due' :
                 tenant.billingStatus === 'canceled' ? 'Canceled' :
                 tenant.billingStatus}
              </span>
            </div>
            {tenant.stripeSubscriptionId && (
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Subscription ID</span>
                <span className="text-sm text-dark-300 font-mono">{tenant.stripeSubscriptionId}</span>
              </div>
            )}
            {tenant.billingInterval && (
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Billing Interval</span>
                <span className="text-sm text-white capitalize">{tenant.billingInterval}ly</span>
              </div>
            )}
            {tenant.currentPeriodEnd && (
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Period End</span>
                <span className="text-sm text-white">{new Date(tenant.currentPeriodEnd).toLocaleDateString()}</span>
              </div>
            )}
            {tenant.canceledAt && (
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Canceled At</span>
                <span className="text-sm text-yellow-400">{new Date(tenant.canceledAt).toLocaleDateString()}</span>
              </div>
            )}
          </div>
          {isOwner && tenant.stripeSubscriptionId && tenant.billingStatus === 'active' && (
            <button
              onClick={() => setShowCancelModal(true)}
              className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors"
            >
              <XCircle className="w-4 h-4" />
              Cancel Subscription
            </button>
          )}
        </div>
      )}

      {/* Cancel Subscription Modal */}
      {showCancelModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full">
            <h3 className="text-lg font-semibold text-white mb-4">Cancel Subscription</h3>
            <p className="text-dark-300 mb-6">Choose how to cancel this tenant's subscription:</p>
            <div className="space-y-3">
              <button
                onClick={() => handleCancelSubscription(false)}
                disabled={cancellingSubscription}
                className="w-full px-4 py-2.5 text-sm font-medium bg-yellow-500/20 text-yellow-400 border border-yellow-500/30 rounded-lg hover:bg-yellow-500/30 transition-colors disabled:opacity-60"
              >
                Cancel at Period End
              </button>
              <button
                onClick={() => handleCancelSubscription(true)}
                disabled={cancellingSubscription}
                className="w-full px-4 py-2.5 text-sm font-medium bg-red-500/20 text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/30 transition-colors disabled:opacity-60"
              >
                Cancel Immediately
              </button>
              <button
                onClick={() => setShowCancelModal(false)}
                className="w-full px-4 py-2 text-sm text-dark-400 hover:text-white transition-colors"
              >
                Keep Subscription
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Members */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Users className="w-5 h-5 text-dark-400" />
          Members
          <span className="text-sm font-normal text-dark-500">({members.length})</span>
        </h2>
        {members.length === 0 ? (
          <p className="text-dark-400 text-sm">No members in this tenant.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-dark-800 text-dark-400 text-left">
                  <th className="pb-2 font-medium">Name</th>
                  <th className="pb-2 font-medium">Email</th>
                  <th className="pb-2 font-medium">Role</th>
                  <th className="pb-2 font-medium">Joined</th>
                </tr>
              </thead>
              <tbody>
                {members.map((m) => (
                  <tr
                    key={m.userId}
                    onClick={() => navigate(`/last/users/${m.userId}`)}
                    className="border-b border-dark-800/50 hover:bg-dark-800/30 cursor-pointer transition-colors"
                  >
                    <td className="py-3 text-white">{m.displayName}</td>
                    <td className="py-3 text-dark-300">{m.email}</td>
                    <td className="py-3">
                      <span className={`px-2 py-0.5 text-xs font-medium rounded ${
                        m.role === 'owner' ? 'bg-amber-500/20 text-amber-400' :
                        m.role === 'admin' ? 'bg-blue-500/20 text-blue-400' :
                        'bg-dark-700 text-dark-300'
                      }`}>
                        {m.role.charAt(0).toUpperCase() + m.role.slice(1)}
                      </span>
                    </td>
                    <td className="py-3 text-dark-400">{new Date(m.joinedAt).toLocaleDateString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
