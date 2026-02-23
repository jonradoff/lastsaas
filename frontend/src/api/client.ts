import axios from 'axios';
import type { AuthResponse, MFARequiredResponse, AuthProviders, ActiveSession, ActivityLogEntry, PasskeyCredential, ImpersonationResponse, TenantMember, TenantDetail, TenantListItem, UserListItem, Message, AboutInfo, SystemLog, ConfigVar, UserDetail, UserMembershipDetail, DeletePreflightResponse, Plan, EntitlementKeyInfo, PublicPlansResponse, CreditBundle, SystemNode, SystemMetric, FinancialTransaction, DailyMetricPoint, IntegrationCheck, APIKey, Webhook, WebhookDelivery, WebhookEventTypeInfo, BrandingConfig, MediaItem, CustomPage } from '../types';

const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
});

// Auth header management
export function setAuthToken(token: string | null) {
  if (token) {
    api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
  } else {
    delete api.defaults.headers.common['Authorization'];
  }
}

export function setTenantHeader(tenantId: string | null) {
  if (tenantId) {
    api.defaults.headers.common['X-Tenant-ID'] = tenantId;
  } else {
    delete api.defaults.headers.common['X-Tenant-ID'];
  }
}

// 401 interceptor
api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 401 && !error.config?.url?.includes('/auth/login') && !error.config?.url?.includes('/auth/refresh')) {
      localStorage.removeItem('lastsaas_access_token');
      localStorage.removeItem('lastsaas_refresh_token');
      delete api.defaults.headers.common['Authorization'];
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// 503 interceptor (system not initialized)
api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 503 && error.response?.data?.redirect === '/setup') {
      window.location.href = '/setup';
    }
    return Promise.reject(error);
  }
);

// --- Bootstrap ---
export const bootstrapApi = {
  status: () => api.get<{ initialized: boolean }>('/bootstrap/status').then(r => r.data),
};

// --- Auth ---
export const authApi = {
  register: (data: { email: string; password: string; displayName: string; invitationToken?: string }) =>
    api.post<AuthResponse>('/auth/register', data).then(r => r.data),
  login: (data: { email: string; password: string }) =>
    api.post<AuthResponse | MFARequiredResponse>('/auth/login', data).then(r => r.data),
  logout: (refreshToken?: string) =>
    api.post('/auth/logout', { refreshToken }),
  refresh: (refreshToken: string) =>
    api.post<AuthResponse>('/auth/refresh', { refreshToken }).then(r => r.data),
  getMe: () =>
    api.get<{ user: import('../types').User; memberships: import('../types').MembershipInfo[] }>('/auth/me').then(r => r.data),
  verifyEmail: (token: string) =>
    api.post('/auth/verify-email', { token }).then(r => r.data),
  resendVerification: (email: string) =>
    api.post('/auth/resend-verification', { email }).then(r => r.data),
  forgotPassword: (email: string) =>
    api.post('/auth/forgot-password', { email }).then(r => r.data),
  resetPassword: (token: string, newPassword: string) =>
    api.post('/auth/reset-password', { token, newPassword }).then(r => r.data),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.post('/auth/change-password', { currentPassword, newPassword }).then(r => r.data),
  acceptInvitation: (token: string) =>
    api.post('/auth/accept-invitation', { token }).then(r => r.data),

  // Auth providers discovery
  getProviders: () =>
    api.get<AuthProviders>('/auth/providers').then(r => r.data),

  // MFA / TOTP
  mfaSetup: () =>
    api.post<{ secret: string; qrCodeUrl: string }>('/auth/mfa/setup').then(r => r.data),
  mfaVerifySetup: (code: string) =>
    api.post<{ recoveryCodes: string[] }>('/auth/mfa/verify-setup', { code }).then(r => r.data),
  mfaDisable: (code: string) =>
    api.post('/auth/mfa/disable', { code }).then(r => r.data),
  mfaChallenge: (mfaToken: string, code: string) =>
    api.post<AuthResponse>('/auth/mfa/challenge', { mfaToken, code }).then(r => r.data),
  mfaRegenerateCodes: (code: string) =>
    api.post<{ recoveryCodes: string[] }>('/auth/mfa/regenerate-codes', { code }).then(r => r.data),

  // Magic Link
  requestMagicLink: (email: string) =>
    api.post('/auth/magic-link', { email }).then(r => r.data),
  verifyMagicLink: (token: string) =>
    api.post<AuthResponse | MFARequiredResponse>('/auth/magic-link/verify', { token }).then(r => r.data),

  // Passkeys / WebAuthn
  passkeyRegisterBegin: () =>
    api.post('/auth/passkeys/register/begin').then(r => r.data),
  passkeyRegisterFinish: (data: { name: string; credential: unknown }) =>
    api.post('/auth/passkeys/register/finish', data).then(r => r.data),
  passkeyLoginBegin: () =>
    api.post('/auth/passkeys/login/begin').then(r => r.data),
  passkeyLoginFinish: (credential: unknown) =>
    api.post<AuthResponse>('/auth/passkeys/login/finish', { credential }).then(r => r.data),
  listPasskeys: () =>
    api.get<{ passkeys: PasskeyCredential[] }>('/auth/passkeys').then(r => r.data),
  deletePasskey: (id: string) =>
    api.delete(`/auth/passkeys/${id}`).then(r => r.data),

  // Sessions
  listSessions: () =>
    api.get<{ sessions: ActiveSession[] }>('/auth/sessions').then(r => r.data),
  revokeSession: (id: string) =>
    api.delete(`/auth/sessions/${id}`).then(r => r.data),
  revokeAllSessions: () =>
    api.delete('/auth/sessions').then(r => r.data),

  // Preferences
  updatePreferences: (data: { themePreference?: string }) =>
    api.patch('/auth/preferences', data).then(r => r.data),

  // Onboarding
  completeOnboarding: () =>
    api.post('/auth/complete-onboarding').then(r => r.data),
};

