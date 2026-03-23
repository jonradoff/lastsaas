import { lazy, Suspense, useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'sonner';
import { AuthProvider } from './contexts/AuthContext';
import { TenantProvider } from './contexts/TenantContext';
import { BrandingProvider } from './contexts/BrandingContext';
import { ThemeProvider } from './contexts/ThemeContext';
import Layout from './components/Layout';
import AdminLayout from './components/AdminLayout';
import ProtectedRoute from './components/ProtectedRoute';
import BrandingThemeInjector from './components/BrandingThemeInjector';
import LoadingSpinner from './components/LoadingSpinner';
import ErrorBoundary from './components/ErrorBoundary';
import { bootstrapApi } from './api/client';

// Auth pages (lazy — not needed until user navigates to them)
const LoginPage = lazy(() => import('./pages/auth/LoginPage'));
const SignupPage = lazy(() => import('./pages/auth/SignupPage'));
const VerifyEmailPage = lazy(() => import('./pages/auth/VerifyEmailPage'));
const ForgotPasswordPage = lazy(() => import('./pages/auth/ForgotPasswordPage'));
const ResetPasswordPage = lazy(() => import('./pages/auth/ResetPasswordPage'));
const AuthCallbackPage = lazy(() => import('./pages/auth/AuthCallbackPage'));
const MFAChallengePage = lazy(() => import('./pages/auth/MFAChallengePage'));
const MagicLinkVerifyPage = lazy(() => import('./pages/auth/MagicLinkVerifyPage'));
import BootstrapPage from './pages/BootstrapPage';

// App pages (eager — core experience)
import DashboardPage from './pages/app/DashboardPage';
import StoreDetailPage from './pages/app/StoreDetailPage';
import TeamPage from './pages/app/TeamPage';
import SettingsPage from './pages/app/SettingsPage';
import PlanPage from './pages/app/PlanPage';
import BuyCreditsPage from './pages/app/BuyCreditsPage';
import BillingSuccessPage from './pages/app/BillingSuccessPage';
import BillingCancelPage from './pages/app/BillingCancelPage';
import TestEntitlementsPage from './pages/app/TestEntitlementsPage';
import ActivityPage from './pages/app/ActivityPage';
import OnboardingPage from './pages/app/OnboardingPage';
import BonusesPage from './pages/app/BonusesPage';

// Admin pages (lazy — only loaded by root tenant admins)
const AdminDashboardPage = lazy(() => import('./pages/admin/DashboardPage'));
const AdminMessagesPage = lazy(() => import('./pages/admin/MessagesPage'));
const AdminUsersPage = lazy(() => import('./pages/admin/UsersPage'));
const AdminTenantsPage = lazy(() => import('./pages/admin/TenantsPage'));
const AdminLogsPage = lazy(() => import('./pages/admin/LogsPage'));
const AdminConfigPage = lazy(() => import('./pages/admin/ConfigPage'));
const AdminUserProfilePage = lazy(() => import('./pages/admin/UserProfilePage'));
const AdminAboutPage = lazy(() => import('./pages/admin/AboutPage'));
const AdminPlansPage = lazy(() => import('./pages/admin/PlansPage'));
const AdminTenantProfilePage = lazy(() => import('./pages/admin/TenantProfilePage'));
const AdminHealthPage = lazy(() => import('./pages/admin/HealthPage'));
const AdminFinancialPage = lazy(() => import('./pages/admin/FinancialPage'));
const AdminAPIPage = lazy(() => import('./pages/admin/APIPage'));
const AdminBrandingPage = lazy(() => import('./pages/admin/BrandingPage'));
const AdminPromotionsPage = lazy(() => import('./pages/admin/PromotionsPage'));
const AdminAnnouncementsPage = lazy(() => import('./pages/admin/AnnouncementsPage'));
const AdminRootMembersPage = lazy(() => import('./pages/admin/RootMembersPage'));
const AdminPMPage = lazy(() => import('./pages/admin/PMPage'));

// Public pages
import LandingPage from './pages/public/LandingPage';
import CustomPage from './pages/public/CustomPage';
import ScanPage from './pages/public/ScanPage';
import ScanResultPage from './pages/public/ScanResultPage';
import TermsPage from './pages/public/TermsPage';
import PrivacyPage from './pages/public/PrivacyPage';

function LazyFallback() {
  return (
    <div className="flex items-center justify-center py-20">
      <LoadingSpinner size="lg" />
    </div>
  );
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60,
      retry: 1,
    },
  },
});

function RegisterRedirect() {
  const { search } = useLocation();
  return <Navigate to={`/signup${search}`} replace />;
}

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
}

