import { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { Shield, Check, X, Zap, Hash, ToggleLeft } from 'lucide-react';
import { toast } from 'sonner';
import { plansApi } from '../../api/client';
import { useTenant } from '../../contexts/TenantContext';
import type { Plan, EntitlementType } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';

function renderTemplate(template: string, vars: Record<string, string | number>): string {
  let result = template;
  for (const [key, value] of Object.entries(vars)) {
    result = result.replace(new RegExp(`\\{\\{\\.${key}\\}\\}`, 'g'), String(value));
  }
  result = result.replace(/\{\{if ne \.(\w+) (\d+)\}\}(.*?)\{\{end\}\}/g, (_match, varName, compare, content) => {
    return String(vars[varName]) !== compare ? content : '';
  });
  return result;
}

interface EntitlementInfo {
  key: string;
  type: EntitlementType;
  description: string;
}

export default function TestEntitlementsPage() {
  const navigate = useNavigate();
  const { isRootTenant } = useTenant();

  const [plans, setPlans] = useState<Plan[]>([]);
  const [currentPlanId, setCurrentPlanId] = useState('');
  const [loading, setLoading] = useState(true);
  const [promptTitle, setPromptTitle] = useState('');
  const [promptBody, setPromptBody] = useState('');
  const [promptNumericBody, setPromptNumericBody] = useState('');
  const [testResults, setTestResults] = useState<Record<string, 'success' | 'fail'>>({});
  const [numericInputs, setNumericInputs] = useState<Record<string, string>>({});
  const [showUpgradeModal, setShowUpgradeModal] = useState(false);
  const [testingKey, setTestingKey] = useState<string | null>(null);
  const [testedValue, setTestedValue] = useState(0);

  useEffect(() => {
    plansApi.list()
      .then((data) => {
        setPlans(data.plans);
        setCurrentPlanId(data.currentPlanId);
        setPromptTitle(data.entitlementUpgradePromptTitle || 'Upgrade required');
        setPromptBody(data.entitlementUpgradePromptBody || '');
        setPromptNumericBody(data.entitlementUpgradePromptNumericBody || '');
      })
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  if (!isRootTenant) {
    return <Navigate to="/dashboard" replace />;
  }

  const currentPlan = plans.find(p => p.id === currentPlanId);

  // Collect all unique entitlement keys across all plans
  const allEntitlements: EntitlementInfo[] = [];
  const seenKeys = new Set<string>();
  for (const plan of plans) {
    for (const [key, val] of Object.entries(plan.entitlements || {})) {
      if (!seenKeys.has(key)) {
        seenKeys.add(key);
        allEntitlements.push({ key, type: val.type, description: val.description || key });
      }
    }
  }

  const handleTest = (entitlementKey: string) => {
    const currentEnt = currentPlan?.entitlements?.[entitlementKey];
    const meta = allEntitlements.find(e => e.key === entitlementKey);

    let passed = false;
    let requestedValue = 0;
    if (meta?.type === 'bool') {
      passed = currentEnt?.boolValue === true;
    } else if (meta?.type === 'numeric') {
      requestedValue = Math.max(1, parseInt(numericInputs[entitlementKey] || '1', 10) || 1);
      passed = (currentEnt?.numericValue ?? 0) >= requestedValue;
    }

    if (passed) {
      setTestResults(prev => ({ ...prev, [entitlementKey]: 'success' }));
      setTimeout(() => setTestResults(prev => {
        const next = { ...prev };
        if (next[entitlementKey] === 'success') delete next[entitlementKey];
        return next;
      }), 3000);
    } else {
      setTestedValue(requestedValue);
      setTestResults(prev => ({ ...prev, [entitlementKey]: 'fail' }));
      setTestingKey(entitlementKey);
      setShowUpgradeModal(true);
    }
  };

  const getRecommendedPlan = (entitlementKey: string, requestedValue: number): Plan | undefined => {
    const meta = allEntitlements.find(e => e.key === entitlementKey);
    const sortedByPrice = [...plans].sort((a, b) => a.monthlyPriceCents - b.monthlyPriceCents);

    return sortedByPrice.find(p => {
      if (p.id === currentPlanId) return false;
      const ent = p.entitlements?.[entitlementKey];
      if (!ent) return false;

      if (meta?.type === 'bool') {
        return ent.boolValue === true;
      } else if (meta?.type === 'numeric') {
        return ent.numericValue >= requestedValue;
      }
      return false;
    });
  };

  const formatCurrentValue = (entitlementKey: string): React.ReactNode => {
    const ent = currentPlan?.entitlements?.[entitlementKey];
    const meta = allEntitlements.find(e => e.key === entitlementKey);
    if (!ent) {
      return <span className="text-dark-500">Not included</span>;
    }
    if (meta?.type === 'bool') {
      return ent.boolValue
        ? <span className="flex items-center gap-1 text-accent-emerald"><Check className="w-4 h-4" /> Enabled</span>
        : <span className="flex items-center gap-1 text-dark-500"><X className="w-4 h-4" /> Disabled</span>;
    }
    if (meta?.type === 'numeric') {
      return ent.numericValue > 0
        ? <span className="text-white font-medium">{ent.numericValue.toLocaleString()}</span>
        : <span className="text-dark-500">0</span>;
    }
    return <span className="text-dark-500">—</span>;
  };

  const formatPlanValue = (plan: Plan, entitlementKey: string): React.ReactNode => {
    const ent = plan.entitlements?.[entitlementKey];
    const meta = allEntitlements.find(e => e.key === entitlementKey);
    if (!ent) {
      return <span className="text-dark-600">—</span>;
    }
    if (meta?.type === 'bool') {
      return ent.boolValue
        ? <Check className="w-4 h-4 text-accent-emerald" />
        : <X className="w-4 h-4 text-dark-600" />;
    }
    if (meta?.type === 'numeric') {
      return <span className={ent.numericValue > 0 ? 'text-white' : 'text-dark-600'}>{ent.numericValue.toLocaleString()}</span>;
    }
    return <span className="text-dark-600">—</span>;
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white flex items-center gap-3">
          <Shield className="w-7 h-7 text-accent-emerald" />
          Test Entitlements
        </h1>
        <p className="text-dark-400 mt-1">
          Current plan: <span className="text-white font-medium">{currentPlan?.name || 'None'}</span>
        </p>
      </div>

      {allEntitlements.length === 0 ? (
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-12 text-center">
          <Shield className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <p className="text-dark-400">No entitlements are defined across any plan.</p>
          <p className="text-dark-500 text-sm mt-1">Add entitlements to your plans in Admin &rarr; Plans.</p>
        </div>
      ) : (
        <>
          {/* Entitlements Test Table */}
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden mb-8">
            <div className="px-6 py-4 border-b border-dark-800">
              <h2 className="text-lg font-semibold text-white">Entitlements on Current Plan</h2>
              <p className="text-sm text-dark-400 mt-0.5">Click "Test" to simulate using each entitlement</p>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-dark-800">
                    <th className="text-left px-6 py-3 text-sm font-medium text-dark-400">Entitlement</th>
                    <th className="text-left px-6 py-3 text-sm font-medium text-dark-400">Type</th>
                    <th className="text-left px-6 py-3 text-sm font-medium text-dark-400">Current Value</th>
                    <th className="text-right px-6 py-3 text-sm font-medium text-dark-400">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {allEntitlements.map((ent) => (
                    <tr key={ent.key} className="border-b border-dark-800/50">
                      <td className="px-6 py-3.5">
                        <p className="text-sm font-medium text-white">{ent.description}</p>
                        <p className="text-xs text-dark-500 font-mono">{ent.key}</p>
                      </td>
                      <td className="px-6 py-3.5">
                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${
                          ent.type === 'bool'
                            ? 'bg-primary-500/10 text-primary-400'
                            : 'bg-accent-purple/10 text-accent-purple'
                        }`}>
                          {ent.type === 'bool' ? <ToggleLeft className="w-3 h-3" /> : <Hash className="w-3 h-3" />}
                          {ent.type === 'bool' ? 'Boolean' : 'Numeric'}
                        </span>
                      </td>
                      <td className="px-6 py-3.5 text-sm">
                        {formatCurrentValue(ent.key)}
                      </td>
                      <td className="px-6 py-3.5 text-right">
                        <div className="flex items-center justify-end gap-2">
                          {testResults[ent.key] === 'success' && (
                            <span className="text-xs text-accent-emerald flex items-center gap-1">
                              <Check className="w-3.5 h-3.5" /> Access granted
                            </span>
                          )}
                          {testResults[ent.key] === 'fail' && (
                            <span className="text-xs text-red-400 flex items-center gap-1">
                              <X className="w-3.5 h-3.5" /> Blocked
                            </span>
                          )}
                          {ent.type === 'numeric' && (
                            <input
                              type="number"
                              min="1"
                              placeholder="Value"
                              value={numericInputs[ent.key] || ''}
                              onChange={(e) => setNumericInputs(prev => ({ ...prev, [ent.key]: e.target.value }))}
                              onKeyDown={(e) => { if (e.key === 'Enter') handleTest(ent.key); }}
                              className="w-20 px-2 py-1.5 text-xs bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 transition-colors text-right"
                            />
                          )}
                          <button
                            onClick={() => handleTest(ent.key)}
                            className="text-xs px-3 py-1.5 rounded-lg border border-dark-700 text-dark-300 hover:text-white hover:border-dark-600 transition-colors"
                          >
                            Test
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Comparison Matrix */}
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
            <div className="px-6 py-4 border-b border-dark-800">
              <h2 className="text-lg font-semibold text-white">Entitlements by Plan</h2>
              <p className="text-sm text-dark-400 mt-0.5">Compare entitlements across all plans</p>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-dark-800">
                    <th className="text-left px-6 py-3 text-sm font-medium text-dark-400 min-w-[200px]">Entitlement</th>
                    {plans.map((plan) => (
                      <th key={plan.id} className="text-center px-4 py-3 text-sm font-medium min-w-[100px]">
                        <span className={plan.id === currentPlanId ? 'text-primary-400' : 'text-dark-400'}>
                          {plan.name}
                        </span>
                        {plan.id === currentPlanId && (
                          <span className="block text-xs text-primary-400/60 mt-0.5">Current</span>
                        )}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {allEntitlements.map((ent) => (
                    <tr key={ent.key} className="border-b border-dark-800/50">
                      <td className="px-6 py-3 text-sm text-dark-300">{ent.description}</td>
                      {plans.map((plan) => (
                        <td key={plan.id} className={`text-center px-4 py-3 text-sm ${
                          plan.id === currentPlanId ? 'bg-primary-500/5' : ''
                        }`}>
                          {formatPlanValue(plan, ent.key)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}

      {/* Upgrade Modal */}
      {showUpgradeModal && testingKey && (() => {
        const entMeta = allEntitlements.find(e => e.key === testingKey);
        const recommended = getRecommendedPlan(testingKey, testedValue);
        const isNumeric = entMeta?.type === 'numeric';
        const templateVars = {
          EntitlementName: entMeta?.description || testingKey,
          PlanName: currentPlan?.name || '',
          AppName: 'MCPLens',
          RecommendedPlanName: recommended?.name || 'a higher plan',
          RequestedValue: testedValue,
          CurrentValue: currentPlan?.entitlements?.[testingKey]?.numericValue ?? 0,
        };
        const bodyTemplate = isNumeric && promptNumericBody ? promptNumericBody : promptBody;
        return (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full">
              <div className="flex items-center gap-3 mb-4">
                <Zap className="w-6 h-6 text-primary-400" />
                <h3 className="text-lg font-semibold text-white">
                  {renderTemplate(promptTitle, templateVars)}
                </h3>
              </div>
              <p className="text-dark-300 mb-6">
                {renderTemplate(bodyTemplate, templateVars)}
              </p>
              <div className="flex gap-3 justify-end">
                <button
                  onClick={() => { setShowUpgradeModal(false); setTestingKey(null); }}
                  className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    setShowUpgradeModal(false);
                    setTestingKey(null);
                    navigate(recommended ? `/plan?upgrade=${recommended.id}` : '/plan');
                  }}
                  className="px-4 py-2 text-sm bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
                >
                  Upgrade Plan
                </button>
              </div>
            </div>
          </div>
        );
      })()}
    </div>
  );
}
