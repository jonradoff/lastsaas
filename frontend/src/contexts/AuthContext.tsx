import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { authApi, setAuthToken } from '../api/client';
import type { User, MembershipInfo, AuthResponse, MFARequiredResponse } from '../types';

interface MFAPendingState {
  mfaToken: string;
}

interface AuthContextType {
  user: User | null;
  memberships: MembershipInfo[];
  isAuthenticated: boolean;
  isLoading: boolean;
  mfaPending: MFAPendingState | null;
  login: (email: string, password: string) => Promise<void>;
  register: (data: { email: string; password: string; displayName: string; invitationToken?: string }) => Promise<void>;
  loginWithTokens: (accessToken: string, refreshToken: string) => Promise<void>;
  completeMfaChallenge: (mfaToken: string, code: string) => Promise<void>;
  clearMfaPending: () => void;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

// Architecture note: JWTs are stored in localStorage rather than httpOnly cookies.
// This is a deliberate trade-off: localStorage is vulnerable to XSS but enables simple
// multi-tab support and API key-style auth headers. The app mitigates XSS risk through
// React's built-in escaping, strict CSP headers, and validated/sanitized user inputs.
// httpOnly cookies would require CSRF protection and complicate the SPA architecture.
const ACCESS_TOKEN_KEY = 'lastsaas_access_token';
const REFRESH_TOKEN_KEY = 'lastsaas_refresh_token';

function isMfaRequired(data: AuthResponse | MFARequiredResponse): data is MFARequiredResponse {
  return 'mfaRequired' in data && data.mfaRequired === true;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [memberships, setMemberships] = useState<MembershipInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [mfaPending, setMfaPending] = useState<MFAPendingState | null>(null);

  const isAuthenticated = !!user;

  const clearAuth = useCallback(() => {
    setUser(null);
    setMemberships([]);
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    localStorage.removeItem('lastsaas_impersonating');
    setAuthToken(null);
  }, []);

  const refreshUser = useCallback(async () => {
    try {
      const data = await authApi.getMe();
      setUser(data.user);
      setMemberships(data.memberships);
    } catch {
      clearAuth();
    }
  }, [clearAuth]);

  const handleAuthResponse = useCallback((data: AuthResponse) => {
    localStorage.setItem(ACCESS_TOKEN_KEY, data.accessToken);
    localStorage.setItem(REFRESH_TOKEN_KEY, data.refreshToken);
    setAuthToken(data.accessToken);
    setUser(data.user);
    setMemberships(data.memberships);
    setMfaPending(null);
  }, []);

  const loginWithTokens = useCallback(async (accessToken: string, refreshToken: string) => {
    localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
    setAuthToken(accessToken);
    await refreshUser();
  }, [refreshUser]);

  const login = useCallback(async (email: string, password: string) => {
    const data = await authApi.login({ email, password });
    if (isMfaRequired(data)) {
      setMfaPending({ mfaToken: data.mfaToken });
      return;
    }
    handleAuthResponse(data as AuthResponse);
  }, [handleAuthResponse]);

  const completeMfaChallenge = useCallback(async (mfaToken: string, code: string) => {
    const data = await authApi.mfaChallenge(mfaToken, code);
    handleAuthResponse(data);
  }, [handleAuthResponse]);

  const clearMfaPending = useCallback(() => {
    setMfaPending(null);
  }, []);

  const register = useCallback(async (data: { email: string; password: string; displayName: string; invitationToken?: string }) => {
    const res = await authApi.register(data);
    localStorage.setItem(ACCESS_TOKEN_KEY, res.accessToken);
    localStorage.setItem(REFRESH_TOKEN_KEY, res.refreshToken);
    setAuthToken(res.accessToken);
    setUser(res.user);
    setMemberships(res.memberships);
  }, []);

  const logout = useCallback(async () => {
    const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
    try {
      if (refreshToken) {
        await authApi.logout(refreshToken);
      }
    } catch {
      // ignore logout errors
    }
    clearAuth();
  }, [clearAuth]);

  // Restore session on mount
  useEffect(() => {
    const token = localStorage.getItem(ACCESS_TOKEN_KEY);
    if (token) {
      setAuthToken(token);
      refreshUser().finally(() => setIsLoading(false));
    } else {
      setIsLoading(false);
    }
  }, [refreshUser]);

  return (
    <AuthContext.Provider value={{ user, memberships, isAuthenticated, isLoading, mfaPending, login, register, loginWithTokens, completeMfaChallenge, clearMfaPending, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
