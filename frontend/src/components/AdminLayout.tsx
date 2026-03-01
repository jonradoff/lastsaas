import { useEffect, useState } from 'react';
import { Link, Outlet, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Users,
  Building2,
  Mail,
  FileText,
  CreditCard,
  DollarSign,
  Activity,
  Settings,
  Info,
  ArrowLeft,
  Shield,
  Code2,
  Paintbrush,
  Tag,
  Megaphone,
  UserPlus,
} from 'lucide-react';
import { useTenant } from '../contexts/TenantContext';
import { messagesApi } from '../api/client';

import { Navigate } from 'react-router-dom';

export default function AdminLayout() {
  const location = useLocation();
  const { isRootTenant } = useTenant();
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    messagesApi.unreadCount()
      .then((data) => setUnreadCount(data.count))
      .catch(() => { /* non-critical: badge just won't show */ });
  }, []);

  if (!isRootTenant) {
    return <Navigate to="/dashboard" replace />;
  }

  const isActive = (path: string) => location.pathname === path;

  const navItems = [
    { path: '/last', icon: LayoutDashboard, label: 'Dashboard' },
    { path: '/last/messages', icon: Mail, label: 'Messages' },
    { path: '/last/users', icon: Users, label: 'Users' },
    { path: '/last/tenants', icon: Building2, label: 'Tenants' },
    { path: '/last/members', icon: UserPlus, label: 'Root Members' },
    { path: '/last/plans', icon: CreditCard, label: 'Plans' },
    { path: '/last/financial', icon: DollarSign, label: 'Financial' },
    { path: '/last/promotions', icon: Tag, label: 'Promotions' },
    { path: '/last/announcements', icon: Megaphone, label: 'Announcements' },
    { path: '/last/health', icon: Activity, label: 'System Health' },
    { path: '/last/logs', icon: FileText, label: 'Logs' },
    { path: '/last/config', icon: Settings, label: 'Configuration' },
    { path: '/last/branding', icon: Paintbrush, label: 'Branding' },
    { path: '/last/api', icon: Code2, label: 'API' },
    { path: '/last/about', icon: Info, label: 'About' },
  ];

  return (
    <div className="min-h-screen bg-dark-950">
      {/* Admin Header */}
      <header className="sticky top-0 z-50 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-4">
              <Link to="/dashboard" className="flex items-center gap-2 text-dark-400 hover:text-white transition-colors">
                <ArrowLeft className="w-4 h-4" />
                <span className="text-sm">Back to App</span>
              </Link>
              <div className="h-6 w-px bg-dark-700" />
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                  <Shield className="w-4 h-4 text-white" />
                </div>
                <span className="font-semibold text-white">Admin</span>
              </div>
            </div>
          </div>
        </div>
      </header>

      <div className="flex">
        {/* Sidebar */}
        <aside className="w-64 min-h-[calc(100vh-4rem)] bg-dark-900/50 border-r border-dark-800 p-4">
          <nav className="space-y-1">
            {navItems.map((item) => (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center gap-3 px-4 py-3 rounded-lg transition-colors ${
                  isActive(item.path)
                    ? 'bg-primary-500/20 text-primary-400'
                    : 'text-dark-400 hover:text-white hover:bg-dark-800/50'
                }`}
              >
                <item.icon className="w-5 h-5" />
                <span className="font-medium">{item.label}</span>
                {item.label === 'Messages' && unreadCount > 0 && (
                  <span className="ml-auto text-xs bg-primary-500 text-white rounded-full px-2 py-0.5 min-w-[20px] text-center">
                    {unreadCount}
                  </span>
                )}
              </Link>
            ))}
          </nav>
        </aside>

        {/* Main Content */}
        <main className="flex-1 p-8">
          <div className="max-w-6xl mx-auto">
            <Outlet context={{ setUnreadCount }} />
          </div>
        </main>
      </div>
    </div>
  );
}