// --- Tenant ---
export const tenantApi = {
  listMembers: () =>
    api.get<{ members: TenantMember[] }>('/tenant/members').then(r => r.data),
  inviteMember: (email: string, role: string) =>
    api.post('/tenant/members/invite', { email, role }).then(r => r.data),
  removeMember: (userId: string) =>
    api.delete(`/tenant/members/${userId}`).then(r => r.data),
  changeRole: (userId: string, role: string) =>
    api.patch(`/tenant/members/${userId}/role`, { role }).then(r => r.data),
  transferOwnership: (userId: string) =>
    api.post(`/tenant/members/${userId}/transfer-ownership`).then(r => r.data),
  getActivity: (params?: { page?: number; perPage?: number; action?: string }) =>
    api.get<{ logs: ActivityLogEntry[]; total: number }>('/tenant/activity', { params }).then(r => r.data),
  updateSettings: (data: { name?: string }) =>
    api.patch('/tenant/settings', data).then(r => r.data),
};

// --- Messages ---
export const messagesApi = {
  list: () =>
    api.get<{ messages: Message[] }>('/messages').then(r => r.data),
  unreadCount: () =>
    api.get<{ count: number }>('/messages/unread-count').then(r => r.data),
  markRead: (id: string) =>
    api.patch(`/messages/${id}/read`).then(r => r.data),
};

