import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { CheckCircle, ChevronRight, User, Users, CreditCard } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import { authApi, tenantApi } from '../../api/client';

type Step = 'profile' | 'team' | 'complete';

export default function OnboardingPage() {
  const navigate = useNavigate();
  const { user, refreshUser } = useAuth();
  const [step, setStep] = useState<Step>('profile');
  const [displayName, setDisplayName] = useState(user?.displayName || '');
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteError, setInviteError] = useState('');
  const [invitedEmails, setInvitedEmails] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const steps: { key: Step; label: string; icon: typeof User }[] = [
    { key: 'profile', label: 'Profile', icon: User },
    { key: 'team', label: 'Team', icon: Users },
    { key: 'complete', label: 'Done', icon: CreditCard },
  ];

  const currentIndex = steps.findIndex(s => s.key === step);

  const handleProfileNext = () => {
    setStep('team');
  };

  const handleInvite = async () => {
    if (!inviteEmail.trim()) return;
    setInviteError('');
    try {
      await tenantApi.inviteMember(inviteEmail.trim(), 'user');
      setInvitedEmails(prev => [...prev, inviteEmail.trim()]);
      setInviteEmail('');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setInviteError(msg || 'Failed to invite');
    }
  };

  const handleTeamNext = () => {
    setStep('complete');
  };

  const handleComplete = async () => {
    setLoading(true);
    try {
      await authApi.completeOnboarding();
      await refreshUser();
      navigate('/dashboard');
    } catch {
      navigate('/dashboard');
    }
  };

  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
      <div className="w-full max-w-lg">
        {/* Stepper */}
        <div className="flex items-center justify-center gap-2 mb-8">
          {steps.map((s, i) => (
            <div key={s.key} className="flex items-center gap-2">
              <div className={`flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
                i < currentIndex ? 'bg-accent-emerald/20 text-accent-emerald' :
                i === currentIndex ? 'bg-primary-500/20 text-primary-400' :
                'bg-dark-800 text-dark-500'
              }`}>
                {i < currentIndex ? (
                  <CheckCircle className="w-4 h-4" />
                ) : (
                  <s.icon className="w-4 h-4" />
                )}
                <span className="hidden sm:inline">{s.label}</span>
              </div>
              {i < steps.length - 1 && <ChevronRight className="w-4 h-4 text-dark-600" />}
            </div>
          ))}
        </div>

        {/* Profile Step */}
        {step === 'profile' && (
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <h2 className="text-xl font-bold text-dark-50 mb-2">Welcome! Let's set up your profile</h2>
            <p className="text-dark-400 text-sm mb-6">Confirm your display name</p>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1.5">Display Name</label>
                <input
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-dark-50 placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                  placeholder="Your name"
                />
              </div>

              <button
                onClick={handleProfileNext}
                disabled={!displayName.trim()}
                className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 transition-all"
              >
                Continue
              </button>
            </div>
          </div>
        )}

        {/* Team Step */}
        {step === 'team' && (
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <h2 className="text-xl font-bold text-dark-50 mb-2">Invite your team</h2>
            <p className="text-dark-400 text-sm mb-6">Optionally invite team members to join your organization</p>

            <div className="space-y-4">
              {invitedEmails.length > 0 && (
                <div className="space-y-1">
                  {invitedEmails.map((email) => (
                    <div key={email} className="flex items-center gap-2 px-3 py-2 bg-dark-800/50 rounded-lg">
                      <CheckCircle className="w-4 h-4 text-accent-emerald" />
                      <span className="text-sm text-dark-50">{email}</span>
                    </div>
                  ))}
                </div>
              )}

              {inviteError && (
                <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{inviteError}</div>
              )}

              <div className="flex gap-2">
                <input
                  type="email"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleInvite(); }}
                  className="flex-1 px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-dark-50 placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors text-sm"
                  placeholder="colleague@example.com"
                />
                <button
                  onClick={handleInvite}
                  disabled={!inviteEmail.trim()}
                  className="px-4 py-2.5 bg-dark-800 border border-dark-700 text-dark-50 font-medium rounded-lg hover:bg-dark-700 disabled:opacity-60 transition-all text-sm"
                >
                  Invite
                </button>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={handleTeamNext}
                  className="flex-1 py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all"
                >
                  Continue
                </button>
                <button
                  onClick={handleTeamNext}
                  className="px-4 py-2.5 text-dark-400 hover:text-dark-300 text-sm transition-colors"
                >
                  Skip
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Complete Step */}
        {step === 'complete' && (
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 text-center">
            <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-accent-emerald to-accent-cyan flex items-center justify-center mx-auto mb-4">
              <CheckCircle className="w-8 h-8 text-dark-50" />
            </div>
            <h2 className="text-xl font-bold text-dark-50 mb-2">You're all set!</h2>
            <p className="text-dark-400 text-sm mb-6">Your account is ready. Let's get started.</p>

            <button
              onClick={handleComplete}
              disabled={loading}
              className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 transition-all"
            >
              {loading ? 'Getting started...' : 'Go to Dashboard'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
