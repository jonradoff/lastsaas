import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { KeyRound } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';

export default function MFAChallengePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const mfaToken = searchParams.get('token') || '';
  const { completeMfaChallenge } = useAuth();
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  if (!mfaToken) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center max-w-md">
          <h1 className="text-xl font-bold text-white mb-2">Invalid Request</h1>
          <p className="text-dark-400 mb-4">Missing MFA token.</p>
          <button onClick={() => navigate('/login')} className="py-2.5 px-6 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all">
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await completeMfaChallenge(mfaToken, code);
      navigate('/dashboard');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg || 'Invalid verification code');
    } finally {
      setLoading(false);
    }
  };

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

        <form onSubmit={handleSubmit} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 space-y-4">
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
              value={code}
              onChange={(e) => setCode(e.target.value)}
              className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors text-center text-lg tracking-widest"
              placeholder="000000"
              maxLength={32}
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 disabled:cursor-not-allowed transition-all"
          >
            {loading ? 'Verifying...' : 'Verify'}
          </button>

          <button
            type="button"
            onClick={() => navigate('/login')}
            className="w-full text-sm text-dark-400 hover:text-dark-300 transition-colors"
          >
            Back to login
          </button>
        </form>
      </div>
    </div>
  );
}