// --- Admin ---
export const adminApi = {
  getAbout: () =>
    api.get<AboutInfo>('/admin/about').then(r => r.data),
  getDashboard: () =>
    api.get<{ users: number; tenants: number; health: { healthy: boolean; issues: string[] } }>('/admin/dashboard').then(r => r.data),
  listTenants: (params?: { page?: number; limit?: number; search?: string; sort?: string }) =>
    api.get<{ tenants: TenantListItem[]; total: number; page: number; limit: number }>('/admin/tenants', { params }).then(r => r.data),
  getTenant: (id: string) =>
    api.get<{ tenant: TenantDetail; members: TenantMember[] }>(`/admin/tenants/${id}`).then(r => r.data),
  updateTenant: (id: string, data: { name?: string; billingWaived?: boolean; subscriptionCredits?: number; purchasedCredits?: number }) =>
    api.put(`/admin/tenants/${id}`, data).then(r => r.data),
  updateTenantStatus: (id: string, isActive: boolean) =>
    api.patch(`/admin/tenants/${id}/status`, { isActive }).then(r => r.data),
  listUsers: (params?: { page?: number; limit?: number; search?: string; sort?: string }) =>
    api.get<{ users: UserListItem[]; total: number; page: number; limit: number }>('/admin/users', { params }).then(r => r.data),
  updateUserStatus: (id: string, isActive: boolean) =>
    api.patch(`/admin/users/${id}/status`, { isActive }).then(r => r.data),
  listLogs: (params?: { page?: number; perPage?: number; severity?: string; search?: string; userId?: string }) =>
    api.get<{ logs: SystemLog[]; total: number }>('/admin/logs', { params }).then(r => r.data),
  listConfig: () =>
    api.get<{ configs: ConfigVar[] }>('/admin/config').then(r => r.data),
  getConfig: (name: string) =>
    api.get<ConfigVar>(`/admin/config/${name}`).then(r => r.data),
  updateConfig: (name: string, value: string, opts?: { description?: string; options?: string }) =>
    api.put<ConfigVar>(`/admin/config/${name}`, { value, ...opts }).then(r => r.data),
  createConfig: (data: { name: string; description: string; type: string; value: string; options?: string }) =>
    api.post<ConfigVar>('/admin/config', data).then(r => r.data),
  deleteConfig: (name: string) =>
    api.delete(`/admin/config/${name}`).then(r => r.data),
  getUser: (id: string) =>
    api.get<{ user: UserDetail; memberships: UserMembershipDetail[] }>(`/admin/users/${id}`).then(r => r.data),
  updateUser: (id: string, data: { email?: string; displayName?: string }) =>
    api.put(`/admin/users/${id}`, data).then(r => r.data),
  updateUserRole: (userId: string, tenantId: string, role: string) =>
    api.patch(`/admin/users/${userId}/role/${tenantId}`, { role }).then(r => r.data),
  preflightDeleteUser: (id: string) =>
    api.get<DeletePreflightResponse>(`/admin/users/${id}/preflight-delete`).then(r => r.data),
  deleteUser: (id: string, data?: { replacementOwners?: Record<string, string>; confirmTenantDeletions?: string[] }) =>
    api.delete(`/admin/users/${id}`, { data }).then(r => r.data),
  listPlans: () =>
    api.get<{ plans: Plan[] }>('/admin/plans').then(r => r.data),
  getPlan: (id: string) =>
    api.get<Plan>(`/admin/plans/${id}`).then(r => r.data),
  createPlan: (data: Omit<Plan, 'id' | 'isSystem' | 'isArchived' | 'createdAt' | 'updatedAt' | 'subscriberCount'>) =>
    api.post<Plan>('/admin/plans', data).then(r => r.data),
  updatePlan: (id: string, data: Omit<Plan, 'id' | 'isSystem' | 'isArchived' | 'createdAt' | 'updatedAt' | 'subscriberCount'>) =>
    api.put<Plan>(`/admin/plans/${id}`, data).then(r => r.data),
  deletePlan: (id: string) =>
    api.delete(`/admin/plans/${id}`).then(r => r.data),
  archivePlan: (id: string) =>
    api.post(`/admin/plans/${id}/archive`).then(r => r.data),
  unarchivePlan: (id: string) =>
    api.post(`/admin/plans/${id}/unarchive`).then(r => r.data),
  listEntitlementKeys: () =>
    api.get<{ keys: EntitlementKeyInfo[] }>('/admin/entitlement-keys').then(r => r.data),
  assignTenantPlan: (tenantId: string, planId?: string | null, billingWaived?: boolean) => {
    const body: Record<string, unknown> = {};
    if (planId !== undefined) body.planId = planId || '';
    if (billingWaived !== undefined) body.billingWaived = billingWaived;
    return api.patch(`/admin/tenants/${tenantId}/plan`, body).then(r => r.data);
  },
  listBundles: () =>
    api.get<{ bundles: CreditBundle[] }>('/admin/credit-bundles').then(r => r.data),
  createBundle: (data: Omit<CreditBundle, 'id' | 'createdAt' | 'updatedAt'>) =>
    api.post<CreditBundle>('/admin/credit-bundles', data).then(r => r.data),
  updateBundle: (id: string, data: Omit<CreditBundle, 'id' | 'createdAt' | 'updatedAt'>) =>
    api.put<CreditBundle>(`/admin/credit-bundles/${id}`, data).then(r => r.data),
  deleteBundle: (id: string) =>
    api.delete(`/admin/credit-bundles/${id}`).then(r => r.data),
  listHealthNodes: () =>
    api.get<{ nodes: SystemNode[] }>('/admin/health/nodes').then(r => r.data),
  getHealthMetrics: (params?: { node?: string; range?: string }) =>
    api.get<{ metrics: SystemMetric[]; from: string; to: string }>('/admin/health/metrics', { params }).then(r => r.data),
  getHealthCurrent: () =>
    api.get<{ metrics: SystemMetric[] }>('/admin/health/current').then(r => r.data),
  getHealthIntegrations: () =>
    api.get<{ integrations: IntegrationCheck[] }>('/admin/health/integrations').then(r => r.data),
  listFinancialTransactions: (params?: { page?: number; perPage?: number; tenantId?: string; search?: string }) =>
    api.get<{ transactions: FinancialTransaction[]; total: number; page: number; perPage: number }>('/admin/financial/transactions', { params }).then(r => r.data),
  getFinancialMetrics: (params?: { range?: string; metric?: string }) =>
    api.get<{ data: DailyMetricPoint[] }>('/admin/financial/metrics', { params }).then(r => r.data),
  adminCancelSubscription: (tenantId: string, immediate: boolean) =>
    api.post(`/admin/tenants/${tenantId}/cancel-subscription`, { immediate }).then(r => r.data),
  adminUpdateSubscription: (tenantId: string, data: { currentPeriodEnd?: string }) =>
    api.patch(`/admin/tenants/${tenantId}/subscription`, data).then(r => r.data),

  // API Keys
  listAPIKeys: () =>
    api.get<{ apiKeys: APIKey[] }>('/admin/api-keys').then(r => r.data),
  createAPIKey: (data: { name: string; authority: string }) =>
    api.post<{ apiKey: APIKey; rawKey: string }>('/admin/api-keys', data).then(r => r.data),
  deleteAPIKey: (id: string) =>
    api.delete(`/admin/api-keys/${id}`).then(r => r.data),

  // Webhooks
  listWebhooks: () =>
    api.get<{ webhooks: Webhook[] }>('/admin/webhooks').then(r => r.data),
  getWebhook: (id: string) =>
    api.get<{ webhook: Webhook; secret: string; deliveries: WebhookDelivery[] }>(`/admin/webhooks/${id}`).then(r => r.data),
  createWebhook: (data: { name: string; description: string; url: string; events: string[] }) =>
    api.post<{ webhook: Webhook; secret: string }>('/admin/webhooks', data).then(r => r.data),
  updateWebhook: (id: string, data: { name: string; description: string; url: string; events: string[] }) =>
    api.put<{ webhook: Webhook }>(`/admin/webhooks/${id}`, data).then(r => r.data),
  deleteWebhook: (id: string) =>
    api.delete(`/admin/webhooks/${id}`).then(r => r.data),
  testWebhook: (id: string) =>
    api.post<{ delivery: WebhookDelivery }>(`/admin/webhooks/${id}/test`).then(r => r.data),
  regenerateWebhookSecret: (id: string) =>
    api.post<{ secret: string; secretPreview: string }>(`/admin/webhooks/${id}/regenerate-secret`).then(r => r.data),
  listWebhookEventTypes: () =>
    api.get<{ eventTypes: WebhookEventTypeInfo[] }>('/admin/webhooks/event-types').then(r => r.data),

  // Impersonation
  impersonateUser: (userId: string) =>
    api.post<ImpersonationResponse>(`/admin/users/${userId}/impersonate`).then(r => r.data),
};

