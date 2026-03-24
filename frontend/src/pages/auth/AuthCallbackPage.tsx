import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { authApi } from '../../api/client';
import LoadingSpinner from '../../components/LoadingSpinner';

export default function AuthCallbackPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { loginWithTokens } = useAuth();
  const [error, setError] = useState('');

  useEffect(() => {
    const code = searchParams.get('code');
    const errorParam = searchParams.get('error');

    if (errorParam) {
      setError(errorParam);
      return;
    }

    if (!code) {
      setError('Missing authentication code');
      return;
    }

    authApi.exchangeCode(code)
      .then((data) => {
        if (data.mfaRequired && data.mfaToken) {
          navigate(`/auth/mfa?token=${encodeURIComponent(data.mfaToken)}`);
          return;
        }
        if (data.accessToken && data.refreshToken) {
          return loginWithTokens(data.accessToken, data.refreshToken)
            .then(() => navigate('/dashboard'));
        }
        setError('Invalid authentication response');
      })
      .catch(() => setError('Failed to complete authentication'));
  }, [searchParams, loginWithTokens, navigate]);

  if (error) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center max-w-md">
          <h1 className="text-xl font-bold text-dark-50 mb-2">Authentication Failed</h1>
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
        <p className="text-dark-400">Completing authentication...</p>
      </div>
    </div>
  );
}
