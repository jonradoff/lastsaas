import { useState } from 'react';
import { Database, CreditCard, Mail, Plug, HelpCircle, X, ExternalLink, LogIn, Github, Fingerprint, KeyRound, Send } from 'lucide-react';
import type { IntegrationCheck } from '../../../types';
import { adminApi } from '../../../api/client';

const ICONS: Record<string, typeof Database> = {
  mongodb: Database,
  stripe: CreditCard,
  resend: Mail,
  google_oauth: LogIn,
  github_oauth: Github,
  microsoft_oauth: LogIn,
  webauthn: Fingerprint,
  saml_sso: KeyRound,
};

const LABELS: Record<string, string> = {
  mongodb: 'MongoDB',
  stripe: 'Stripe',
  resend: 'Resend',
  google_oauth: 'Google Login',
  github_oauth: 'GitHub Login',
  microsoft_oauth: 'Microsoft Login',
  webauthn: 'Passkeys (WebAuthn)',
  saml_sso: 'SSO / SAML',
};

const CALLS_24H_LABEL: Record<string, string> = {
  stripe: 'API calls',
  resend: 'Emails',
};

function getSetupHelp(origin: string): Record<string, { title: string; steps: string[]; links: { label: string; url: string }[] }> {
  const hostname = new URL(origin).hostname;
  return {
    stripe: {
      title: 'Stripe Setup',
      steps: [
        'Create a Stripe account at stripe.com if you don\'t have one.',
        'Go to Dashboard > Developers > API keys and copy your Secret key and Publishable key.',
        `Set up a webhook endpoint at Dashboard > Developers > Webhooks pointing to ${origin}/api/billing/webhook. Choose "Your Account" (not Connected accounts) and the latest API version.`,
        'Subscribe the webhook to these events: checkout.session.completed, invoice.paid, invoice.payment_failed, customer.subscription.updated, customer.subscription.deleted',
        'Copy the webhook signing secret.',
        'Enable the Customer Portal at Dashboard > Settings > Billing > Portal.',
        'Add the following environment variables. For local development, create a .env file in the project root (this is auto-loaded on startup). For Fly.io, use: flyctl secrets set STRIPE_SECRET_KEY=sk_... STRIPE_PUBLISHABLE_KEY=pk_... STRIPE_WEBHOOK_SECRET=whsec_...',
        'The YAML config files (config/dev.yaml, config/prod.yaml) already reference these variables via ${STRIPE_SECRET_KEY} syntax. Do not edit the YAML files — just set the environment variables.',
        'Redeploy or restart the server for the changes to take effect.',
      ],
      links: [
        { label: 'Stripe Dashboard', url: 'https://dashboard.stripe.com' },
        { label: 'API Keys', url: 'https://dashboard.stripe.com/apikeys' },
        { label: 'Webhooks', url: 'https://dashboard.stripe.com/webhooks' },
        { label: 'Customer Portal Settings', url: 'https://dashboard.stripe.com/settings/billing/portal' },
      ],
    },
    resend: {
      title: 'Resend Setup',
      steps: [
        'Create a Resend account at resend.com.',
        'Go to API Keys and create a new key with sending access.',
        'Add and verify your sending domain under Domains (or use the default onboarding domain for testing).',
        `Add the following environment variables. For local development, create a .env file in the project root (this is auto-loaded on startup). For Fly.io, use: flyctl secrets set RESEND_API_KEY=re_... FROM_EMAIL=noreply@${hostname} FROM_NAME=YourApp`,
        'The YAML config files (config/dev.yaml, config/prod.yaml) already reference these variables via ${RESEND_API_KEY} syntax. Do not edit the YAML files — just set the environment variables.',
        'Redeploy or restart the server for the changes to take effect.',
      ],
      links: [
        { label: 'Resend Dashboard', url: 'https://resend.com' },
        { label: 'Resend API Keys', url: 'https://resend.com/api-keys' },
        { label: 'Resend Domains', url: 'https://resend.com/domains' },
      ],
    },
    google_oauth: {
      title: 'Google Login Setup',
      steps: [
        'Go to the Google Cloud Console and create a project (or select an existing one).',
        'Navigate to APIs & Services > OAuth consent screen. Choose "External" user type, fill in the app name and support email, and publish the consent screen.',
        'Go to APIs & Services > Credentials and click "Create Credentials" > "OAuth client ID". Select "Web application" as the application type.',
        `Add your frontend URL (${origin}) to "Authorized JavaScript origins".`,
        `Add your backend callback URL to "Authorized redirect URIs": ${origin}/api/auth/google/callback`,
        'Copy the Client ID and Client Secret from the credentials page.',
        `Add the following environment variables. For local development, create a .env file in the project root (this is auto-loaded on startup). For Fly.io, use: flyctl secrets set GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... GOOGLE_REDIRECT_URL=${origin}/api/auth/google/callback`,
        'The YAML config files (config/dev.yaml, config/prod.yaml) already reference these variables via ${GOOGLE_CLIENT_ID} syntax. Do not edit the YAML files — just set the environment variables.',
        'Redeploy or restart the server for the changes to take effect.',
      ],
      links: [
        { label: 'Google Cloud Console', url: 'https://console.cloud.google.com' },
        { label: 'OAuth Consent Screen', url: 'https://console.cloud.google.com/apis/credentials/consent' },
        { label: 'Credentials', url: 'https://console.cloud.google.com/apis/credentials' },
      ],
    },
    github_oauth: {
      title: 'GitHub Login Setup',
      steps: [
        'Go to GitHub Settings > Developer settings > OAuth Apps and click "New OAuth App".',
        `Set the Homepage URL to ${origin}`,
        `Set the Authorization callback URL to ${origin}/api/auth/github/callback`,
        'Copy the Client ID and generate a Client Secret.',
        `Add the following environment variables: GITHUB_CLIENT_ID=... GITHUB_CLIENT_SECRET=... GITHUB_REDIRECT_URL=${origin}/api/auth/github/callback`,
        'Redeploy or restart the server for the changes to take effect.',
      ],
      links: [
        { label: 'GitHub OAuth Apps', url: 'https://github.com/settings/developers' },
      ],
    },
    microsoft_oauth: {
      title: 'Microsoft Login Setup',
      steps: [
        'Go to the Azure Portal > App registrations and click "New registration".',
        `Set the Redirect URI to ${origin}/api/auth/microsoft/callback (Web platform).`,
        'Under "Certificates & secrets", create a new client secret and copy its value.',
        'Copy the Application (client) ID from the Overview page.',
        `Add the following environment variables: MICROSOFT_CLIENT_ID=... MICROSOFT_CLIENT_SECRET=... MICROSOFT_REDIRECT_URL=${origin}/api/auth/microsoft/callback`,
        'Redeploy or restart the server for the changes to take effect.',
      ],
      links: [
        { label: 'Azure App Registrations', url: 'https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade' },
      ],
    },
  };
}