// --- Plans (public, authenticated) ---
export const plansApi = {
  list: () =>
    api.get<PublicPlansResponse>('/plans').then(r => r.data),
};

// --- Credit Bundles (public, authenticated) ---
export const bundlesApi = {
  list: () =>
    api.get<{ bundles: CreditBundle[] }>('/credit-bundles').then(r => r.data),
};

// --- Billing ---
export const billingApi = {
  checkout: (data: { planId?: string; bundleId?: string; billingInterval?: string; seatQuantity?: number; removeBillingWaiver?: boolean }) =>
    api.post<{ checkoutUrl?: string; waived?: boolean }>('/billing/checkout', data).then(r => r.data),
  portal: () =>
    api.post<{ portalUrl: string }>('/billing/portal').then(r => r.data),
  listTransactions: (params?: { page?: number; perPage?: number }) =>
    api.get<{ transactions: FinancialTransaction[]; total: number; page: number; perPage: number }>('/billing/transactions', { params }).then(r => r.data),
  getInvoice: (id: string) =>
    api.get<{ transaction: FinancialTransaction; tenant: { name: string } }>(`/billing/transactions/${id}/invoice`).then(r => r.data),
  getInvoicePDF: (id: string) =>
    api.get(`/billing/transactions/${id}/invoice/pdf`, { responseType: 'blob' }).then(r => r.data),
  cancel: () =>
    api.post<{ message: string; currentPeriodEnd?: string }>('/billing/cancel').then(r => r.data),
  getConfig: () =>
    api.get<{ publishableKey: string }>('/billing/config').then(r => r.data),
};

