import { useEffect, useState } from 'react';
import { Copy } from 'lucide-react';
import { authApi } from '../../../api/client';
import LoadingSpinner from '../../../components/LoadingSpinner';

interface MFASetupModalProps {
  onClose: () => void;
  onComplete: () => void;
}

export default function MFASetupModal({ onClose, onComplete }: MFASetupModalProps) {
  const [step, setStep] = useState<'qr' | 'verify' | 'codes'>('qr');
  const [qrCodeUrl, setQrCodeUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [code, setCode] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    authApi.mfaSetup()
      .then((data) => {
        setQrCodeUrl(data.qrCodeUrl);
        setSecret(data.secret);
      })
      .catch(() => setError('Failed to initialize MFA setup'));
  }, []);

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const data = await authApi.mfaVerifySetup(code);
      setRecoveryCodes(data.recoveryCodes);
      setStep('codes');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg || 'Invalid code');
    } finally {
      setLoading(false);
    }
  };

  const copyRecoveryCodes = () => {
    navigator.clipboard.writeText(recoveryCodes.join('\n'));
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full" onClick={e => e.stopPropagation()}>
        {step === 'qr' && (
          <>
            <h3 className="text-lg font-semibold text-white mb-4">Set Up Two-Factor Authentication</h3>
            <p className="text-sm text-dark-400 mb-4">Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.)</p>
            {qrCodeUrl ? (
              <div className="flex justify-center mb-4">
                <img src={qrCodeUrl} alt="QR Code" className="w-48 h-48 rounded-lg bg-white p-2" />
              </div>
            ) : (
              <div className="flex justify-center mb-4 py-8"><LoadingSpinner /></div>
            )}
            {secret && (
              <div className="mb-4">
                <p className="text-xs text-dark-400 mb-1">Or enter this key manually:</p>
                <code className="block text-xs bg-dark-800 text-dark-300 px-3 py-2 rounded-lg font-mono break-all">{secret}</code>
              </div>
            )}
            <button onClick={() => setStep('verify')} className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm">
              Next
            </button>
          </>
        )}

        {step === 'verify' && (
          <>
            <h3 className="text-lg font-semibold text-white mb-4">Verify Code</h3>
            <p className="text-sm text-dark-400 mb-4">Enter the 6-digit code from your authenticator app</p>
            {error && <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{error}</div>}
            <form onSubmit={handleVerify} className="space-y-4">
              <input
                type="text"
                required
                autoFocus
                autoComplete="one-time-code"
                inputMode="numeric"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white text-center text-lg tracking-widest focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                placeholder="000000"
                maxLength={6}
              />
              <button type="submit" disabled={loading} className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 transition-all text-sm">
                {loading ? 'Verifying...' : 'Enable MFA'}
              </button>
            </form>
          </>
        )}

        {step === 'codes' && (
          <>
            <h3 className="text-lg font-semibold text-white mb-2">Recovery Codes</h3>
            <p className="text-sm text-dark-400 mb-4">Save these codes in a safe place. Each code can only be used once.</p>
            <div className="bg-dark-800 rounded-lg p-4 mb-4 grid grid-cols-2 gap-2">
              {recoveryCodes.map((c, i) => (
                <code key={i} className="text-sm text-dark-300 font-mono">{c}</code>
              ))}
            </div>
            <div className="flex gap-2">
              <button onClick={copyRecoveryCodes} className="flex-1 py-2.5 px-4 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all text-sm flex items-center justify-center gap-2">
                <Copy className="w-4 h-4" /> Copy
              </button>
              <button onClick={() => { onComplete(); onClose(); }} className="flex-1 py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm">
                Done
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
