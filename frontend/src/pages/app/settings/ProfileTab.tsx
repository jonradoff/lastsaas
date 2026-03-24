import { useState } from 'react';
import { User, KeyRound, CheckCircle, AlertCircle, Download, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { useAuth } from '../../../contexts/AuthContext';
import { authApi } from '../../../api/client';
import { getErrorMessage } from '../../../utils/errors';

function PasswordStrength({ password }: { password: string }) {
  const checks = [
    { label: '10+ characters', met: password.length >= 10 },
    { label: 'Uppercase letter', met: /[A-Z]/.test(password) },
    { label: 'Lowercase letter', met: /[a-z]/.test(password) },
    { label: 'Number', met: /\d/.test(password) },
    { label: 'Special character', met: /[^A-Za-z0-9]/.test(password) },
  ];
  const score = checks.filter(c => c.met).length;
  const strength = score <= 2 ? 'Weak' : score <= 3 ? 'Fair' : score <= 4 ? 'Good' : 'Strong';
  const color = score <= 2 ? 'bg-red-500' : score <= 3 ? 'bg-amber-500' : score <= 4 ? 'bg-primary-500' : 'bg-accent-emerald';
  const textColor = score <= 2 ? 'text-red-400' : score <= 3 ? 'text-amber-400' : score <= 4 ? 'text-primary-400' : 'text-accent-emerald';

  return (
    <div className="mt-2">
      <div className="flex items-center gap-2 mb-1.5">
        <div className="flex-1 h-1.5 bg-dark-700 rounded-full overflow-hidden flex gap-0.5">
          {[1, 2, 3, 4, 5].map(i => (
            <div key={i} className={`flex-1 rounded-full transition-colors ${i <= score ? color : 'bg-dark-700'}`} />
          ))}
        </div>
        <span className={`text-xs font-medium ${textColor}`}>{strength}</span>
      </div>
      <div className="flex flex-wrap gap-x-3 gap-y-0.5">
        {checks.map(c => (
          <span key={c.label} className={`text-xs ${c.met ? 'text-accent-emerald' : 'text-dark-500'}`}>
            {c.met ? '\u2713' : '\u2717'} {c.label}
          </span>
        ))}
      </div>
    </div>
  );
}

export default function ProfileTab() {
  const { user, refreshUser } = useAuth();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletePassword, setDeletePassword] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [exporting, setExporting] = useState(false);

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');
    setChangingPassword(true);
    try {
      await authApi.changePassword(currentPassword, newPassword);
      setPasswordSuccess('Password changed successfully');
      setCurrentPassword('');
      setNewPassword('');
      toast.success('Password changed successfully');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setPasswordError(msg || 'Failed to change password');
    } finally {
      setChangingPassword(false);
    }
  };

  const handleExportData = async () => {
    setExporting(true);
    try {
      const blob = await authApi.exportData();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'account-data.json';
      a.click();
      URL.revokeObjectURL(url);
      toast.success('Data exported successfully');
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setExporting(false);
    }
  };

  const handleDeleteAccount = async () => {
    setDeleting(true);
    try {
      await authApi.deleteAccount(deletePassword);
      toast.success('Account deleted');
      window.location.href = '/login';
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setDeleting(false);
    }
  };

  const handleResendVerification = async () => {
    if (!user?.email) return;
    try {
      await authApi.resendVerification(user.email);
      await refreshUser();
    } catch {
      // ignore
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
          <User className="w-5 h-5 text-dark-400" />
          Profile
        </h2>
        <div className="space-y-3">
          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-dark-400">Name</span>
            <span className="text-sm text-white">{user?.displayName}</span>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Email</span>
            <span className="text-sm text-white">{user?.email}</span>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Email Verified</span>
            <div className="flex items-center gap-2">
              {user?.emailVerified ? (
                <span className="flex items-center gap-1 text-sm text-accent-emerald">
                  <CheckCircle className="w-4 h-4" /> Verified
                </span>
              ) : (
                <div className="flex items-center gap-2">
                  <span className="flex items-center gap-1 text-sm text-amber-400">
                    <AlertCircle className="w-4 h-4" /> Not verified
                  </span>
                  <button
                    onClick={handleResendVerification}
                    className="text-xs text-primary-400 hover:text-primary-300 transition-colors"
                  >
                    Resend
                  </button>
                </div>
              )}
            </div>
          </div>
          <div className="flex items-center justify-between py-2 border-t border-dark-800">
            <span className="text-sm text-dark-400">Auth Methods</span>
            <div className="flex gap-2">
              {user?.authMethods.map((method) => (
                <span key={method} className="px-2 py-0.5 bg-dark-800 rounded text-xs text-dark-300 capitalize">
                  {method}
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
          <KeyRound className="w-5 h-5 text-dark-400" />
          Change Password
        </h2>

        {passwordError && (
          <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{passwordError}</div>
        )}
        {passwordSuccess && (
          <div className="mb-4 bg-accent-emerald/10 border border-accent-emerald/20 rounded-lg p-3 text-sm text-accent-emerald">{passwordSuccess}</div>
        )}

        <form onSubmit={handleChangePassword} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Current Password</label>
            <input
              type="password"
              required
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">New Password</label>
            <input
              type="password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
              placeholder="Min 10 chars, mixed case, number, special"
            />
            {newPassword && <PasswordStrength password={newPassword} />}
          </div>
          <button
            type="submit"
            disabled={changingPassword}
            className="py-2.5 px-6 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 disabled:cursor-not-allowed transition-all text-sm"
          >
            {changingPassword ? 'Changing...' : 'Change Password'}
          </button>
        </form>
      </div>

      {/* Data Export */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-2">
          <Download className="w-5 h-5 text-dark-400" />
          Export My Data
        </h2>
        <p className="text-sm text-dark-400 mb-4">Download a JSON file containing your profile, memberships, and messages.</p>
        <button
          onClick={handleExportData}
          disabled={exporting}
          className="py-2 px-4 bg-dark-800 text-dark-200 text-sm font-medium rounded-lg hover:bg-dark-700 disabled:opacity-60 transition-colors"
        >
          {exporting ? 'Exporting...' : 'Download Data'}
        </button>
      </div>

      {/* Delete Account */}
      <div className="bg-red-500/5 border border-red-500/20 rounded-2xl p-6">
        <h2 className="text-lg font-semibold text-red-400 flex items-center gap-2 mb-2">
          <Trash2 className="w-5 h-5" />
          Delete Account
        </h2>
        <p className="text-sm text-dark-400 mb-4">
          Permanently delete your account and all associated data. This action cannot be undone.
        </p>
        <button
          onClick={() => setShowDeleteModal(true)}
          className="py-2 px-4 bg-red-500/10 text-red-400 text-sm font-medium rounded-lg border border-red-500/20 hover:bg-red-500/20 transition-colors"
        >
          Delete My Account
        </button>
      </div>

      {/* Delete Account Modal */}
      {showDeleteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setShowDeleteModal(false)} />
          <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-md" role="dialog" aria-modal="true">
            <h3 className="text-lg font-semibold text-red-400 mb-2">Delete Account</h3>
            <p className="text-sm text-dark-400 mb-4">
              This will permanently delete your account and all data. If you own any teams with other members, you must transfer ownership first.
            </p>
            {user?.authMethods.includes('password') && (
              <div className="mb-4">
                <label className="block text-sm font-medium text-dark-300 mb-1.5">Confirm your password</label>
                <input
                  type="password"
                  value={deletePassword}
                  onChange={e => setDeletePassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-red-500 transition-colors"
                  placeholder="Enter your password"
                />
              </div>
            )}
            <div className="flex justify-end gap-3">
              <button
                onClick={() => { setShowDeleteModal(false); setDeletePassword(''); }}
                className="px-4 py-2 text-sm text-dark-400 hover:text-white transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteAccount}
                disabled={deleting || (user?.authMethods.includes('password') && !deletePassword)}
                className="px-4 py-2 bg-red-500 text-white text-sm font-medium rounded-lg hover:bg-red-600 disabled:opacity-60 transition-colors"
              >
                {deleting ? 'Deleting...' : 'Permanently Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