function BootstrapGuard({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<'loading' | 'initialized' | 'needs-setup'>('loading');

  useEffect(() => {
    bootstrapApi.status()
      .then((data) => setStatus(data.initialized ? 'initialized' : 'needs-setup'))
      .catch(() => setStatus('initialized')); // If bootstrap endpoint fails, assume initialized
  }, []);

  if (status === 'loading') {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (status === 'needs-setup') {
    return (
      <BrowserRouter>
        <Routes>
          <Route path="/setup" element={<BootstrapPage />} />
          <Route path="*" element={<Navigate to="/setup" replace />} />
        </Routes>
      </BrowserRouter>
    );
  }

  return <>{children}</>;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BootstrapGuard>
        <BrandingProvider>
          <AuthProvider>
            <ThemeProvider>
              <TenantProvider>
                <BrowserRouter>
                  <ScrollToTop />
                  <BrandingThemeInjector />
                  <ErrorBoundary>
                  <Routes>
                    {/* Public landing page */}
                    <Route path="/" element={<LandingPage />} />

                    {/* Public scanner pages (no auth required) */}
                    <Route path="/scan" element={<ScanPage />} />
                    <Route path="/scan/:domain" element={<ScanResultPage />} />

                    {/* Legal pages */}
                    <Route path="/terms" element={<TermsPage />} />
                    <Route path="/privacy" element={<PrivacyPage />} />

                    {/* /register alias for /signup (preserves query params) */}
                    <Route path="/register" element={<RegisterRedirect />} />

                    {/* Public custom pages */}
                    <Route path="/p/:slug" element={<CustomPage />} />

                    {/* Public auth routes (lazy-loaded) */}
                    <Route path="/login" element={<Suspense fallback={<LazyFallback />}><LoginPage /></Suspense>} />
                    <Route path="/signup" element={<Suspense fallback={<LazyFallback />}><SignupPage /></Suspense>} />
                    <Route path="/verify-email" element={<Suspense fallback={<LazyFallback />}><VerifyEmailPage /></Suspense>} />
                    <Route path="/forgot-password" element={<Suspense fallback={<LazyFallback />}><ForgotPasswordPage /></Suspense>} />
                    <Route path="/reset-password" element={<Suspense fallback={<LazyFallback />}><ResetPasswordPage /></Suspense>} />
                    <Route path="/auth/callback" element={<Suspense fallback={<LazyFallback />}><AuthCallbackPage /></Suspense>} />
                    <Route path="/auth/mfa" element={<Suspense fallback={<LazyFallback />}><MFAChallengePage /></Suspense>} />
                    <Route path="/auth/magic-link" element={<Suspense fallback={<LazyFallback />}><MagicLinkVerifyPage /></Suspense>} />

                    {/* Protected app routes */}
                    <Route element={<ProtectedRoute />}>
                      {/* Onboarding (no layout) */}
                      <Route path="/onboarding" element={<OnboardingPage />} />

                      <Route element={<Layout />}>
                        <Route path="/dashboard" element={<DashboardPage />} />
                        <Route path="/dashboard/store/:id" element={<StoreDetailPage />} />
                        <Route path="/team" element={<TeamPage />} />
                        <Route path="/plan" element={<PlanPage />} />
                        <Route path="/buy-credits" element={<BuyCreditsPage />} />
                        <Route path="/billing/success" element={<BillingSuccessPage />} />
                        <Route path="/billing/cancel" element={<BillingCancelPage />} />
                        <Route path="/settings" element={<SettingsPage />} />
                        <Route path="/activity" element={<ActivityPage />} />
                        <Route path="/bonuses" element={<BonusesPage />} />
                        <Route path="/test-entitlements" element={<TestEntitlementsPage />} />
                        <Route path="/messages" element={<Suspense fallback={<LazyFallback />}><AdminMessagesPage /></Suspense>} />
                      </Route>

                      {/* Admin routes (root tenant only, enforced by AdminLayout) */}
                      <Route path="/last" element={<AdminLayout />}>
                        <Route index element={<Suspense fallback={<LazyFallback />}><AdminDashboardPage /></Suspense>} />
                        <Route path="messages" element={<Suspense fallback={<LazyFallback />}><AdminMessagesPage /></Suspense>} />
                        <Route path="users" element={<Suspense fallback={<LazyFallback />}><AdminUsersPage /></Suspense>} />
                        <Route path="users/:userId" element={<Suspense fallback={<LazyFallback />}><AdminUserProfilePage /></Suspense>} />
                        <Route path="tenants" element={<Suspense fallback={<LazyFallback />}><AdminTenantsPage /></Suspense>} />
                        <Route path="tenants/:tenantId" element={<Suspense fallback={<LazyFallback />}><AdminTenantProfilePage /></Suspense>} />
                        <Route path="members" element={<Suspense fallback={<LazyFallback />}><AdminRootMembersPage /></Suspense>} />
                        <Route path="plans" element={<Suspense fallback={<LazyFallback />}><AdminPlansPage /></Suspense>} />
                        <Route path="financial" element={<Suspense fallback={<LazyFallback />}><AdminFinancialPage /></Suspense>} />
                        <Route path="pm" element={<Suspense fallback={<LazyFallback />}><AdminPMPage /></Suspense>} />
                        <Route path="promotions" element={<Suspense fallback={<LazyFallback />}><AdminPromotionsPage /></Suspense>} />
                        <Route path="announcements" element={<Suspense fallback={<LazyFallback />}><AdminAnnouncementsPage /></Suspense>} />
                        <Route path="health" element={<Suspense fallback={<LazyFallback />}><AdminHealthPage /></Suspense>} />
                        <Route path="logs" element={<Suspense fallback={<LazyFallback />}><AdminLogsPage /></Suspense>} />
                        <Route path="config" element={<Suspense fallback={<LazyFallback />}><AdminConfigPage /></Suspense>} />
                        <Route path="api" element={<Suspense fallback={<LazyFallback />}><AdminAPIPage /></Suspense>} />
                        <Route path="branding" element={<Suspense fallback={<LazyFallback />}><AdminBrandingPage /></Suspense>} />
                        <Route path="about" element={<Suspense fallback={<LazyFallback />}><AdminAboutPage /></Suspense>} />
                      </Route>
                    </Route>

                    {/* Fallback */}
                    <Route path="*" element={<Navigate to="/dashboard" replace />} />
                  </Routes>
                  </ErrorBoundary>
                  <Toaster
                    position="top-right"
                    toastOptions={{
                      style: {
                        background: 'var(--color-dark-900)',
                        border: '1px solid var(--color-dark-700)',
                        color: 'var(--color-dark-100)',
                      },
                    }}
                  />
                </BrowserRouter>
              </TenantProvider>
            </ThemeProvider>
          </AuthProvider>
        </BrandingProvider>
      </BootstrapGuard>
    </QueryClientProvider>
  );
}
