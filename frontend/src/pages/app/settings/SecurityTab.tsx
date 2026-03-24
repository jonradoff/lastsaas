import { useEffect, useState } from 'react';
import { CheckCircle, Shield, Fingerprint, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { useAuth } from '../../../contexts/AuthContext';
import { useBranding } from '../../../contexts/BrandingContext';
import { authApi } from '../../../api/client';
import { getErrorMessage } from '../../../utils/errors';
import type { PasskeyCredential } from '../../../types';
import LoadingSpinner from '../../../components/LoadingSpinner';
import ConfirmModal from '../../../components/ConfirmModal';
import MFASetupModal from './MFASetupModal';

export default function SecurityTab() {
  const { user, refreshUser } = useAuth();
  const { branding } = useBranding();
  const passkeysEnabled = branding?.authProviders?.passkeys ?? false;
  const mfaConfigEnabled = branding?.authProviders?.mfa ?? false;
  const showMfaSection = mfaConfigEnabled || user?.totpEnabled;

  // MFA state
  const [showMfaSetup, setShowMfaSetup] = useState(false);
  const [mfaDisableCode, setMfaDisableCode] = useState('');
  const [mfaDisableError, setMfaDisableError] = useState('');
  const [mfaDisabling, setMfaDisabling] = useState(false);
  const [showDisableMfa, setShowDisableMfa] = useState(false);

  // Passkeys state
  const [passkeys, setPasskeys] = useState<PasskeyCredential[]>([]);
  const [passkeysLoading, setPasskeysLoading] = useState(false);
  const [passkeyName, setPasskeyName] = useState('');
  const [addingPasskey, setAddingPasskey] = useState(false);
  const [passkeyError, setPasskeyError] = useState('');
  const [confirmDeletePasskeyId, setConfirmDeletePasskeyId] = useState<string | null>(null);
  const [confirmLoading, setConfirmLoading] = useState(false);

  const loadPasskeys = () => {
    setPasskeysLoading(true);
    authApi.listPasskeys()
      .then((data) => setPasskeys(data.passkeys || []))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setPasskeysLoading(false));
  };

  useEffect(() => { loadPasskeys(); }, []);

  const handleDisableMfa = async (e: React.FormEvent) => {
    e.preventDefault();
    setMfaDisableError('');
    setMfaDisabling(true);
    try {
      await authApi.mfaDisable(mfaDisableCode);
      await refreshUser();
      setShowDisableMfa(false);
      setMfaDisableCode('');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setMfaDisableError(msg || 'Invalid code');
    } finally {
      setMfaDisabling(false);
    }
  };

  const handleAddPasskey = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasskeyError('');
    setAddingPasskey(true);
    try {
      const options = await authApi.passkeyRegisterBegin();
      const credential = await navigator.credentials.create({ publicKey: options });
      if (!credential) throw new Error('No credential created');
      await authApi.passkeyRegisterFinish({ name: passkeyName || 'My Passkey', credential });
      setPasskeyName('');
      loadPasskeys();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
        || (err as Error)?.message || 'Failed to add passkey';
      setPasskeyError(msg);
    } finally {
      setAddingPasskey(false);
    }
  };

  const handleDeletePasskey = async (id: string) => {
    setConfirmLoading(true);
    try {
      await authApi.deletePasskey(id);
      setPasskeys(p => p.filter(pk => pk.id !== id));
      toast.success('Passkey deleted');
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setConfirmLoading(false);
      setConfirmDeletePasskeyId(null);
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      {/* MFA Section */}
      {showMfaSection && (
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-dark-50 flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5 text-dark-400" />
          Two-Factor Authentication
        </h2>

        {user?.totpEnabled ? (
          <div>
            <div className="flex items-center gap-2 mb-4">
              <span className="flex items-center gap-1 text-sm text-accent-emerald">
                <CheckCircle className="w-4 h-4" /> Enabled
              </span>
            </div>

            {showDisableMfa ? (
              <form onSubmit={handleDisableMfa} className="space-y-3">
                {mfaDisableError && (
                  <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{mfaDisableError}</div>
                )}
                <div>
                  <label className="block text-sm font-medium text-dark-300 mb-1.5">Enter TOTP code or recovery code to disable</label>
                  <input
                    type="text"
                    required
                    autoFocus
                    value={mfaDisableCode}
                    onChange={(e) => setMfaDisableCode(e.target.value)}
                    className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-dark-50 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                    placeholder="000000"
                  />
                </div>
                <div className="flex gap-2">
                  <button type="submit" disabled={mfaDisabling} className="py-2 px-4 bg-red-500/20 text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/30 text-sm disabled:opacity-60 transition-all">
                    {mfaDisabling ? 'Disabling...' : 'Confirm Disable'}
                  </button>
                  <button type="button" onClick={() => { setShowDisableMfa(false); setMfaDisableCode(''); setMfaDisableError(''); }} className="py-2 px-4 bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-700 text-sm transition-all">
                    Cancel
                  </button>
                </div>
              </form>
            ) : (
              <button
                onClick={() => setShowDisableMfa(true)}
                className="py-2 px-4 bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-700 text-sm transition-all"
              >
                Disable MFA
              </button>
            )}
          </div>
        ) : (
          <div>
            <p className="text-sm text-dark-400 mb-4">Add an extra layer of security to your account with a TOTP authenticator app.</p>
            <button
              onClick={() => setShowMfaSetup(true)}
              className="py-2.5 px-6 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm"
            >
              Enable Two-Factor Authentication
            </button>
          </div>
        )}
      </div>
      )}

      {/* Passkeys Section */}
      {passkeysEnabled && (
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-dark-50 flex items-center gap-2 mb-4">
          <Fingerprint className="w-5 h-5 text-dark-400" />
          Passkeys
        </h2>

        {passkeysLoading ? (
          <LoadingSpinner size="sm" className="py-4" />
        ) : (
          <>
            {passkeys.length > 0 && (
              <div className="space-y-2 mb-4">
                {passkeys.map(pk => (
                  <div key={pk.id} className="flex items-center justify-between py-2 px-3 bg-dark-800/50 rounded-lg">
                    <div>
                      <p className="text-sm text-dark-50">{pk.name}</p>
                      <p className="text-xs text-dark-400">
                        Added {new Date(pk.createdAt).toLocaleDateString()}
                        {pk.lastUsedAt && ` · Last used ${new Date(pk.lastUsedAt).toLocaleDateString()}`}
                      </p>
                    </div>
                    <button onClick={() => setConfirmDeletePasskeyId(pk.id)} className="text-dark-400 hover:text-red-400 transition-colors" aria-label="Delete passkey">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            {passkeyError && (
              <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{passkeyError}</div>
            )}

            <form onSubmit={handleAddPasskey} className="flex gap-2">
              <input
                type="text"
                value={passkeyName}
                onChange={(e) => setPasskeyName(e.target.value)}
                className="flex-1 px-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-dark-50 placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors text-sm"
                placeholder="Passkey name (e.g., MacBook)"
              />
              <button type="submit" disabled={addingPasskey} className="py-2 px-4 bg-dark-800 border border-dark-700 text-dark-50 rounded-lg hover:bg-dark-700 text-sm disabled:opacity-60 transition-all">
                {addingPasskey ? 'Adding...' : 'Add Passkey'}
              </button>
            </form>
          </>
        )}
      </div>
      )}

      {/* MFA Setup Modal */}
      {showMfaSetup && (
        <MFASetupModal onClose={() => setShowMfaSetup(false)} onComplete={refreshUser} />
      )}

      {/* Confirm Delete Passkey */}
      <ConfirmModal
        open={confirmDeletePasskeyId !== null}
        onClose={() => setConfirmDeletePasskeyId(null)}
        onConfirm={() => confirmDeletePasskeyId && handleDeletePasskey(confirmDeletePasskeyId)}
        title="Delete Passkey"
        message="This passkey will be permanently removed. You won't be able to use it to sign in anymore."
        confirmLabel="Delete"
        confirmVariant="danger"
        loading={confirmLoading}
      />
    </div>
  );
}
