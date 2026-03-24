import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Users, UserPlus, Trash2, Crown, ShieldCheck, User, Zap } from 'lucide-react';
import { toast } from 'sonner';
import { tenantApi, plansApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import { useAuth } from '../../contexts/AuthContext';
import { useTenant } from '../../contexts/TenantContext';
import type { TenantMember, Plan } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import ConfirmModal from '../../components/ConfirmModal';

const roleIcons = { owner: Crown, admin: ShieldCheck, user: User };

function renderTemplate(template: string, vars: Record<string, string | number>): string {
  let result = template;
  for (const [key, value] of Object.entries(vars)) {
    result = result.replace(new RegExp(`\\{\\{\\.${key}\\}\\}`, 'g'), String(value));
  }
  // Handle simple {{if ne .Var N}}...{{end}} blocks
  result = result.replace(/\{\{if ne \.(\w+) (\d+)\}\}(.*?)\{\{end\}\}/g, (_match, varName, compare, content) => {
    return String(vars[varName]) !== compare ? content : '';
  });
  return result;
}

export default function TeamPage() {
  const { user } = useAuth();
  const { role: myRole } = useTenant();
  const navigate = useNavigate();
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState('user');
  const [inviting, setInviting] = useState(false);
  const [showInvite, setShowInvite] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [showUpgradeModal, setShowUpgradeModal] = useState(false);
  const [upgradePromptTitle, setUpgradePromptTitle] = useState('');
  const [upgradePromptBody, setUpgradePromptBody] = useState('');
  const [currentPlanUserLimit, setCurrentPlanUserLimit] = useState(0);
  const [currentPlanId, setCurrentPlanId] = useState('');
  const [plans, setPlans] = useState<Plan[]>([]);

  const [removeMember, setRemoveMember] = useState<TenantMember | null>(null);
  const [removeLoading, setRemoveLoading] = useState(false);

  const canManage = myRole === 'owner' || myRole === 'admin';
  const isOwner = myRole === 'owner';

  const fetchMembers = () => {
    tenantApi.listMembers()
      .then((data) => setMembers(data.members))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchMembers(); }, []);

  useEffect(() => {
    plansApi.list()
      .then((data) => {
        setCurrentPlanUserLimit(data.currentPlanUserLimit);
        setCurrentPlanId(data.currentPlanId);
        setPlans(data.plans);
        setUpgradePromptTitle(data.upgradePromptTitle);
        setUpgradePromptBody(data.upgradePromptBody);
      })
      .catch(err => toast.error(getErrorMessage(err)));
  }, []);

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setInviting(true);
    try {
      await tenantApi.inviteMember(inviteEmail, inviteRole);
      setSuccess(`Invitation sent to ${inviteEmail}`);
      setInviteEmail('');
      setShowInvite(false);
    } catch (err: unknown) {
      const data = (err as { response?: { data?: { error?: string; code?: string } } })?.response?.data;
      if (data?.code === 'USER_LIMIT_REACHED') {
        setShowUpgradeModal(true);
      } else {
        setError(data?.error || 'Failed to send invitation');
      }
    } finally {
      setInviting(false);
    }
  };

  const handleRemove = async (member: TenantMember) => {
    setRemoveLoading(true);
    try {
      await tenantApi.removeMember(member.userId);
      setMembers(members.filter(m => m.userId !== member.userId));
      toast.success(`${member.displayName} removed from team`);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setRemoveLoading(false);
      setRemoveMember(null);
    }
  };

  const handleChangeRole = async (member: TenantMember, newRole: string) => {
    try {
      await tenantApi.changeRole(member.userId, newRole);
      setMembers(members.map(m => m.userId === member.userId ? { ...m, role: newRole as TenantMember['role'] } : m));
      toast.success(`${member.displayName}'s role changed to ${newRole}`);
    } catch (err) {
      toast.error(getErrorMessage(err));
    }
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Users className="w-7 h-7 text-primary-400" />
            Team
          </h1>
          <p className="text-dark-400 mt-1">{members.length} members</p>
        </div>
        {canManage && (
          <button
            onClick={() => setShowInvite(!showInvite)}
            className="flex items-center gap-2 px-4 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm"
          >
            <UserPlus className="w-4 h-4" />
            Invite Member
          </button>
        )}
      </div>

      {error && (
        <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{error}</div>
      )}
      {success && (
        <div className="mb-4 bg-accent-emerald/10 border border-accent-emerald/20 rounded-lg p-3 text-sm text-accent-emerald">{success}</div>
      )}

      {/* Invite Form */}
      {showInvite && (
        <form onSubmit={handleInvite} className="mb-6 bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
          <div className="flex items-end gap-4">
            <div className="flex-1">
              <label className="block text-sm font-medium text-dark-300 mb-1.5">Email Address</label>
              <input
                type="email"
                required
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-400 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                placeholder="teammate@example.com"
              />
            </div>
            <div className="w-36">
              <label className="block text-sm font-medium text-dark-300 mb-1.5">Role</label>
              <select
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
              >
                <option value="user">User</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <button
              type="submit"
              disabled={inviting}
              className="px-6 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 transition-all text-sm"
            >
              {inviting ? 'Sending...' : 'Send Invite'}
            </button>
          </div>
        </form>
      )}

      {/* Members Table */}
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-dark-800">
              <th className="text-left px-6 py-4 text-sm font-medium text-dark-400">Member</th>
              <th className="text-left px-6 py-4 text-sm font-medium text-dark-400">Role</th>
              <th className="text-left px-6 py-4 text-sm font-medium text-dark-400">Joined</th>
              {canManage && <th className="text-right px-6 py-4 text-sm font-medium text-dark-400">Actions</th>}
            </tr>
          </thead>
          <tbody>
            {members.map((member) => {
              const RoleIcon = roleIcons[member.role];
              const isMe = member.userId === user?.id;
              return (
                <tr key={member.userId} className="border-b border-dark-800/50 hover:bg-dark-800/30 transition-colors">
                  <td className="px-6 py-4">
                    <div>
                      <p className="text-sm font-medium text-white">
                        {member.displayName}
                        {isMe && <span className="text-dark-500 ml-2">(you)</span>}
                      </p>
                      <p className="text-xs text-dark-500">{member.email}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <RoleIcon className="w-4 h-4 text-dark-400" />
                      {isOwner && !isMe && member.role !== 'owner' ? (
                        <select
                          value={member.role}
                          onChange={(e) => handleChangeRole(member, e.target.value)}
                          className="bg-dark-800 border border-dark-700 rounded text-sm text-dark-300 px-2 py-1 focus:outline-none focus:border-primary-500"
                        >
                          <option value="user">User</option>
                          <option value="admin">Admin</option>
                        </select>
                      ) : (
                        <span className="text-sm text-dark-300 capitalize">{member.role}</span>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-dark-400">
                    {new Date(member.joinedAt).toLocaleDateString()}
                  </td>
                  {canManage && (
                    <td className="px-6 py-4 text-right">
                      {!isMe && member.role !== 'owner' && (
                        <button
                          onClick={() => setRemoveMember(member)}
                          className="text-dark-500 hover:text-red-400 transition-colors p-1"
                          title="Remove member"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Upgrade Modal */}
      {showUpgradeModal && (() => {
        const templateVars = { UserLimit: currentPlanUserLimit, PlanName: plans.find(p => p.id === currentPlanId)?.name || '' };
        const sortedByPrice = [...plans].sort((a, b) => a.monthlyPriceCents - b.monthlyPriceCents);
        const currentIdx = sortedByPrice.findIndex(p => p.id === currentPlanId);
        const recommendedPlan = sortedByPrice.slice(currentIdx + 1).find(p =>
          p.userLimit === 0 || p.userLimit > members.length
        );
        return (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full">
              <div className="flex items-center gap-3 mb-4">
                <Zap className="w-6 h-6 text-primary-400" />
                <h3 className="text-lg font-semibold text-white">
                  {renderTemplate(upgradePromptTitle, templateVars)}
                </h3>
              </div>
              <p className="text-dark-300 mb-6">
                {renderTemplate(upgradePromptBody, templateVars)}
              </p>
              <div className="flex gap-3 justify-end">
                <button
                  onClick={() => setShowUpgradeModal(false)}
                  className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    setShowUpgradeModal(false);
                    navigate(recommendedPlan ? `/plan?upgrade=${recommendedPlan.id}` : '/plan');
                  }}
                  className="px-4 py-2 text-sm bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
                >
                  Upgrade Plan
                </button>
              </div>
            </div>
          </div>
        );
      })()}

      <ConfirmModal
        open={removeMember !== null}
        onClose={() => setRemoveMember(null)}
        onConfirm={() => removeMember && handleRemove(removeMember)}
        title="Remove Team Member"
        message={`Are you sure you want to remove ${removeMember?.displayName} from the team?`}
        confirmLabel="Remove"
        confirmVariant="danger"
        loading={removeLoading}
      />
    </div>
  );
}
