import { useState, useMemo } from 'react';
import { Settings } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import { useBranding } from '../../contexts/BrandingContext';
import ProfileTab from './settings/ProfileTab';
import SecurityTab from './settings/SecurityTab';
import SessionsTab from './settings/SessionsTab';
import BillingTab from './settings/BillingTab';

export default function SettingsPage() {
  const { user } = useAuth();
  const { branding } = useBranding();
  const passkeysEnabled = branding?.authProviders?.passkeys ?? false;
  const mfaConfigEnabled = branding?.authProviders?.mfa ?? false;
  const showMfaSection = mfaConfigEnabled || user?.totpEnabled;
  const showSecurityTab = passkeysEnabled || showMfaSection;

  const tabs = useMemo(() => [
    { key: 'profile' as const, label: 'Profile' },
    ...(showSecurityTab ? [{ key: 'security' as const, label: 'Security' }] : []),
    { key: 'sessions' as const, label: 'Sessions' },
    { key: 'billing' as const, label: 'Billing' },
  ], [showSecurityTab]);

  const [tab, setTab] = useState<'profile' | 'security' | 'sessions' | 'billing'>('profile');

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
          <Settings className="w-7 h-7 text-primary-400" />
          Settings
        </h1>
        <p className="text-dark-400 mt-1">Manage your account</p>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-1 mb-6 bg-dark-900/50 border border-dark-800 rounded-xl p-1 max-w-md" role="tablist">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            role="tab"
            aria-selected={tab === t.key}
            className={`flex-1 px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              tab === t.key ? 'bg-dark-700 text-dark-50' : 'text-dark-400 hover:text-dark-300'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'profile' && <ProfileTab />}
      {tab === 'security' && <SecurityTab />}
      {tab === 'sessions' && <SessionsTab />}
      {tab === 'billing' && <BillingTab />}
    </div>
  );
}
