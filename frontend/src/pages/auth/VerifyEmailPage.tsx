import { useState, useEffect } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { MailCheck, MailX, Loader2 } from 'lucide-react';
import { authApi } from '../../api/client';

export default function VerifyEmailPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token');
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [message, setMessage] = useState('');

  useEffect(() => {
    if (!token) {
      setStatus('error');
      setMessage('Missing verification token');
      return;
    }

    authApi.verifyEmail(token)
      .then(() => {
        setStatus('success');
        setMessage('Your email has been verified successfully.');
      })
      .catch((err) => {
        setStatus('error');
        setMessage(err.response?.data?.error || 'Verification failed. The token may have expired.');
      });
  }, [token]);

  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
      <div className="w-full max-w-md text-center">
        {status === 'loading' && (
          <>
            <Loader2 className="w-12 h-12 text-primary-500 animate-spin mx-auto mb-4" />
            <h1 className="text-xl font-bold text-dark-50">Verifying your email...</h1>
          </>
        )}

        {status === 'success' && (
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8">
            <div className="w-14 h-14 rounded-2xl bg-accent-emerald/20 flex items-center justify-center mx-auto mb-4">
              <MailCheck className="w-7 h-7 text-accent-emerald" />
            </div>
            <h1 className="text-xl font-bold text-dark-50 mb-2">Email Verified</h1>
            <p className="text-dark-400 mb-6">{message}</p>
            <Link
              to="/login"
              className="inline-block py-2.5 px-6 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all"
            >
              Continue to Login
            </Link>
          </div>
        )}

        {status === 'error' && (
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8">
            <div className="w-14 h-14 rounded-2xl bg-red-500/20 flex items-center justify-center mx-auto mb-4">
              <MailX className="w-7 h-7 text-red-400" />
            </div>
            <h1 className="text-xl font-bold text-dark-50 mb-2">Verification Failed</h1>
            <p className="text-dark-400 mb-6">{message}</p>
            <Link
              to="/login"
              className="inline-block py-2.5 px-6 bg-dark-800 border border-dark-700 text-dark-50 font-medium rounded-lg hover:bg-dark-700 transition-all"
            >
              Back to Login
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
