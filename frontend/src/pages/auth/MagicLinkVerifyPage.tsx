import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { authApi } from '../../api/client';
import LoadingSpinner from '../../components/LoadingSpinner';

export default function MagicLinkVerifyPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') || '';
  const { loginWithTokens } = useAuth();
  const [error, setError] = useState('');

  useEffect(() => {
    if (!token) {
      setError('Missing verification token');
      return;
    }

    authApi.verifyMagicLink(token)
      .then((data) => {
        if ('mfaRequired' in data && data.mfaRequired) {
          navigate(`/auth/mfa?token=${encodeURIComponent(data.mfaToken)}`);
          return;
        }
        const authData = data as import('../../types').AuthResponse;
        return loginWithTokens(authData.accessToken, authData.refreshToken)
          .then(() => navigate('/dashboard'));
      })
      .catch((err) => {
        const msg = err?.response?.data?.error;
        setError(msg || 'Invalid or expired link');
      });
  }, [token, loginWithTokens, navigate]);

  if (error) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center max-w-md">
          <h1 className="text-xl font-bold text-dark-50 mb-2">Verification Failed</h1>
          <p className="text-dark-400 mb-4">{error}</p>
          <button
            onClick={() => navigate('/login')}
            className="py-2.5 px-6 bg-dark-800 border border-dark-700 text-dark-50 font-medium rounded-lg hover:bg-dark-700 transition-all"
          >
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center">
      <div className="text-center">
        <LoadingSpinner size="lg" className="mb-4" />
        <p className="text-dark-400">Verifying your sign-in link...</p>
      </div>
    </div>
  );
}