function timeAgo(dateStr: string): string {
  if (!dateStr) return 'Never';
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ago`;
}

function SetupModal({ name, setupHelp, onClose }: { name: string; setupHelp: Record<string, { title: string; steps: string[]; links: { label: string; url: string }[] }>; onClose: () => void }) {
  const help = setupHelp[name];
  if (!help) return null;

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-lg max-h-[80vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <h3 className="text-lg font-semibold text-dark-50">{help.title}</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-dark-50 transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <ol className="space-y-3">
            {help.steps.map((step, i) => (
              <li key={i} className="flex gap-3 text-sm text-dark-300">
                <span className="flex-shrink-0 w-6 h-6 rounded-full bg-primary-500/20 text-primary-400 flex items-center justify-center text-xs font-medium">
                  {i + 1}
                </span>
                <span className="pt-0.5">{step}</span>
              </li>
            ))}
          </ol>
          {help.links.length > 0 && (
            <div className="pt-4 border-t border-dark-800">
              <p className="text-xs font-medium text-dark-400 uppercase tracking-wide mb-2">Useful Links</p>
              <div className="flex flex-wrap gap-2">
                {help.links.map((link) => (
                  <a
                    key={link.url}
                    href={link.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-dark-800 hover:bg-dark-700 border border-dark-700 rounded-lg text-xs text-dark-300 hover:text-dark-50 transition-colors"
                  >
                    <ExternalLink className="w-3 h-3" />
                    {link.label}
                  </a>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function SendTestEmailModal({ onClose }: { onClose: () => void }) {
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ success: boolean; error?: string } | null>(null);

  const handleSend = async () => {
    if (!email.trim()) return;
    setLoading(true);
    setResult(null);
    try {
      const res = await adminApi.sendTestEmail(email.trim());
      setResult({ success: !!res.success, error: res.error });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to send test email';
      setResult({ success: false, error: msg });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-md" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <h3 className="text-lg font-semibold text-dark-50">Send Test Email</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-dark-50 transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Recipient email</label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && !loading) handleSend(); }}
              placeholder="you@example.com"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-dark-50 text-sm placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
              autoFocus
              disabled={loading}
            />
          </div>
          <button
            onClick={handleSend}
            disabled={loading || !email.trim()}
            className="w-full inline-flex items-center justify-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-60 disabled:cursor-not-allowed rounded-lg text-sm font-medium text-white transition-colors"
          >
            <Send className="w-4 h-4" />
            {loading ? 'Sending...' : 'Send Test Email'}
          </button>
          {result && (
            <div className={`p-3 rounded-lg text-sm ${
              result.success
                ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400'
                : 'bg-red-500/10 border border-red-500/20 text-red-400'
            }`}>
              {result.success
                ? 'Test email sent successfully. Check your inbox.'
                : `Failed to send: ${result.error}`}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function IntegrationsPanel({ integrations }: { integrations: IntegrationCheck[] }) {
  const [helpFor, setHelpFor] = useState<string | null>(null);
  const [showTestEmail, setShowTestEmail] = useState(false);
  const setupHelp = getSetupHelp(window.location.origin);

  if (integrations.length === 0) return null;

  return (
    <div id="integrations">
      <div className="flex items-center gap-2 mb-4">
        <Plug className="w-4 h-4 text-dark-400" />
        <h2 className="text-sm font-medium text-dark-400 uppercase tracking-wide">Integrations</h2>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {integrations.map((check) => {
          const Icon = ICONS[check.name] || Plug;
          const label = LABELS[check.name] || check.name;
          const isHealthy = check.status === 'healthy';
          const isNotConfigured = check.status === 'not_configured';
          const isUnhealthy = check.status === 'unhealthy';
          const hasHelp = isNotConfigured && setupHelp[check.name];
          const callsLabel = CALLS_24H_LABEL[check.name];
          const canTestEmail = check.name === 'resend' && isHealthy;

          return (
            <div
              key={check.name}
              className={`bg-dark-900/50 backdrop-blur-sm border rounded-2xl p-5 ${
                isUnhealthy ? 'border-red-500/30' :
                isNotConfigured ? 'border-yellow-500/20' :
                'border-dark-800'
              }`}
            >
              <div className="flex items-start gap-3">
                <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                  isHealthy ? 'bg-emerald-500/20' :
                  isUnhealthy ? 'bg-red-500/20' :
                  'bg-yellow-500/10'
                }`}>
                  <Icon className={`w-5 h-5 ${
                    isHealthy ? 'text-emerald-400' :
                    isUnhealthy ? 'text-red-400' :
                    'text-yellow-500/60'
                  }`} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-dark-50">{label}</span>
                    <span className={`inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full ${
                      isHealthy ? 'bg-emerald-500/20 text-emerald-400' :
                      isUnhealthy ? 'bg-red-500/20 text-red-400' :
                      'bg-yellow-500/10 text-yellow-500/80'
                    }`}>
                      <span className={`w-1.5 h-1.5 rounded-full ${
                        isHealthy ? 'bg-emerald-400' :
                        isUnhealthy ? 'bg-red-400' :
                        'bg-yellow-500/60'
                      }`} />
                      {isNotConfigured ? 'Not Configured' : isHealthy ? 'Healthy' : 'Unhealthy'}
                    </span>
                  </div>
                  {!isNotConfigured && (
                    <p className={`text-xs mt-1 ${isUnhealthy ? 'text-red-400/80' : 'text-dark-500'}`}>
                      {check.message}
                    </p>
                  )}
                </div>
              </div>
              {!isNotConfigured && check.lastCheck && (
                <div className="mt-3 space-y-1 text-xs text-dark-400">
                  <div className="flex justify-between">
                    <span>Last check</span>
                    <span className="text-dark-300">{timeAgo(check.lastCheck)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Response</span>
                    <span className="text-dark-300">{check.responseMs}ms</span>
                  </div>
                  {callsLabel && (
                    <div className="flex justify-between">
                      <span>{callsLabel} (24h)</span>
                      <span className="text-dark-300">{check.calls24h.toLocaleString()}</span>
                    </div>
                  )}
                </div>
              )}
              {hasHelp && (
                <button
                  onClick={() => setHelpFor(check.name)}
                  className="mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 bg-yellow-500/10 hover:bg-yellow-500/20 border border-yellow-500/20 rounded-lg text-xs text-yellow-400 transition-colors"
                >
                  <HelpCircle className="w-3.5 h-3.5" />
                  Setup Help
                </button>
              )}
              {canTestEmail && (
                <button
                  onClick={() => setShowTestEmail(true)}
                  className="mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/20 rounded-lg text-xs text-emerald-400 transition-colors"
                >
                  <Send className="w-3.5 h-3.5" />
                  Send Test Email
                </button>
              )}
            </div>
          );
        })}
      </div>

      {helpFor && <SetupModal name={helpFor} setupHelp={setupHelp} onClose={() => setHelpFor(null)} />}
      {showTestEmail && <SendTestEmailModal onClose={() => setShowTestEmail(false)} />}
    </div>
  );
}
