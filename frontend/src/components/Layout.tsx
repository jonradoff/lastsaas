import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, Users, Settings, LogOut, Shield, ChevronDown, Bell, CreditCard, Zap,
  FileText, Image, Globe, Star, Heart, BookOpen, MessageCircle, HelpCircle, Sun, Moon, Megaphone,
} from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { useTenant } from '../contexts/TenantContext';
import { useBranding } from '../contexts/BrandingContext';
import { useTheme } from '../contexts/ThemeContext';
import { usePlan } from '../contexts/PlanContext';
import { messagesApi, plansApi, bundlesApi, announcementsApi } from '../api/client';
import ImpersonationBanner from './ImpersonationBanner';
import { useState, useRef, useEffect } from 'react';
import type { LucideIcon } from 'lucide-react';

const iconMap: Record<string, LucideIcon> = {
  LayoutDashboard, Users, Settings, CreditCard, FileText, Image, Globe, Shield, Zap, Star, Heart, BookOpen, MessageCircle, HelpCircle,
};

export default function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, isAuthenticated, logout, memberships } = useAuth();
  const { activeTenant, setActiveTenant } = useTenant();
  const { branding } = useBranding();
  const { resolvedTheme, setTheme } = useTheme();
  const { tierName, isLoaded: planLoaded } = usePlan();
  const [showTenantMenu, setShowTenantMenu] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);
  const [showCredits, setShowCredits] = useState(false);
  const [tenantCredits, setTenantCredits] = useState(0);
  const [hasBundles, setHasBundles] = useState(false);
  const [showTeam, setShowTeam] = useState(true);
  const [latestAnnouncement, setLatestAnnouncement] = useState<{ id: string; title: string } | null>(null);
  const [dismissedAnnouncement, setDismissedAnnouncement] = useState<string>(() =>
    localStorage.getItem('dismissed_announcement') || ''
  );
  const menuRef = useRef<HTMLDivElement>(null);

  const isActive = (path: string) => location.pathname === path;

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([
        messagesApi.unreadCount(),
        plansApi.list(),
        bundlesApi.list(),
        announcementsApi.list(),
      ]).then(([messagesResult, plansResult, bundlesResult, announcementsResult]) => {
        if (messagesResult.status === 'fulfilled') {
          setUnreadCount(messagesResult.value.count);
        }
        if (plansResult.status === 'fulfilled') {
          const data = plansResult.value;
          const hasCredits = data.plans.some((p: { usageCreditsPerMonth: number; bonusCredits: number }) => p.usageCreditsPerMonth > 0 || p.bonusCredits > 0);
          setShowCredits(hasCredits);
          setTenantCredits(data.tenantSubscriptionCredits + data.tenantPurchasedCredits);
          setShowTeam(data.maxPlanUserLimit !== 1);
        }
        if (bundlesResult.status === 'fulfilled') {
          setHasBundles(bundlesResult.value.bundles.length > 0);
        }
        if (announcementsResult.status === 'fulfilled') {
          const anns = announcementsResult.value.announcements;
          if (anns.length > 0) {
            setLatestAnnouncement({ id: anns[0].id, title: anns[0].title });
          }
        }
      });
    }
  }, [isAuthenticated]);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowTenantMenu(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  // Build nav items from branding config or fallback to defaults
  const TIER_ORDER: Record<string, number> = { free: 0, pro: 1, max: 2, agency: 3 };
  const currentLevel = planLoaded ? (TIER_ORDER[tierName] ?? 0) : 0;
  const showBonuses = currentLevel >= 1; // Pro+

  const defaultNavItems = [
    { path: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    ...(showTeam ? [{ path: '/team', icon: Users, label: 'Team' }] : []),
    ...(showBonuses ? [{ path: '/bonuses', icon: Star, label: 'Bonuses' }] : []),
    { path: '/plan', icon: CreditCard, label: 'Plan' },
    { path: '/settings', icon: Settings, label: 'Settings' },
  ];

  const navItems = branding.navItems.length > 0
    ? branding.navItems
        .filter(item => item.visible)
        .filter(item => {
          // Hide team item if showTeam is false
          if (item.id === 'team' && !showTeam) return false;
          return true;
        })
        .sort((a, b) => a.sortOrder - b.sortOrder)
        .map(item => ({
          path: item.target,
          icon: iconMap[item.icon] || FileText,
          label: item.label,
        }))
    : defaultNavItems;

  // Resolve logo display
  const appName = branding.appName || 'MCPLens';
  const logoMode = branding.logoMode || 'text';
  const logoUrl = branding.logoUrl;

  const isImpersonating = localStorage.getItem('mcplens_impersonating') === 'true';

  return (
    <div className="min-h-screen bg-dark-950">
      <ImpersonationBanner />
      {/* Header */}
      <header className={`sticky ${isImpersonating ? 'top-10' : 'top-0'} z-40 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800`}>
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Logo + Nav */}
            <div className="flex items-center gap-6">
              <Link to="/dashboard" className="flex items-center gap-2">
                {(logoMode === 'image' || logoMode === 'both') && logoUrl ? (
                  <img src={logoUrl} alt={appName} className="h-8 w-8 rounded-lg object-contain" />
                ) : (
                  <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                    <span className="text-white font-bold text-sm">{appName.slice(0, 2).toUpperCase()}</span>
                  </div>
                )}
                {(logoMode === 'text' || logoMode === 'both') && (
                  <span className="font-semibold text-dark-100 hidden sm:block">{appName}</span>
                )}
              </Link>

              {isAuthenticated && (
                <nav className="hidden md:flex items-center gap-1">
                  {navItems.map((item) => (
                    <Link
                      key={item.path}
                      to={item.path}
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
                        isActive(item.path)
                          ? 'bg-primary-500/20 text-primary-400'
                          : 'text-dark-400 hover:text-dark-100 hover:bg-dark-800/50'
                      }`}
                    >
                      <item.icon className="w-4 h-4" />
                      <span>{item.label}</span>
                    </Link>
                  ))}
                  {memberships.some(m => m.isRoot) && (
                    <Link
                      to="/last"
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors ${
                        location.pathname.startsWith('/last')
                          ? 'bg-accent-purple/20 text-accent-purple'
                          : 'text-dark-400 hover:text-dark-100 hover:bg-dark-800/50'
                      }`}
                    >
                      <Shield className="w-4 h-4" />
                      <span>Admin</span>
                    </Link>
                  )}
                </nav>
              )}
            </div>

            {/* Right side */}
            {isAuthenticated && (
              <div className="flex items-center gap-4">
                {/* Tenant Switcher */}
                {memberships.length > 1 && (
                  <div className="relative" ref={menuRef}>
                    <button
                      onClick={() => setShowTenantMenu(!showTenantMenu)}
                      className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-dark-800 border border-dark-700 text-sm text-dark-300 hover:text-dark-100 transition-colors"
                    >
                      <span className="max-w-[120px] truncate">{activeTenant?.tenantName}</span>
                      <ChevronDown className="w-3.5 h-3.5" />
                    </button>
                    {showTenantMenu && (
                      <div className="absolute right-0 mt-2 w-56 bg-dark-800 border border-dark-700 rounded-xl shadow-xl py-1 z-50">
                        {memberships.map((m) => (
                          <button
                            key={m.tenantId}
                            onClick={() => {
                              setActiveTenant(m);
                              setShowTenantMenu(false);
                            }}
                            className={`w-full text-left px-4 py-2.5 text-sm transition-colors ${
                              m.tenantId === activeTenant?.tenantId
                                ? 'bg-primary-500/10 text-primary-400'
                                : 'text-dark-300 hover:bg-dark-700 hover:text-dark-100'
                            }`}
                          >
                            <div className="flex items-center justify-between">
                              <span className="truncate">{m.tenantName}</span>
                              <span className="text-xs text-dark-500 capitalize">{m.role}</span>
                            </div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Credits indicator */}
                {showCredits && (
                  <button
                    onClick={() => navigate(hasBundles ? '/buy-credits' : '/plan')}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-dark-800 border border-dark-700 text-sm text-dark-300 hover:text-dark-100 hover:border-primary-500/30 transition-colors"
                    title="Usage credits"
                  >
                    <Zap className="w-4 h-4 text-primary-400" />
                    <span className="font-medium">{tenantCredits.toLocaleString()}</span>
                  </button>
                )}

                {/* Theme toggle */}
                <button
                  onClick={() => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark')}
                  className="text-dark-400 hover:text-dark-100 transition-colors"
                  title={`Switch to ${resolvedTheme === 'dark' ? 'light' : 'dark'} mode`}
                  aria-label="Toggle theme"
                >
                  {resolvedTheme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
                </button>

                {/* Messages */}
                <Link
                  to="/messages"
                  className="relative text-dark-400 hover:text-dark-100 transition-colors"
                  aria-label="Messages"
                >
                  <Bell className="w-5 h-5" />
                  {unreadCount > 0 && (
                    <span className="absolute -top-1.5 -right-1.5 bg-primary-500 text-white text-[10px] font-medium rounded-full w-4 h-4 flex items-center justify-center">
                      {unreadCount}
                    </span>
                  )}
                </Link>

                {/* User info + Logout */}
                <span className="text-sm text-dark-400 hidden sm:block">{user?.displayName}</span>
                <button
                  onClick={handleLogout}
                  className="flex items-center gap-2 text-dark-400 hover:text-dark-100 transition-colors"
                  aria-label="Sign out"
                >
                  <LogOut className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Announcement Banner */}
      {latestAnnouncement && latestAnnouncement.id !== dismissedAnnouncement && (
        <div className="bg-primary-500/10 border-b border-primary-500/20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2 flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm">
              <Megaphone className="w-4 h-4 text-primary-400 flex-shrink-0" />
              <span className="text-primary-200">{latestAnnouncement.title}</span>
            </div>
            <button
              onClick={() => {
                setDismissedAnnouncement(latestAnnouncement.id);
                localStorage.setItem('dismissed_announcement', latestAnnouncement.id);
              }}
              className="text-xs text-dark-400 hover:text-dark-100 transition-colors ml-4"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet context={{ setUnreadCount, showTeam }} />
      </main>
    </div>
  );
}
