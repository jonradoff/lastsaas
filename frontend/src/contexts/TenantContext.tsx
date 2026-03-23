import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { setTenantHeader } from '../api/client';
import { useAuth } from './AuthContext';
import type { MembershipInfo } from '../types';

interface TenantContextType {
  activeTenant: MembershipInfo | null;
  setActiveTenant: (membership: MembershipInfo) => void;
  isRootTenant: boolean;
  role: 'owner' | 'admin' | 'user' | null;
}

const TenantContext = createContext<TenantContextType | null>(null);

const ACTIVE_TENANT_KEY = 'mcplens_active_tenant';

export function TenantProvider({ children }: { children: ReactNode }) {
  const { memberships, isAuthenticated } = useAuth();
  const [activeTenant, setActiveTenantState] = useState<MembershipInfo | null>(null);

  const setActiveTenant = useCallback((membership: MembershipInfo) => {
    setActiveTenantState(membership);
    setTenantHeader(membership.tenantId);
    localStorage.setItem(ACTIVE_TENANT_KEY, membership.tenantId);
  }, []);

  // Restore or auto-select tenant when memberships change
  useEffect(() => {
    if (!isAuthenticated || memberships.length === 0) {
      setActiveTenantState(null);
      setTenantHeader(null);
      return;
    }

    const savedTenantId = localStorage.getItem(ACTIVE_TENANT_KEY);
    const saved = savedTenantId ? memberships.find(m => m.tenantId === savedTenantId) : null;

    if (saved) {
      setActiveTenant(saved);
    } else {
      // Default to first membership (root tenant if available)
      const root = memberships.find(m => m.isRoot);
      setActiveTenant(root || memberships[0]);
    }
  }, [memberships, isAuthenticated, setActiveTenant]);

  const isRootTenant = activeTenant?.isRoot ?? false;
  const role = activeTenant?.role ?? null;

  return (
    <TenantContext.Provider value={{ activeTenant, setActiveTenant, isRootTenant, role }}>
      {children}
    </TenantContext.Provider>
  );
}

export function useTenant() {
  const context = useContext(TenantContext);
  if (!context) {
    throw new Error('useTenant must be used within a TenantProvider');
  }
  return context;
}
