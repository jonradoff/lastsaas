import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { LogIn, Github, Mail, KeyRound, Fingerprint } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import { useBranding } from '../../contexts/BrandingContext';
import { authApi } from '../../api/client';

function GoogleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" />
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
    </svg>
  );
}

function MicrosoftIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M11.4 24H0V12.6h11.4V24zM24 24H12.6V12.6H24V24zM11.4 11.4H0V0h11.4v11.4zM24 11.4H12.6V0H24v11.4z" />
    </svg>
  );
}

export default function LoginPage() {
  const navigate = useNavigate();
  const { login, mfaPending, completeMfaChallenge, clearMfaPending } = useAuth();
  const { branding } = useBranding();
  const [form, setForm] = useState({ email: '', password: '' });
  const [mfaCode, setMfaCode] = useState('');
  const [magicLinkEmail, setMagicLinkEmail] = useState('');
  const [magicLinkSent, setMagicLinkSent] = useState(false);
  const [showMagicLink, setShowMagicLink] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const providers = branding.authProviders;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(form.email, form.password);
      navigate('/dashboard');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg || 'Invalid email or password');
    } finally {
      setLoading(false);
    }
  };

  const handleMfaSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!mfaPending) return;
    setError('');
    setLoading(true);
    try {
      await completeMfaChallenge(mfaPending.mfaToken, mfaCode);
      navigate('/dashboard');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg || 'Invalid verification code');
    } finally {
      setLoading(false);
    }
  };

  const handleMagicLink = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await authApi.requestMagicLink(magicLinkEmail);
      setMagicLinkSent(true);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg || 'Failed to send magic link');
    } finally {
      setLoading(false);
    }
  };

  const handlePasskeyLogin = async () => {
    setError('');
    setLoading(true);
    try {
      const options = await authApi.passkeyLoginBegin();
      const credential = await navigator.credentials.get({ publicKey: options });
      if (!credential) throw new Error('No credential returned');
      const data = await authApi.passkeyLoginFinish(credential);
      localStorage.setItem('mcplens_access_token', data.accessToken);
      localStorage.setItem('mcplens_refresh_token', data.refreshToken);
      navigate('/dashboard');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
        || (err as Error)?.message || 'Passkey authentication failed';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  const heading = branding.loginHeading || 'Welcome back';
  const subtext = branding.loginSubtext || 'Sign in to your account';
  const logoUrl = branding.logoUrl;

  const hasOAuth = providers && (providers.google || providers.github || providers.microsoft);

  // MFA challenge screen
  if (mfaPending) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
        <div className="w-full max-w-md">
          <div className="text-center mb-8">
            <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center mx-auto mb-4">
              <KeyRound className="w-7 h-7 text-white" />
            </div>
            <h1 className="text-2xl font-bold text-white">Two-Factor Authentication</h1>
            <p className="text-dark-400 mt-2">Enter the code from your authenticator app or a recovery code</p>
          </div>

          <form onSubmit={handleMfaSubmit} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 space-y-4">
            {error && (
              <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">
                {error}
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1.5">Verification Code</label>
              <input
                type="text"
                required
                autoFocus
                autoComplete="one-time-code"
                inputMode="numeric"
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors text-center text-lg tracking-widest"
                placeholder="000000"
                maxLength={32}
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {loading ? 'Verifying...' : 'Verify'}
            </button>

            <button
              type="button"
              onClick={clearMfaPending}
              className="w-full text-sm text-dark-400 hover:text-dark-300 transition-colors"
            >
              Back to login
            </button>
          </form>
        </div>
      </div>
    );
  }

  // Magic link form
  if (showMagicLink) {
    if (magicLinkSent) {
      return (
        <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
          <div className="w-full max-w-md">
            <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center">
              <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-green-500 to-green-600 flex items-center justify-center mx-auto mb-4">
                <Mail className="w-7 h-7 text-white" />
              </div>
              <h1 className="text-xl font-bold text-white mb-2">Check your email</h1>
              <p className="text-dark-400 mb-6">We sent a sign-in link to <span className="text-white">{magicLinkEmail}</span></p>
              <button
                onClick={() => { setShowMagicLink(false); setMagicLinkSent(false); }}
                className="text-sm text-primary-400 hover:text-primary-300 transition-colors"
              >
                Back to login
              </button>
            </div>
          </div>
        </div>
      );
    }

    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
        <div className="w-full max-w-md">
          <div className="text-center mb-8">
            <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center mx-auto mb-4">
              <Mail className="w-7 h-7 text-white" />
            </div>
            <h1 className="text-2xl font-bold text-white">Sign in with email</h1>
            <p className="text-dark-400 mt-2">We'll send you a sign-in link</p>
          </div>

          <form onSubmit={handleMagicLink} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 space-y-4">
            {error && (
              <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">
                {error}
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1.5">Email</label>
              <input
                type="email"
                required
                autoFocus
                value={magicLinkEmail}
                onChange={(e) => setMagicLinkEmail(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                placeholder="you@example.com"
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {loading ? 'Sending...' : 'Send sign-in link'}
            </button>

            <button
              type="button"
              onClick={() => setShowMagicLink(false)}
              className="w-full text-sm text-dark-400 hover:text-dark-300 transition-colors"
            >
              Back to login
            </button>
          </form>
        </div>
      </div>
    );
  }

  // Main login form
  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          {logoUrl ? (
            <img src={logoUrl} alt={branding.appName} className="h-14 mx-auto mb-4 object-contain" />
          ) : (
            <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center mx-auto mb-4">
              <LogIn className="w-7 h-7 text-white" />
            </div>
          )}
          <h1 className="text-2xl font-bold text-white">{heading}</h1>
          <p className="text-dark-400 mt-2">{subtext}</p>
        </div>

        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 space-y-4">
          {error && (
            <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">
              {error}
            </div>
          )}

          {/* OAuth buttons */}
          {hasOAuth && (
            <>
              <div className="space-y-2">
                {providers?.google && (
                  <a
                    href="/api/auth/google"
                    className="flex items-center justify-center gap-3 w-full py-2.5 px-4 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all"
                  >
                    <GoogleIcon className="w-5 h-5" />
                    Continue with Google
                  </a>
                )}
                {providers?.github && (
                  <a
                    href="/api/auth/github"
                    className="flex items-center justify-center gap-3 w-full py-2.5 px-4 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all"
                  >
                    <Github className="w-5 h-5" />
                    Continue with GitHub
                  </a>
                )}
                {providers?.microsoft && (
                  <a
                    href="/api/auth/microsoft"
                    className="flex items-center justify-center gap-3 w-full py-2.5 px-4 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all"
                  >
                    <MicrosoftIcon className="w-4 h-4" />
                    Continue with Microsoft
                  </a>
                )}
              </div>

              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-dark-700" />
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-3 bg-dark-900/50 text-dark-500">or</span>
                </div>
              </div>
            </>
          )}

          {/* Passkey button */}
          {providers?.passkeys && (
            <button
              type="button"
              onClick={handlePasskeyLogin}
              disabled={loading}
              className="flex items-center justify-center gap-3 w-full py-2.5 px-4 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 disabled:opacity-50 transition-all"
            >
              <Fingerprint className="w-5 h-5" />
              Sign in with passkey
            </button>
          )}

          {/* Password form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1.5">Email</label>
              <input
                type="email"
                required
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                placeholder="you@example.com"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-dark-300 mb-1.5">Password</label>
              <input
                type="password"
                required
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                placeholder="Your password"
              />
            </div>

            <div className="flex items-center justify-end">
              <Link to="/forgot-password" className="text-sm text-primary-400 hover:text-primary-300 transition-colors">
                Forgot password?
              </Link>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {loading ? 'Signing in...' : 'Sign In'}
            </button>
          </form>

          {/* Magic link option */}
          {providers?.magicLink && (
            <button
              type="button"
              onClick={() => setShowMagicLink(true)}
              className="flex items-center justify-center gap-2 w-full text-sm text-dark-400 hover:text-dark-300 transition-colors"
            >
              <Mail className="w-4 h-4" />
              Sign in with email link
            </button>
          )}

          <div className="text-center text-sm text-dark-400">
            Don't have an account?{' '}
            <Link to="/signup" className="text-primary-400 hover:text-primary-300 transition-colors">
              Sign up
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
