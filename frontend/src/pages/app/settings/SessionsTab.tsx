import { useEffect, useState } from 'react';
import { Monitor, Smartphone } from 'lucide-react';
import { toast } from 'sonner';
import { authApi } from '../../../api/client';
import { getErrorMessage } from '../../../utils/errors';
import type { ActiveSession } from '../../../types';
import LoadingSpinner from '../../../components/LoadingSpinner';
import ConfirmModal from '../../../components/ConfirmModal';

export default function SessionsTab() {
  const [sessions, setSessions] = useState<ActiveSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [confirmRevokeId, setConfirmRevokeId] = useState<string | null>(null);
  const [confirmRevokeAll, setConfirmRevokeAll] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);

  const loadSessions = () => {
    setSessionsLoading(true);
    authApi.listSessions()
      .then((data) => setSessions(data.sessions))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setSessionsLoading(false));
  };

  useEffect(() => { loadSessions(); }, []);

  const handleRevokeSession = async (id: string) => {
    setConfirmLoading(true);
    try {
      await authApi.revokeSession(id);
      setSessions(s => s.filter(session => session.id !== id));
      toast.success('Session revoked');
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setConfirmLoading(false);
      setConfirmRevokeId(null);
    }
  };

  const handleRevokeAllSessions = async () => {
    setConfirmLoading(true);
    try {
      await authApi.revokeAllSessions();
      loadSessions();
      toast.success('All other sessions revoked');
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setConfirmLoading(false);
      setConfirmRevokeAll(false);
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-dark-50 flex items-center gap-2">
            <Monitor className="w-5 h-5 text-dark-400" />
            Active Sessions
          </h2>
          {sessions.length > 1 && (
            <button
              onClick={() => setConfirmRevokeAll(true)}
              className="text-xs text-red-400 hover:text-red-300 transition-colors"
            >
              Revoke all other sessions
            </button>
          )}
        </div>

        {sessionsLoading ? (
          <LoadingSpinner size="sm" className="py-8" />
        ) : sessions.length === 0 ? (
          <p className="text-sm text-dark-400">No active sessions found.</p>
        ) : (
          <div className="space-y-3">
            {sessions.map(session => (
              <div key={session.id} className="flex items-center justify-between py-3 px-4 bg-dark-800/50 rounded-lg">
                <div className="flex items-start gap-3">
                  <Smartphone className="w-5 h-5 text-dark-400 mt-0.5 shrink-0" />
                  <div>
                    <p className="text-sm text-dark-50">
                      {session.deviceInfo || session.userAgent.slice(0, 50)}
                      {session.isCurrent && (
                        <span className="ml-2 px-2 py-0.5 bg-primary-500/20 text-primary-400 text-xs rounded">Current</span>
                      )}
                    </p>
                    <p className="text-xs text-dark-400">
                      {session.ipAddress} · Last active {new Date(session.lastActiveAt).toLocaleString()}
                    </p>
                  </div>
                </div>
                {!session.isCurrent && (
                  <button
                    onClick={() => setConfirmRevokeId(session.id)}
                    className="text-xs text-red-400 hover:text-red-300 transition-colors shrink-0"
                  >
                    Revoke
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      <ConfirmModal
        open={confirmRevokeId !== null}
        onClose={() => setConfirmRevokeId(null)}
        onConfirm={() => confirmRevokeId && handleRevokeSession(confirmRevokeId)}
        title="Revoke Session"
        message="This will sign out the device associated with this session. Are you sure?"
        confirmLabel="Revoke"
        confirmVariant="danger"
        loading={confirmLoading}
      />
      <ConfirmModal
        open={confirmRevokeAll}
        onClose={() => setConfirmRevokeAll(false)}
        onConfirm={handleRevokeAllSessions}
        title="Revoke All Sessions"
        message="This will sign out all other devices. You will remain signed in on this device."
        confirmLabel="Revoke All"
        confirmVariant="danger"
        loading={confirmLoading}
      />
    </div>
  );
}
