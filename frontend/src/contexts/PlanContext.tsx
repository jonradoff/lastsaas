import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { plansApi } from '../api/client';
import { useTenant } from './TenantContext';
import { useAuth } from './AuthContext';
import type { Plan, PublicPlansResponse, EntitlementValue } from '../types';

interface PlanContextType {
  /** Current plan for the active tenant, or null if none */
  currentPlan: Plan | null;
  /** All available plans */
  plans: Plan[];
  /** Raw plans response with billing info */
  plansResponse: PublicPlansResponse | null;
  /** Whether plan data has loaded */
  isLoaded: boolean;
  /** Check a boolean entitlement */
  hasEntitlement: (key: string) => boolean;
  /** Get a numeric entitlement value (returns 0 if not set) */
  getNumericEntitlement: (key: string) => number;
  /** Get an entitlement value (raw) */
  getEntitlement: (key: string) => EntitlementValue | null;
  /** Current plan name, lowercased for tier checks */
  tierName: 'free' | 'pro' | 'max' | 'agency' | string;
  /** Is the tenant on a paid plan? */
  isPaid: boolean;
  /** Refresh plan data (e.g., after checkout) */
  refresh: () => void;
}

const PlanContext = createContext<PlanContextType | null>(null);

export function PlanProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const { activeTenant } = useTenant();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [currentPlan, setCurrentPlan] = useState<Plan | null>(null);
  const [plansResponse, setPlansResponse] = useState<PublicPlansResponse | null>(null);
  const [isLoaded, setIsLoaded] = useState(false);

  const loadPlans = useCallback(async () => {
    if (!isAuthenticated || !activeTenant) {
      setCurrentPlan(null);
      setPlans([]);
      setPlansResponse(null);
      setIsLoaded(false);
      return;
    }

    try {
      const data = await plansApi.list();
      setPlans(data.plans);
      setPlansResponse(data);
      const current = data.plans.find(p => p.id === data.currentPlanId) ?? null;
      setCurrentPlan(current);
    } catch {
      // Fail silently — plan context is optional for page rendering
    } finally {
      setIsLoaded(true);
    }
  }, [isAuthenticated, activeTenant]);

  useEffect(() => {
    loadPlans();
  }, [loadPlans]);

  const hasEntitlement = useCallback((key: string): boolean => {
    if (!currentPlan) return false;
    const ent = currentPlan.entitlements[key];
    if (!ent) return false;
    if (ent.type === 'bool') return ent.boolValue;
    if (ent.type === 'numeric') return ent.numericValue > 0;
    return false;
  }, [currentPlan]);

  const getNumericEntitlement = useCallback((key: string): number => {
    if (!currentPlan) return 0;
    const ent = currentPlan.entitlements[key];
    if (!ent || ent.type !== 'numeric') return 0;
    return ent.numericValue;
  }, [currentPlan]);

  const getEntitlement = useCallback((key: string): EntitlementValue | null => {
    if (!currentPlan) return null;
    return currentPlan.entitlements[key] ?? null;
  }, [currentPlan]);

  const tierName = (currentPlan?.name ?? 'free').toLowerCase();
  const isPaid = currentPlan != null && currentPlan.monthlyPriceCents > 0;

  return (
    <PlanContext.Provider value={{
      currentPlan,
      plans,
      plansResponse,
      isLoaded,
      hasEntitlement,
      getNumericEntitlement,
      getEntitlement,
      tierName,
      isPaid,
      refresh: loadPlans,
    }}>
      {children}
    </PlanContext.Provider>
  );
}

export function usePlan() {
  const context = useContext(PlanContext);
  if (!context) {
    throw new Error('usePlan must be used within a PlanProvider');
  }
  return context;
}
