import { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Save, Shield, Trash2, FileText, AlertTriangle, X, Zap } from 'lucide-react';
import { adminApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import type { UserDetail, UserMembershipDetail, DeletePreflightResponse } from '../../types';
import { useAuth } from '../../contexts/AuthContext';
import { useTenant } from '../../contexts/TenantContext';
import LoadingSpinner from '../../components/LoadingSpinner';

export default function UserProfilePage() {
  const { userId } = useParams<{ userId: string }>();
  const navigate = useNavigate();
  const { user: currentUser } = useAuth();
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';
  const isOwner = role === 'owner';

  const [user, setUser] = useState<UserDetail | null>(null);
  const [memberships, setMemberships] = useState<UserMembershipDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState('');

  // Edit fields
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [saveSuccess, setSaveSuccess] = useState('');

  // Delete flow
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [preflight, setPreflight] = useState<DeletePreflightResponse | null>(null);
  const [preflightLoading, setPreflightLoading] = useState(false);
  const [replacementOwners, setReplacementOwners] = useState<Record<string, string>>({});
  const [confirmedTenantDeletions, setConfirmedTenantDeletions] = useState<string[]>([]);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  const fetchUser = useCallback(async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const data = await adminApi.getUser(userId);
      setUser(data.user);
      setMemberships(data.memberships);
      setEmail(data.user.email);
      setDisplayName(data.user.displayName);
    } catch (err: unknown) {
      setFetchError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [userId, navigate]);

  useEffect(() => { fetchUser(); }, [fetchUser]);

  const handleSave = async () => {
    if (!user) return;
    setSaving(true);
    setSaveError('');
    setSaveSuccess('');
    try {
      const updates: { email?: string; displayName?: string } = {};
      if (email.trim() !== user.email) updates.email = email.trim();
      if (displayName.trim() !== user.displayName) updates.displayName = displayName.trim();
      if (Object.keys(updates).length === 0) {
        setSaveSuccess('No changes to save');
        setSaving(false);
        return;
      }
      await adminApi.updateUser(user.id, updates);
      setSaveSuccess('User updated successfully');
      await fetchUser();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to update user';
      setSaveError(msg);
    } finally {
      setSaving(false);
    }
  };

  const handleToggleStatus = async () => {
    if (!user) return;
    try {
      await adminApi.updateUserStatus(user.id, !user.isActive);
      await fetchUser();
    } catch {
      // ignore
    }
  };

  const handleDeleteClick = async () => {
    if (!user) return;
    setPreflightLoading(true);
    setDeleteError('');
    setReplacementOwners({});
    setConfirmedTenantDeletions([]);
    try {
      const data = await adminApi.preflightDeleteUser(user.id);
      setPreflight(data);
      if (!data.canDelete) {
        setDeleteError(data.reason || 'This user cannot be deleted');
      }
      setShowDeleteModal(true);
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to check delete eligibility';
      setDeleteError(msg);
      setShowDeleteModal(true);
    } finally {
      setPreflightLoading(false);
    }
  };

  const handleConfirmDelete = async () => {
    if (!user) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await adminApi.deleteUser(user.id, {
        replacementOwners: Object.keys(replacementOwners).length > 0 ? replacementOwners : undefined,
        confirmTenantDeletions: confirmedTenantDeletions.length > 0 ? confirmedTenantDeletions : undefined,
      });
      navigate('/last/users');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to delete user';
      setDeleteError(msg);
    } finally {
      setDeleting(false);
    }
  };

  const canSubmitDelete = () => {
    if (!preflight?.canDelete) return false;
    if (!preflight.ownerships) return true;
    for (const own of preflight.ownerships) {
      if (own.isRoot) return false;
      if (own.otherMembers.length > 0 && !replacementOwners[own.tenantId]) return false;
      if (own.otherMembers.length === 0 && !confirmedTenantDeletions.includes(own.tenantId)) return false;
    }
    return true;
  };

  const isSelf = currentUser?.id === userId;

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;
  if (fetchError) return (
    <div className="py-20 text-center">
      <p className="text-red-400 text-lg mb-2">Failed to load user profile</p>
      <p className="text-dark-400 text-sm font-mono">{fetchError}</p>
      <button onClick={() => navigate('/last/users')} className="mt-4 px-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white transition-colors">
        Back to Users
      </button>
    </div>
  );
  if (!user) return null;

  return (
    <div>
      {/* Header */}
      <div className="flex items-center gap-4 mb-8">
        <button onClick={() => navigate('/last/users')} className="p-2 hover:bg-dark-800 rounded-lg transition-colors">
          <ArrowLeft className="w-5 h-5 text-dark-400" />
        </button>
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Shield className="w-7 h-7 text-primary-400" />
            User Profile
          </h1>
          <p className="text-dark-400 mt-1">{user.displayName} &middot; {user.email}</p>
        </div>
      </div>

      {/* Info / Edit Section */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-4">User Information</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm text-dark-400 mb-1">Display Name</label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          </div>
          <div>
            <label className="block text-sm text-dark-400 mb-1">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4 text-sm">
          <div>
            <span className="text-dark-500">Email Verified</span>
            <p className={user.emailVerified ? 'text-accent-emerald' : 'text-red-400'}>
              {user.emailVerified ? 'Yes' : 'No'}
            </p>
          </div>
          <div>
            <span className="text-dark-500">Auth Methods</span>
            <p className="text-dark-300">{user.authMethods.join(', ')}</p>
          </div>
          <div>
            <span className="text-dark-500">Created</span>
            <p className="text-dark-300">{new Date(user.createdAt).toLocaleDateString()}</p>
          </div>
          <div>
            <span className="text-dark-500">Last Login</span>
            <p className="text-dark-300">{user.lastLoginAt ? new Date(user.lastLoginAt).toLocaleString() : 'Never'}</p>
          </div>
        </div>

        {saveError && (
          <div className="mb-4 px-4 py-2 bg-red-500/10 border border-red-500/20 rounded-lg text-sm text-red-400">
            {saveError}
          </div>
        )}
        {saveSuccess && (
          <div className="mb-4 px-4 py-2 bg-accent-emerald/10 border border-accent-emerald/20 rounded-lg text-sm text-accent-emerald">
            {saveSuccess}
          </div>
        )}

        {canWrite && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 disabled:opacity-60 transition-colors"
          >
            <Save className="w-4 h-4" />
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        )}
      </div>

      {/* Status Section */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-4">Account Status</h2>
        <div className="flex items-center gap-4">
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
            user.isActive ? 'bg-accent-emerald/10 text-accent-emerald' : 'bg-red-500/10 text-red-400'
          }`}>
            {user.isActive ? 'Active' : 'Disabled'}
          </span>
          {canWrite && (
            <button
              onClick={handleToggleStatus}
              className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                user.isActive
                  ? 'border-red-500/30 text-red-400 hover:bg-red-500/10'
                  : 'border-accent-emerald/30 text-accent-emerald hover:bg-accent-emerald/10'
              }`}
            >
              {user.isActive ? 'Disable Account' : 'Enable Account'}
            </button>
          )}
        </div>
      </div>

      {/* Memberships Section */}
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 mb-6">
        <h2 className="text-lg font-semibold text-white mb-4">Tenant Memberships</h2>

        {memberships.length === 0 ? (
          <p className="text-dark-400 text-sm">This user is not a member of any tenants.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-dark-700">
                <th className="text-left px-4 py-2 text-dark-400 font-medium">Tenant</th>
                <th className="text-left px-4 py-2 text-dark-400 font-medium">Role</th>
                <th className="text-left px-4 py-2 text-dark-400 font-medium">Plan</th>
                <th className="text-left px-4 py-2 text-dark-400 font-medium">Credits</th>
                <th className="text-left px-4 py-2 text-dark-400 font-medium">Joined</th>
              </tr>
            </thead>
            <tbody>
              {memberships.map((m) => (
                <tr
                  key={m.tenantId}
                  onClick={() => navigate(`/last/tenants/${m.tenantId}`)}
                  className="border-b border-dark-800/50 hover:bg-dark-800/30 cursor-pointer transition-colors"
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="text-white hover:text-primary-400 transition-colors">{m.tenantName}</span>
                      <span className="text-dark-500 text-xs font-mono">({m.tenantSlug})</span>
                      {m.isRoot && (
                        <span className="px-1.5 py-0.5 bg-primary-500/10 text-primary-400 text-xs rounded">Root</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-0.5 text-xs font-medium rounded ${
                      m.role === 'owner' ? 'bg-amber-500/20 text-amber-400' :
                      m.role === 'admin' ? 'bg-blue-500/20 text-blue-400' :
                      'bg-dark-700 text-dark-300'
                    }`}>
                      {m.role.charAt(0).toUpperCase() + m.role.slice(1)}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-dark-300">
                    {m.planName}
                    {m.billingWaived && (
                      <span className="ml-2 text-xs text-amber-400">(waived)</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 text-dark-300">
                      <Zap className="w-3.5 h-3.5 text-primary-400" />
                      {(m.subscriptionCredits + m.purchasedCredits).toLocaleString()}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-dark-400">
                    {new Date(m.joinedAt).toLocaleDateString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* View Logs + Danger Zone */}
      <div className="flex flex-col gap-6">
        <Link
          to={`/last/logs?userId=${userId}`}
          className="inline-flex items-center gap-2 px-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-dark-300 hover:text-white hover:border-dark-600 transition-colors w-fit"
        >
          <FileText className="w-4 h-4" />
          View User Logs
        </Link>

        {isOwner && !isSelf && (
          <div className="bg-red-500/5 border border-red-500/15 rounded-2xl p-6">
            <h2 className="text-lg font-semibold text-red-400 mb-2">Danger Zone</h2>
            <p className="text-dark-400 text-sm mb-4">
              Permanently delete this user account and all associated data. This action cannot be undone.
            </p>
            <button
              onClick={handleDeleteClick}
              disabled={preflightLoading}
              className="flex items-center gap-2 px-4 py-2 bg-red-500/10 border border-red-500/30 text-red-400 text-sm font-medium rounded-lg hover:bg-red-500/20 disabled:opacity-60 transition-colors"
            >
              <Trash2 className="w-4 h-4" />
              {preflightLoading ? 'Checking...' : 'Delete User'}
            </button>
          </div>
        )}
      </div>

      {/* Delete Modal */}
      {isOwner && showDeleteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setShowDeleteModal(false)} />
          <div className="relative bg-dark-900 border border-dark-700 rounded-2xl p-6 w-full max-w-lg max-h-[80vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-white flex items-center gap-2">
                <AlertTriangle className="w-5 h-5 text-red-400" />
                Delete User
              </h3>
              <button onClick={() => setShowDeleteModal(false)} className="p-1 hover:bg-dark-800 rounded-lg transition-colors">
                <X className="w-5 h-5 text-dark-400" />
              </button>
            </div>

            {!preflight?.canDelete ? (
              <div className="px-4 py-3 bg-red-500/10 border border-red-500/20 rounded-lg text-sm text-red-400">
                {deleteError || preflight?.reason || 'This user cannot be deleted.'}
              </div>
            ) : (
              <>
                <p className="text-dark-300 text-sm mb-4">
                  You are about to permanently delete <strong className="text-white">{user.displayName}</strong> ({user.email}).
                  This will remove their account, memberships, messages, and tokens.
                </p>

                {/* Ownership resolution */}
                {preflight.ownerships && preflight.ownerships.length > 0 && (
                  <div className="space-y-4 mb-4">
                    {preflight.ownerships.map((own) => (
                      <div key={own.tenantId} className="p-4 bg-dark-800 border border-dark-700 rounded-lg">
                        <p className="text-sm text-white font-medium mb-2">
                          Tenant: {own.tenantName}
                          {own.isRoot && <span className="ml-2 text-xs text-red-400">(Root — cannot delete owner)</span>}
                        </p>

                        {own.isRoot ? (
                          <p className="text-xs text-red-400">
                            Root tenant ownership must be transferred via CLI before this user can be deleted.
                          </p>
                        ) : own.otherMembers.length > 0 ? (
                          <div>
                            <p className="text-xs text-dark-400 mb-2">Select a new owner for this tenant:</p>
                            <select
                              value={replacementOwners[own.tenantId] || ''}
                              onChange={(e) => setReplacementOwners(prev => ({ ...prev, [own.tenantId]: e.target.value }))}
                              className="w-full px-3 py-2 bg-dark-900 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
                            >
                              <option value="">Choose a replacement owner...</option>
                              {own.otherMembers.map((member) => (
                                <option key={member.userId} value={member.userId}>
                                  {member.displayName} ({member.email})
                                </option>
                              ))}
                            </select>
                          </div>
                        ) : (
                          <div>
                            <div className="px-3 py-2 bg-red-500/10 border border-red-500/20 rounded-lg text-xs text-red-400 mb-2">
                              This user is the only member of this tenant. Deleting them will <strong>permanently remove the entire tenant</strong>. This is irreversible.
                            </div>
                            <label className="flex items-center gap-2 text-sm text-dark-300 cursor-pointer">
                              <input
                                type="checkbox"
                                checked={confirmedTenantDeletions.includes(own.tenantId)}
                                onChange={(e) => {
                                  if (e.target.checked) {
                                    setConfirmedTenantDeletions(prev => [...prev, own.tenantId]);
                                  } else {
                                    setConfirmedTenantDeletions(prev => prev.filter(id => id !== own.tenantId));
                                  }
                                }}
                                className="rounded border-dark-600"
                              />
                              I understand that tenant "{own.tenantName}" will be permanently deleted
                            </label>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}

                {deleteError && (
                  <div className="mb-4 px-4 py-2 bg-red-500/10 border border-red-500/20 rounded-lg text-sm text-red-400">
                    {deleteError}
                  </div>
                )}

                <div className="flex justify-end gap-3">
                  <button
                    onClick={() => setShowDeleteModal(false)}
                    className="px-4 py-2 bg-dark-800 border border-dark-700 text-dark-300 text-sm rounded-lg hover:text-white transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleConfirmDelete}
                    disabled={!canSubmitDelete() || deleting}
                    className="px-4 py-2 bg-red-500 text-white text-sm font-medium rounded-lg hover:bg-red-600 disabled:opacity-60 disabled:hover:bg-red-500 transition-colors"
                  >
                    {deleting ? 'Deleting...' : 'Permanently Delete User'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