// --- Branding (public, no auth) ---
export const brandingApi = {
  get: () =>
    api.get<BrandingConfig>('/branding').then(r => r.data),
  getPublicPages: () =>
    api.get<{ pages: CustomPage[] }>('/branding/pages').then(r => r.data),
  getPublicPage: (slug: string) =>
    api.get<CustomPage>(`/branding/page/${slug}`).then(r => r.data),
};

// --- Branding Admin ---
export const brandingAdminApi = {
  update: (data: Partial<BrandingConfig>) =>
    api.put('/admin/branding', data).then(r => r.data),
  uploadAsset: (key: 'logo' | 'favicon', file: File) => {
    const form = new FormData();
    form.append('key', key);
    form.append('file', file);
    return api.post('/admin/branding/asset', form, { headers: { 'Content-Type': 'multipart/form-data' } }).then(r => r.data);
  },
  deleteAsset: (key: 'logo' | 'favicon') =>
    api.delete(`/admin/branding/asset/${key}`).then(r => r.data),
  listMedia: () =>
    api.get<{ media: MediaItem[] }>('/admin/branding/media').then(r => r.data),
  uploadMedia: (file: File) => {
    const form = new FormData();
    form.append('file', file);
    return api.post<MediaItem>('/admin/branding/media', form, { headers: { 'Content-Type': 'multipart/form-data' } }).then(r => r.data);
  },
  deleteMedia: (key: string) =>
    api.delete(`/admin/branding/media/${key}`).then(r => r.data),
  listPages: () =>
    api.get<{ pages: CustomPage[] }>('/admin/branding/pages').then(r => r.data),
  createPage: (data: Partial<CustomPage>) =>
    api.post<CustomPage>('/admin/branding/pages', data).then(r => r.data),
  updatePage: (id: string, data: Partial<CustomPage>) =>
    api.put(`/admin/branding/pages/${id}`, data).then(r => r.data),
  deletePage: (id: string) =>
    api.delete(`/admin/branding/pages/${id}`).then(r => r.data),
};

export default api;
