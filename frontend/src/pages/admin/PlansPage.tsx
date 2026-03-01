import { useEffect, useState, useCallback } from 'react';
import { CreditCard, Plus, Trash2, X, Shield, AlertTriangle, Zap, Archive, RotateCcw } from 'lucide-react';
import { toast } from 'sonner';
import { adminApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import type { Plan, EntitlementValue, EntitlementType, EntitlementKeyInfo, CreditBundle } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import ConfirmModal from '../../components/ConfirmModal';
import { useTenant } from '../../contexts/TenantContext';

function formatPrice(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function PlansPage() {
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';

  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [editPlan, setEditPlan] = useState<Plan | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Plan | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  // Credit Bundles state
  const [bundles, setBundles] = useState<CreditBundle[]>([]);
  const [editBundle, setEditBundle] = useState<CreditBundle | null>(null);
  const [showCreateBundle, setShowCreateBundle] = useState(false);
  const [deleteBundleTarget, setDeleteBundleTarget] = useState<CreditBundle | null>(null);
  const [deletingBundle, setDeletingBundle] = useState(false);
  const [deleteBundleError, setDeleteBundleError] = useState('');

  // Archive/unarchive state
  const [archiveTarget, setArchiveTarget] = useState<Plan | null>(null);
  const [archiveLoading, setArchiveLoading] = useState(false);

  const fetchPlans = useCallback(async () => {
    try {
      const data = await adminApi.listPlans();
      setPlans(data.plans);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchBundles = useCallback(async () => {
    try {
      const data = await adminApi.listBundles();
      setBundles(data.bundles);
    } catch (err) {
      toast.error(getErrorMessage(err));
    }
  }, []);

  useEffect(() => { fetchPlans(); fetchBundles(); }, [fetchPlans, fetchBundles]);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await adminApi.deletePlan(deleteTarget.id);
      setDeleteTarget(null);
      toast.success(`${deleteTarget.name} deleted`);
      fetchPlans();
    } catch (err: unknown) {
      setDeleteError(getErrorMessage(err));
    } finally {
      setDeleting(false);
    }
  };

  const handleArchiveConfirm = async () => {
    if (!archiveTarget) return;
    setArchiveLoading(true);
    try {
      if (archiveTarget.isArchived) {
        await adminApi.unarchivePlan(archiveTarget.id);
        toast.success(`${archiveTarget.name} unarchived`);
      } else {
        await adminApi.archivePlan(archiveTarget.id);
        toast.success(`${archiveTarget.name} archived`);
      }
      fetchPlans();
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setArchiveLoading(false);
      setArchiveTarget(null);
    }
  };

  const confirmDeleteBundle = async () => {
    if (!deleteBundleTarget) return;
    setDeletingBundle(true);
    setDeleteBundleError('');
    try {
      await adminApi.deleteBundle(deleteBundleTarget.id);
      setDeleteBundleTarget(null);
      toast.success(`${deleteBundleTarget.name} deleted`);
      fetchBundles();
    } catch (err: unknown) {
      setDeleteBundleError(getErrorMessage(err));
    } finally {
      setDeletingBundle(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <CreditCard className="w-7 h-7 text-primary-400" />
            Plans
          </h1>
          <p className="text-dark-400 mt-1">{plans.length} plan{plans.length !== 1 ? 's' : ''}</p>
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add Plan
          </button>
        )}
      </div>

      {/* Plans Table */}
      <div className="bg-dark-900/50 rounded-xl border border-dark-800 overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-dark-800">
              <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Name</th>
              <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Status</th>
              <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Price</th>
              <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Subscribers</th>
              <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Credits</th>
              <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Entitlements</th>
              <th className="text-right px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-dark-800">
            {plans.map((plan) => (
              <tr
                key={plan.id}
                onClick={() => setEditPlan(plan)}
                className={`hover:bg-dark-800/30 transition-colors cursor-pointer ${plan.isArchived ? 'opacity-50' : ''}`}
              >
                <td className="px-6 py-4">
                  <span className="text-white font-medium">{plan.name}</span>
                  {plan.description && <div className="text-xs text-dark-500 mt-0.5">{plan.description}</div>}
                </td>
                <td className="px-6 py-4">
                  {plan.isSystem ? (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium bg-primary-500/20 text-primary-400 rounded-full">
                      <Shield className="w-3 h-3" />
                      System
                    </span>
                  ) : plan.isArchived ? (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium bg-amber-500/20 text-amber-400 rounded-full">
                      <Archive className="w-3 h-3" />
                      Archived
                    </span>
                  ) : (
                    <span className="inline-flex px-2 py-0.5 text-xs font-medium bg-accent-emerald/10 text-accent-emerald rounded-full">
                      Active
                    </span>
                  )}
                </td>
                <td className="px-6 py-4 text-dark-300 text-sm">
                  {plan.pricingModel === 'per_seat' ? (
                    <>
                      {plan.monthlyPriceCents > 0 && `${formatPrice(plan.monthlyPriceCents)} + `}
                      {formatPrice(plan.perSeatPriceCents)}/seat/mo
                      {plan.includedSeats > 0 && <span className="text-xs text-dark-500 ml-1">({plan.includedSeats} incl)</span>}
                    </>
                  ) : (
                    plan.monthlyPriceCents === 0 ? 'Free' : `${formatPrice(plan.monthlyPriceCents)}/mo`
                  )}
                  {plan.annualDiscountPct > 0 && (
                    <span className="ml-1 text-xs text-accent-green">({plan.annualDiscountPct}% annual)</span>
                  )}
                </td>
                <td className="px-6 py-4">
                  <span className={`inline-flex px-2 py-0.5 text-xs font-medium rounded-full tabular-nums ${
                    (plan.subscriberCount ?? 0) > 0 ? 'bg-primary-500/10 text-primary-400' : 'bg-dark-700 text-dark-500'
                  }`}>
                    {plan.subscriberCount ?? 0}
                  </span>
                </td>
                <td className="px-6 py-4 text-dark-300 text-sm">
                  {plan.usageCreditsPerMonth > 0 ? (
                    <span>{plan.usageCreditsPerMonth}/mo ({plan.creditResetPolicy})</span>
                  ) : '—'}
                  {plan.bonusCredits > 0 && (
                    <span className="ml-1 text-xs text-accent-purple">+{plan.bonusCredits} bonus</span>
                  )}
                </td>
                <td className="px-6 py-4 text-sm">
                  {Object.keys(plan.entitlements || {}).length === 0 ? (
                    <span className="text-dark-500">—</span>
                  ) : (
                    <div className="flex flex-wrap gap-1">
                      {Object.entries(plan.entitlements).map(([key, val]) => (
                        <span key={key} className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
                          val.type === 'bool'
                            ? val.boolValue ? 'bg-accent-emerald/10 text-accent-emerald' : 'bg-dark-700 text-dark-500'
                            : val.numericValue > 0 ? 'bg-primary-500/10 text-primary-400' : 'bg-dark-700 text-dark-500'
                        }`}>
                          {key}{val.type === 'numeric' ? `: ${val.numericValue}` : val.boolValue ? '' : ': off'}
                        </span>
                      ))}
                    </div>
                  )}
                </td>
                <td className="px-6 py-4 text-right">
                  {canWrite && (
                    <div className="flex items-center justify-end gap-1">
                      {!plan.isSystem && !plan.isArchived && (
                        <button
                          onClick={(e) => { e.stopPropagation(); setArchiveTarget(plan); }}
                          className="p-2 text-dark-400 hover:text-amber-400 transition-colors"
                          title="Archive plan"
                          aria-label="Archive plan"
                        >
                          <Archive className="w-4 h-4" />
                        </button>
                      )}
                      {!plan.isSystem && plan.isArchived && (
                        <button
                          onClick={(e) => { e.stopPropagation(); setArchiveTarget(plan); }}
                          className="p-2 text-dark-400 hover:text-accent-emerald transition-colors"
                          title="Unarchive plan"
                          aria-label="Unarchive plan"
                        >
                          <RotateCcw className="w-4 h-4" />
                        </button>
                      )}
                      {!plan.isSystem && (plan.subscriberCount ?? 0) === 0 && (
                        <button
                          onClick={(e) => { e.stopPropagation(); setDeleteTarget(plan); }}
                          className="p-2 text-dark-400 hover:text-red-400 transition-colors"
                          title="Delete plan"
                          aria-label="Delete plan"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Create Modal */}
      {canWrite && showCreate && (
        <PlanFormModal
          onClose={() => setShowCreate(false)}
          onSaved={() => { setShowCreate(false); fetchPlans(); }}
        />
      )}

      {/* Edit Modal */}
      {editPlan && (
        <PlanFormModal
          plan={editPlan}
          subscriberCount={editPlan.subscriberCount}
          readOnly={!canWrite}
          onClose={() => setEditPlan(null)}
          onSaved={() => { setEditPlan(null); fetchPlans(); }}
        />
      )}

      {/* Delete Confirm Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => { setDeleteTarget(null); setDeleteError(''); }} />
          <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-md">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 rounded-full bg-red-500/20 flex items-center justify-center">
                <AlertTriangle className="w-5 h-5 text-red-400" />
              </div>
              <h3 className="text-lg font-semibold text-white">Delete Plan</h3>
            </div>
            <p className="text-dark-300 mb-6">
              Are you sure you want to delete <strong className="text-white">{deleteTarget.name}</strong>? This cannot be undone.
            </p>
            {deleteError && (
              <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
                {deleteError}
              </div>
            )}
            <div className="flex justify-end gap-3">
              <button
                onClick={() => { setDeleteTarget(null); setDeleteError(''); }}
                className="px-4 py-2 text-sm font-medium text-dark-300 hover:text-white transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmDelete}
                disabled={deleting}
                className="px-4 py-2 text-sm font-medium bg-red-500 text-white rounded-lg hover:bg-red-600 transition-colors disabled:opacity-50"
              >
                {deleting ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ─── Credit Bundles Section ─────────────────────────────────────── */}
      <div className="mt-12">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h2 className="text-xl font-bold text-white flex items-center gap-3">
              <Zap className="w-6 h-6 text-accent-purple" />
              Credit Bundles
            </h2>
            <p className="text-dark-400 mt-1">
              {bundles.length} bundle{bundles.length !== 1 ? 's' : ''}
              {bundles.length === 0 && ' — end users cannot purchase one-time credits'}
            </p>
          </div>
          {canWrite && (
            <button
              onClick={() => setShowCreateBundle(true)}
              className="flex items-center gap-2 px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 transition-colors"
            >
              <Plus className="w-4 h-4" />
              Add Bundle
            </button>
          )}
        </div>

        {bundles.length > 0 && (
          <div className="bg-dark-900/50 rounded-xl border border-dark-800 overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-dark-800">
                  <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Name</th>
                  <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Credits</th>
                  <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Price</th>
                  <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Active</th>
                  <th className="text-left px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Sort Order</th>
                  <th className="text-right px-6 py-4 text-xs font-semibold text-dark-400 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-dark-800">
                {bundles.map((bundle) => (
                  <tr
                    key={bundle.id}
                    onClick={() => setEditBundle(bundle)}
                    className="hover:bg-dark-800/30 transition-colors cursor-pointer"
                  >
                    <td className="px-6 py-4 text-white font-medium">{bundle.name}</td>
                    <td className="px-6 py-4 text-dark-300 text-sm">{bundle.credits.toLocaleString()}</td>
                    <td className="px-6 py-4 text-dark-300 text-sm">{formatPrice(bundle.priceCents)}</td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex px-2 py-0.5 text-xs font-medium rounded-full ${
                        bundle.isActive ? 'bg-accent-emerald/10 text-accent-emerald' : 'bg-dark-700 text-dark-500'
                      }`}>
                        {bundle.isActive ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-dark-300 text-sm">{bundle.sortOrder}</td>
                    <td className="px-6 py-4 text-right">
                      {canWrite && (
                        <button
                          onClick={(e) => { e.stopPropagation(); setDeleteBundleTarget(bundle); }}
                          className="p-2 text-dark-400 hover:text-red-400 transition-colors"
                          title="Delete bundle"
                          aria-label="Delete bundle"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Create Bundle Modal */}
      {canWrite && showCreateBundle && (
        <BundleFormModal
          onClose={() => setShowCreateBundle(false)}
          onSaved={() => { setShowCreateBundle(false); fetchBundles(); }}
        />
      )}

      {/* Edit Bundle Modal */}
      {editBundle && (
        <BundleFormModal
          bundle={editBundle}
          readOnly={!canWrite}
          onClose={() => setEditBundle(null)}
          onSaved={() => { setEditBundle(null); fetchBundles(); }}
        />
      )}

      {/* Archive/Unarchive Confirm Modal */}
      <ConfirmModal
        open={archiveTarget !== null}
        onClose={() => setArchiveTarget(null)}
        onConfirm={handleArchiveConfirm}
        title={archiveTarget?.isArchived ? 'Unarchive Plan' : 'Archive Plan'}
        message={archiveTarget?.isArchived
          ? `Are you sure you want to unarchive ${archiveTarget?.name}? It will become available for new subscriptions.`
          : `Are you sure you want to archive ${archiveTarget?.name}? Existing subscribers will keep their plan, but no new subscriptions can be created.`}
        confirmLabel={archiveTarget?.isArchived ? 'Unarchive' : 'Archive'}
        confirmVariant={archiveTarget?.isArchived ? 'primary' : 'danger'}
        loading={archiveLoading}
      />

      {/* Delete Bundle Confirm Modal */}
      {deleteBundleTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => { setDeleteBundleTarget(null); setDeleteBundleError(''); }} />
          <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-md">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 rounded-full bg-red-500/20 flex items-center justify-center">
                <AlertTriangle className="w-5 h-5 text-red-400" />
              </div>
              <h3 className="text-lg font-semibold text-white">Delete Credit Bundle</h3>
            </div>
            <p className="text-dark-300 mb-6">
              Are you sure you want to delete <strong className="text-white">{deleteBundleTarget.name}</strong>? This cannot be undone.
            </p>
            {deleteBundleError && (
              <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
                {deleteBundleError}
              </div>
            )}
            <div className="flex justify-end gap-3">
              <button
                onClick={() => { setDeleteBundleTarget(null); setDeleteBundleError(''); }}
                className="px-4 py-2 text-sm font-medium text-dark-300 hover:text-white transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmDeleteBundle}
                disabled={deletingBundle}
                className="px-4 py-2 text-sm font-medium bg-red-500 text-white rounded-lg hover:bg-red-600 transition-colors disabled:opacity-50"
              >
                {deletingBundle ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Plan Form Modal (Create / Edit) ────────────────────────────────────────

interface PlanFormModalProps {
  plan?: Plan;
  subscriberCount?: number;
  readOnly?: boolean;
  onClose: () => void;
  onSaved: () => void;
}

function PlanFormModal({ plan, subscriberCount, readOnly, onClose, onSaved }: PlanFormModalProps) {
  const isEdit = !!plan;

  const [name, setName] = useState(plan?.name ?? '');
  const [description, setDescription] = useState(plan?.description ?? '');
  const [monthlyPriceDollars, setMonthlyPriceDollars] = useState(plan ? (plan.monthlyPriceCents / 100).toFixed(2) : '0.00');
  const [annualDiscountPct, setAnnualDiscountPct] = useState(String(plan?.annualDiscountPct ?? 0));
  const [usageCreditsPerMonth, setUsageCreditsPerMonth] = useState(String(plan?.usageCreditsPerMonth ?? 0));
  const [creditResetPolicy, setCreditResetPolicy] = useState<'reset' | 'accrue'>(plan?.creditResetPolicy ?? 'reset');
  const [bonusCredits, setBonusCredits] = useState(String(plan?.bonusCredits ?? 0));
  const [userLimit, setUserLimit] = useState(String(plan?.userLimit ?? 0));
  const [pricingModel, setPricingModel] = useState<'flat' | 'per_seat'>(plan?.pricingModel ?? 'flat');
  const [perSeatPriceDollars, setPerSeatPriceDollars] = useState(plan ? (plan.perSeatPriceCents / 100).toFixed(2) : '0.00');
  const [includedSeats, setIncludedSeats] = useState(String(plan?.includedSeats ?? 0));
  const [minSeats, setMinSeats] = useState(String(plan?.minSeats ?? 1));
  const [maxSeats, setMaxSeats] = useState(String(plan?.maxSeats ?? 0));
  const [trialDays, setTrialDays] = useState(String(plan?.trialDays ?? 0));
  const [entitlements, setEntitlements] = useState<Record<string, EntitlementValue>>(plan?.entitlements ?? {});

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // Known entitlement keys from all plans
  const [knownKeys, setKnownKeys] = useState<EntitlementKeyInfo[]>([]);
  useEffect(() => {
    adminApi.listEntitlementKeys().then(data => setKnownKeys(data.keys)).catch(() => { /* non-critical, entitlement key hints are optional */ });
  }, []);

  // New entitlement row state
  const [newKey, setNewKey] = useState('');
  const [newType, setNewType] = useState<EntitlementType>('bool');

  const handleSave = async () => {
    setSaving(true);
    setError('');
    const priceCents = Math.round(parseFloat(monthlyPriceDollars || '0') * 100);
    if (isNaN(priceCents) || priceCents < 0) {
      setError('Invalid price');
      setSaving(false);
      return;
    }

    const discount = parseInt(annualDiscountPct) || 0;
    if (discount < 0 || discount > 100) {
      setError('Annual discount must be between 0 and 100');
      setSaving(false);
      return;
    }

    const trial = parseInt(trialDays) || 0;
    if (trial < 0) {
      setError('Trial days cannot be negative');
      setSaving(false);
      return;
    }

    const parsedMinSeats = parseInt(minSeats) || 1;
    const parsedMaxSeats = parseInt(maxSeats) || 0;
    if (pricingModel === 'per_seat' && parsedMaxSeats > 0 && parsedMaxSeats < parsedMinSeats) {
      setError('Max seats cannot be less than min seats');
      setSaving(false);
      return;
    }

    const perSeatPriceCents = Math.round(parseFloat(perSeatPriceDollars || '0') * 100);

    const payload = {
      name: name.trim(),
      description: description.trim(),
      monthlyPriceCents: priceCents,
      annualDiscountPct: discount,
      usageCreditsPerMonth: parseInt(usageCreditsPerMonth) || 0,
      creditResetPolicy,
      bonusCredits: parseInt(bonusCredits) || 0,
      userLimit: parseInt(userLimit) || 0,
      pricingModel,
      perSeatPriceCents,
      includedSeats: parseInt(includedSeats) || 0,
      minSeats: parsedMinSeats,
      maxSeats: parsedMaxSeats,
      trialDays: trial,
      entitlements,
    };

    try {
      if (isEdit) {
        await adminApi.updatePlan(plan!.id, payload);
      } else {
        await adminApi.createPlan(payload);
      }
      onSaved();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const addEntitlement = () => {
    const key = newKey.trim().toLowerCase().replace(/\s+/g, '_');
    if (!key || entitlements[key] !== undefined) return;
    const known = knownKeys.find(k => k.key === key);
    const desc = known?.description ?? '';
    setEntitlements(prev => ({
      ...prev,
      [key]: newType === 'bool' ? { type: 'bool', boolValue: false, numericValue: 0, description: desc } : { type: 'numeric', boolValue: false, numericValue: 0, description: desc },
    }));
    setNewKey('');
  };

  const removeEntitlement = (key: string) => {
    setEntitlements(prev => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const updateEntitlementValue = (key: string, val: EntitlementValue) => {
    setEntitlements(prev => ({ ...prev, [key]: val }));
  };

  // Merge known keys that aren't already in entitlements, for display
  const allKeys = Array.from(new Set([
    ...Object.keys(entitlements),
    ...knownKeys.map(k => k.key),
  ]));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold text-white">{readOnly ? 'View Plan' : isEdit ? 'Edit Plan' : 'Create Plan'}</h3>
          <button onClick={onClose} className="p-2 text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        {isEdit && (subscriberCount ?? 0) > 0 && (
          <div className="mb-4 p-3 bg-amber-500/10 border border-amber-500/30 rounded-lg flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400 mt-0.5 flex-shrink-0" />
            <p className="text-sm text-amber-400">
              <strong>{subscriberCount} tenant{subscriberCount !== 1 ? 's' : ''}</strong> subscribed to this plan. Changes to pricing, credits, limits, and entitlements will affect existing subscribers.
            </p>
          </div>
        )}

        <div className="space-y-6">
          {/* Basics */}
          <div>
            <h4 className="text-sm font-semibold text-dark-300 uppercase tracking-wider mb-3">Basics</h4>
            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1">Name</label>
                <input
                  value={name}
                  onChange={e => setName(e.target.value)}
                  disabled={readOnly || (isEdit && plan?.isSystem)}
                  className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                  placeholder="e.g. Pro"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1">Description</label>
                <input
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  disabled={readOnly}
                  className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                  placeholder="Short description"
                />
              </div>
            </div>
          </div>

          {/* Pricing */}
          <div>
            <h4 className="text-sm font-semibold text-dark-300 uppercase tracking-wider mb-3">Pricing</h4>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1">Monthly Price ($)</label>
                <input
                  type="text"
                  inputMode="decimal"
                  value={monthlyPriceDollars}
                  onChange={e => setMonthlyPriceDollars(e.target.value)}
                  onFocus={e => e.target.select()}
                  onBlur={() => { const n = parseFloat(monthlyPriceDollars); setMonthlyPriceDollars(isNaN(n) ? '0.00' : n.toFixed(2)); }}
                  disabled={readOnly}
                  className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1">Annual Discount %</label>
                <input
                  type="text"
                  inputMode="numeric"
                  value={annualDiscountPct}
                  onChange={e => setAnnualDiscountPct(e.target.value)}
                  onFocus={e => e.target.select()}
                  onBlur={() => setAnnualDiscountPct(String(parseInt(annualDiscountPct) || 0))}
                  disabled={readOnly}
                  className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                />
                <p className="text-xs text-dark-500 mt-1">Set to 0 to hide annual option</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1">Trial Days</label>
                <input
                  type="text"
                  inputMode="numeric"
                  value={trialDays}
                  onChange={e => setTrialDays(e.target.value)}
                  onFocus={e => e.target.select()}
                  onBlur={() => setTrialDays(String(parseInt(trialDays) || 0))}
                  disabled={readOnly}
                  className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                />
                <p className="text-xs text-dark-500 mt-1">0 = no trial</p>
              </div>
            </div>
          </div>

          {/* Pricing Model */}
          <div>
            <h4 className="text-sm font-semibold text-dark-300 uppercase tracking-wider mb-3">Pricing Model</h4>
            <div className="flex gap-3 mb-3">
              <button
                type="button"
                onClick={() => setPricingModel('flat')}
                disabled={readOnly}
                className={`flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-colors disabled:opacity-50 ${
                  pricingModel === 'flat' ? 'bg-primary-500/20 border-primary-500/50 text-primary-400' : 'bg-dark-800 border-dark-700 text-dark-400 hover:text-white'
                }`}
              >
                Flat Rate
              </button>
              <button
                type="button"
                onClick={() => setPricingModel('per_seat')}
                disabled={readOnly}
                className={`flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-colors disabled:opacity-50 ${
                  pricingModel === 'per_seat' ? 'bg-primary-500/20 border-primary-500/50 text-primary-400' : 'bg-dark-800 border-dark-700 text-dark-400 hover:text-white'
                }`}
              >
                Per Seat
              </button>
            </div>

            {pricingModel === 'per_seat' && (
              <div className="space-y-3 p-3 bg-dark-800/30 rounded-lg border border-dark-700/50">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-sm font-medium text-dark-300 mb-1">Per Seat Price ($/mo)</label>
                    <input
                      type="text"
                      inputMode="decimal"
                      value={perSeatPriceDollars}
                      onChange={e => setPerSeatPriceDollars(e.target.value)}
                      onFocus={e => e.target.select()}
                      onBlur={() => { const n = parseFloat(perSeatPriceDollars); setPerSeatPriceDollars(isNaN(n) ? '0.00' : n.toFixed(2)); }}
                      disabled={readOnly}
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-dark-300 mb-1">Included Seats</label>
                    <input
                      type="text"
                      inputMode="numeric"
                      value={includedSeats}
                      onChange={e => setIncludedSeats(e.target.value)}
                      onFocus={e => e.target.select()}
                      onBlur={() => setIncludedSeats(String(parseInt(includedSeats) || 0))}
                      disabled={readOnly}
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                    />
                    <p className="text-xs text-dark-500 mt-1">Seats included in base price (0 = purely per-seat)</p>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-sm font-medium text-dark-300 mb-1">Min Seats</label>
                    <input
                      type="text"
                      inputMode="numeric"
                      value={minSeats}
                      onChange={e => setMinSeats(e.target.value)}
                      onFocus={e => e.target.select()}
                      onBlur={() => setMinSeats(String(parseInt(minSeats) || 1))}
                      disabled={readOnly}
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-dark-300 mb-1">Max Seats</label>
                    <input
                      type="text"
                      inputMode="numeric"
                      value={maxSeats}
                      onChange={e => setMaxSeats(e.target.value)}
                      onFocus={e => e.target.select()}
                      onBlur={() => setMaxSeats(String(parseInt(maxSeats) || 0))}
                      disabled={readOnly}
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                    />
                    <p className="text-xs text-dark-500 mt-1">0 = unlimited</p>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Credits */}
          <div>
            <h4 className="text-sm font-semibold text-dark-300 uppercase tracking-wider mb-3">Credits</h4>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1">Usage Credits / Month</label>
                <input
                  type="text"
                  inputMode="numeric"
                  value={usageCreditsPerMonth}
                  onChange={e => setUsageCreditsPerMonth(e.target.value)}
                  onFocus={e => e.target.select()}
                  onBlur={() => setUsageCreditsPerMonth(String(parseInt(usageCreditsPerMonth) || 0))}
                  disabled={readOnly}
                  className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                />
              </div>
              {(parseInt(usageCreditsPerMonth) || 0) > 0 && (
                <div>
                  <label className="block text-sm font-medium text-dark-300 mb-1">Reset Policy</label>
                  <select
                    value={creditResetPolicy}
                    onChange={e => setCreditResetPolicy(e.target.value as 'reset' | 'accrue')}
                    disabled={readOnly}
                    className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                  >
                    <option value="reset">Reset each month</option>
                    <option value="accrue">Accrue (roll over)</option>
                  </select>
                </div>
              )}
            </div>
            <div className="mt-3">
              <label className="block text-sm font-medium text-dark-300 mb-1">Bonus Credits (one-time)</label>
              <input
                type="text"
                inputMode="numeric"
                value={bonusCredits}
                onChange={e => setBonusCredits(e.target.value)}
                onFocus={e => e.target.select()}
                onBlur={() => setBonusCredits(String(parseInt(bonusCredits) || 0))}
                disabled={readOnly}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
              />
              <p className="text-xs text-dark-500 mt-1">Added once when plan is activated</p>
            </div>
          </div>

          {/* Limits */}
          <div>
            <h4 className="text-sm font-semibold text-dark-300 uppercase tracking-wider mb-3">Limits</h4>
            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1">User Limit</label>
              <input
                type="text"
                inputMode="numeric"
                value={userLimit}
                onChange={e => setUserLimit(e.target.value)}
                onFocus={e => e.target.select()}
                onBlur={() => setUserLimit(String(parseInt(userLimit) || 0))}
                disabled={readOnly}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
              />
              <p className="text-xs text-dark-500 mt-1">0 = unlimited</p>
            </div>
          </div>

          {/* Entitlements */}
          <div>
            <h4 className="text-sm font-semibold text-dark-300 uppercase tracking-wider mb-3">Entitlements</h4>
            {allKeys.length > 0 && (
              <div className="space-y-2 mb-3">
                {allKeys.map(key => {
                  const knownKey = knownKeys.find(k => k.key === key);
                  const type = entitlements[key]?.type ?? knownKey?.type ?? 'bool';
                  const val = entitlements[key];
                  const isBool = type === 'bool';
                  const desc = val?.description ?? knownKey?.description ?? '';

                  return (
                    <div key={key} className="bg-dark-800/50 rounded-lg px-3 py-2 space-y-2">
                      <div className="flex items-center gap-3">
                        <span className="text-sm text-white font-mono flex-1">{key}</span>
                        <span className="text-xs text-dark-500 uppercase">{type}</span>
                        {isBool ? (
                          <button
                            onClick={() => updateEntitlementValue(key, { type: 'bool', boolValue: !(val?.boolValue ?? false), numericValue: 0, description: desc })}
                            disabled={readOnly}
                            className={`w-10 h-6 rounded-full transition-colors relative disabled:opacity-50 ${val?.boolValue ? 'bg-primary-500' : 'bg-dark-600'}`}
                          >
                            <span className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${val?.boolValue ? 'translate-x-4' : ''}`} />
                          </button>
                        ) : (
                          <input
                            type="text"
                            inputMode="numeric"
                            value={val?.numericValue ?? 0}
                            onChange={e => {
                              const raw = e.target.value;
                              const n = raw === '' ? 0 : parseInt(raw) || 0;
                              updateEntitlementValue(key, { type: 'numeric', boolValue: false, numericValue: n, description: desc });
                            }}
                            onFocus={e => e.target.select()}
                            disabled={readOnly}
                            className="w-24 px-2 py-1 bg-dark-700 border border-dark-600 rounded text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
                          />
                        )}
                        {!readOnly && (
                          <button
                            onClick={() => removeEntitlement(key)}
                            className="p-1 text-dark-400 hover:text-red-400 transition-colors"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        )}
                      </div>
                      <input
                        value={val?.description ?? desc}
                        onChange={e => updateEntitlementValue(key, { ...(val ?? { type: type as EntitlementType, boolValue: false, numericValue: 0, description: '' }), description: e.target.value })}
                        placeholder="Description (shown to end users)"
                        disabled={readOnly}
                        className="w-full px-2 py-1 bg-dark-700/50 border border-dark-600/50 rounded text-dark-300 text-xs focus:border-primary-500 focus:outline-none disabled:opacity-50"
                      />
                    </div>
                  );
                })}
              </div>
            )}

            {/* Add new entitlement */}
            {!readOnly && (
              <div className="flex items-center gap-2">
                <input
                  value={newKey}
                  onChange={e => setNewKey(e.target.value)}
                  placeholder="entitlement_name"
                  className="flex-1 px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm font-mono focus:border-primary-500 focus:outline-none"
                  onKeyDown={e => { if (e.key === 'Enter') addEntitlement(); }}
                />
                <select
                  value={newType}
                  onChange={e => setNewType(e.target.value as EntitlementType)}
                  className="px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none"
                >
                  <option value="bool">Boolean</option>
                  <option value="numeric">Numeric</option>
                </select>
                <button
                  onClick={addEntitlement}
                  disabled={!newKey.trim()}
                  className="px-3 py-2 bg-dark-700 text-dark-300 text-sm rounded-lg hover:bg-dark-600 hover:text-white transition-colors disabled:opacity-30"
                >
                  <Plus className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
            {error}
          </div>
        )}

        {/* Actions */}
        <div className="flex justify-end gap-3 mt-6 pt-4 border-t border-dark-800">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-dark-300 hover:text-white transition-colors"
          >
            {readOnly ? 'Close' : 'Cancel'}
          </button>
          {!readOnly && (
            <button
              onClick={handleSave}
              disabled={saving || !name.trim()}
              className="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-50"
            >
              {saving ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Plan'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── Bundle Form Modal (Create / Edit) ─────────────────────────────────────

interface BundleFormModalProps {
  bundle?: CreditBundle;
  readOnly?: boolean;
  onClose: () => void;
  onSaved: () => void;
}

function BundleFormModal({ bundle, readOnly, onClose, onSaved }: BundleFormModalProps) {
  const isEdit = !!bundle;

  const [name, setName] = useState(bundle?.name ?? '');
  const [credits, setCredits] = useState(String(bundle?.credits ?? 100));
  const [priceDollars, setPriceDollars] = useState(bundle ? (bundle.priceCents / 100).toFixed(2) : '9.99');
  const [isActive, setIsActive] = useState(bundle?.isActive ?? true);
  const [sortOrder, setSortOrder] = useState(String(bundle?.sortOrder ?? 0));

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const handleSave = async () => {
    setSaving(true);
    setError('');
    const priceCents = Math.round(parseFloat(priceDollars || '0') * 100);
    if (isNaN(priceCents) || priceCents <= 0) {
      setError('Price must be greater than $0');
      setSaving(false);
      return;
    }
    const creditsNum = parseInt(credits) || 0;
    if (creditsNum <= 0) {
      setError('Credits must be greater than 0');
      setSaving(false);
      return;
    }

    const sortOrderNum = parseInt(sortOrder) || 0;
    if (sortOrderNum < 0) {
      setError('Sort order cannot be negative');
      setSaving(false);
      return;
    }

    const payload = {
      name: name.trim(),
      credits: creditsNum,
      priceCents,
      isActive,
      sortOrder: sortOrderNum,
    };

    try {
      if (isEdit) {
        await adminApi.updateBundle(bundle!.id, payload);
      } else {
        await adminApi.createBundle(payload);
      }
      onSaved();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-lg">
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold text-white">{readOnly ? 'View Credit Bundle' : isEdit ? 'Edit Credit Bundle' : 'Create Credit Bundle'}</h3>
          <button onClick={onClose} className="p-2 text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">Name</label>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              disabled={readOnly}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
              placeholder="e.g. Starter Pack"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1">Credits</label>
              <input
                type="text"
                inputMode="numeric"
                value={credits}
                onChange={e => setCredits(e.target.value)}
                onFocus={e => e.target.select()}
                onBlur={() => setCredits(String(parseInt(credits) || 0))}
                disabled={readOnly}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1">Price ($)</label>
              <input
                type="text"
                inputMode="decimal"
                value={priceDollars}
                onChange={e => setPriceDollars(e.target.value)}
                onFocus={e => e.target.select()}
                onBlur={() => { const n = parseFloat(priceDollars); setPriceDollars(isNaN(n) ? '0.00' : n.toFixed(2)); }}
                disabled={readOnly}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1">Sort Order</label>
              <input
                type="text"
                inputMode="numeric"
                value={sortOrder}
                onChange={e => setSortOrder(e.target.value)}
                onFocus={e => e.target.select()}
                onBlur={() => setSortOrder(String(parseInt(sortOrder) || 0))}
                disabled={readOnly}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none disabled:opacity-50"
              />
              <p className="text-xs text-dark-500 mt-1">Lower numbers display first</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1">Active</label>
              <button
                onClick={() => setIsActive(!isActive)}
                disabled={readOnly}
                className={`w-12 h-7 rounded-full transition-colors relative mt-1 disabled:opacity-50 ${isActive ? 'bg-primary-500' : 'bg-dark-600'}`}
              >
                <span className={`absolute top-0.5 left-0.5 w-6 h-6 rounded-full bg-white transition-transform ${isActive ? 'translate-x-5' : ''}`} />
              </button>
              <p className="text-xs text-dark-500 mt-1">Inactive bundles are hidden from users</p>
            </div>
          </div>
        </div>

        {error && (
          <div className="mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-3 mt-6 pt-4 border-t border-dark-800">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-dark-300 hover:text-white transition-colors"
          >
            {readOnly ? 'Close' : 'Cancel'}
          </button>
          {!readOnly && (
            <button
              onClick={handleSave}
              disabled={saving || !name.trim()}
              className="px-4 py-2 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-50"
            >
              {saving ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Bundle'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
