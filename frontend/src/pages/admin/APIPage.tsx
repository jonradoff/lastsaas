import { useEffect, useState, useCallback } from 'react';
import {
  Code2, Key, Webhook, Plus, Trash2, AlertTriangle, X, Copy, Check,
  Shield, User, ExternalLink, Pencil, Play, ChevronDown, ChevronUp,
  FileText, BookOpen,
} from 'lucide-react';
import { toast } from 'sonner';
import { adminApi } from '../../api/client';
import type { APIKey, Webhook as WebhookType, WebhookDelivery, WebhookEventTypeInfo } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';
import { useTenant } from '../../contexts/TenantContext';

// ─── Helpers ─────────────────────────────────────────────

function formatDate(dateStr?: string): string {
  if (!dateStr) return 'Never';
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function timeAgo(dateStr?: string): string {
  if (!dateStr) return 'Never';
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

// ─── API Documentation Section ──────────────────────────

function DocsSection() {
  const origin = window.location.origin;
  return (
    <div className="mb-10">
      <div className="flex items-center gap-2 mb-4">
        <BookOpen className="w-4 h-4 text-dark-400" />
        <h2 className="text-sm font-medium text-dark-400 uppercase tracking-wide">Documentation</h2>
      </div>
      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
        <p className="text-dark-300 text-sm mb-4">
          Complete API documentation is available in human-readable and markdown formats.
        </p>
        <div className="flex flex-wrap gap-3">
          <a
            href={`${origin}/api/docs`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-4 py-2.5 bg-primary-500/10 hover:bg-primary-500/20 border border-primary-500/20 rounded-xl text-sm text-primary-400 transition-colors"
          >
            <FileText className="w-4 h-4" />
            API Documentation
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
          <a
            href={`${origin}/api/docs/markdown`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-4 py-2.5 bg-dark-800 hover:bg-dark-700 border border-dark-700 rounded-xl text-sm text-dark-300 transition-colors"
          >
            <Code2 className="w-4 h-4" />
            Markdown Format
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        </div>
      </div>
    </div>
  );
}

// ─── API Keys Section ───────────────────────────────────

function CreateKeyModal({ onClose, onCreated }: {
  onClose: () => void;
  onCreated: (data: { apiKey: APIKey; rawKey: string }) => void;
}) {
  const [name, setName] = useState('');
  const [authority, setAuthority] = useState<'admin' | 'user'>('user');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const handleCreate = async () => {
    setSaving(true);
    setError('');
    try {
      const data = await adminApi.createAPIKey({ name: name.trim(), authority });
      onCreated(data);
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-lg">
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <h3 className="text-lg font-semibold text-white">Create API Key</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Name</label>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="e.g., CI/CD Pipeline"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 text-sm focus:outline-none focus:border-primary-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Authority Level</label>
            <select
              value={authority}
              onChange={e => setAuthority(e.target.value as 'admin' | 'user')}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:outline-none focus:border-primary-500"
            >
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
            <p className="mt-2 text-xs text-dark-500">
              {authority === 'admin'
                ? 'Admin keys can access all admin API endpoints (read-only admin actions, not owner-level).'
                : 'User keys can access tenant-scoped endpoints. Requires X-Tenant-ID header.'}
            </p>
          </div>
          {error && (
            <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">{error}</div>
          )}
        </div>
        <div className="flex justify-end gap-3 p-6 pt-0">
          <button onClick={onClose} className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors">Cancel</button>
          <button
            onClick={handleCreate}
            disabled={saving || !name.trim()}
            className="px-4 py-2 text-sm bg-primary-500 hover:bg-primary-600 text-white rounded-lg disabled:opacity-50 transition-colors"
          >
            {saving ? 'Creating...' : 'Create Key'}
          </button>
        </div>
      </div>
    </div>
  );
}

function RevealKeyModal({ rawKey, apiKey, onClose }: {
  rawKey: string;
  apiKey: APIKey;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(rawKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-lg">
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-emerald-500/20 flex items-center justify-center">
              <Check className="w-5 h-5 text-emerald-400" />
            </div>
            <h3 className="text-lg font-semibold text-white">API Key Created</h3>
          </div>
          <button onClick={onClose} className="text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <p className="text-sm text-dark-300">
            Your API key <span className="font-medium text-white">{apiKey.name}</span> has been created.
          </p>
          <div className="relative">
            <code className="block w-full p-3 bg-dark-800 border border-dark-700 rounded-lg text-sm text-emerald-400 font-mono break-all pr-12">
              {rawKey}
            </code>
            <button
              onClick={handleCopy}
              className="absolute top-2 right-2 p-1.5 bg-dark-700 hover:bg-dark-600 rounded-md transition-colors"
              title="Copy to clipboard"
            >
              {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4 text-dark-300" />}
            </button>
          </div>
          <div className="p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-lg">
            <p className="text-xs text-yellow-400 font-medium">
              This is the only time this key will be shown. Copy it now and store it securely.
            </p>
          </div>
          <div className="text-xs text-dark-500 space-y-1">
            <p>Use this key in your API requests:</p>
            <code className="block p-2 bg-dark-800 rounded text-dark-400 font-mono">
              Authorization: Bearer {rawKey.substring(0, 12)}...
            </code>
          </div>
        </div>
        <div className="flex justify-end p-6 pt-0">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm bg-primary-500 hover:bg-primary-600 text-white rounded-lg transition-colors"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

function APIKeysSection({ canWrite }: { canWrite: boolean }) {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [revealData, setRevealData] = useState<{ apiKey: APIKey; rawKey: string } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<APIKey | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  const fetchKeys = useCallback(async () => {
    try {
      const data = await adminApi.listAPIKeys();
      setKeys(data.apiKeys);
    } catch { /* ignore */ } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await adminApi.deleteAPIKey(deleteTarget.id);
      setDeleteTarget(null);
      fetchKeys();
    } catch (err: unknown) {
      setDeleteError(getErrorMessage(err));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="mb-10">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Key className="w-4 h-4 text-dark-400" />
          <h2 className="text-sm font-medium text-dark-400 uppercase tracking-wide">API Keys</h2>
          {!loading && <span className="text-xs text-dark-500">({keys.length})</span>}
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-primary-500 hover:bg-primary-600 text-white text-xs font-medium rounded-lg transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            Create Key
          </button>
        )}
      </div>

      {loading ? (
        <LoadingSpinner size="lg" className="py-12" />
      ) : keys.length === 0 ? (
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center">
          <Key className="w-8 h-8 text-dark-600 mx-auto mb-3" />
          <p className="text-dark-400 text-sm">{canWrite ? 'No API keys yet. Create one to get started.' : 'No API keys yet.'}</p>
        </div>
      ) : (
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-dark-800">
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Name</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Authority</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Key</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Created</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Last Used</th>
                <th className="text-right px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide"></th>
              </tr>
            </thead>
            <tbody>
              {keys.map(k => (
                <tr key={k.id} className="border-b border-dark-800/50 hover:bg-dark-800/30 transition-colors">
                  <td className="px-5 py-3 text-sm text-white font-medium">{k.name}</td>
                  <td className="px-5 py-3">
                    <span className={`inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full ${
                      k.authority === 'admin'
                        ? 'bg-primary-500/20 text-primary-400'
                        : 'bg-dark-700 text-dark-300'
                    }`}>
                      {k.authority === 'admin' ? <Shield className="w-3 h-3" /> : <User className="w-3 h-3" />}
                      {k.authority === 'admin' ? 'Admin' : 'User'}
                    </span>
                  </td>
                  <td className="px-5 py-3">
                    <code className="text-xs text-dark-500 font-mono">lsk_...{k.keyPreview}</code>
                  </td>
                  <td className="px-5 py-3 text-xs text-dark-400">{formatDate(k.createdAt)}</td>
                  <td className="px-5 py-3 text-xs text-dark-400">{k.lastUsedAt ? timeAgo(k.lastUsedAt) : 'Never'}</td>
                  <td className="px-5 py-3 text-right">
                    {canWrite && (
                      <button
                        onClick={() => setDeleteTarget(k)}
                        className="p-1.5 text-dark-500 hover:text-red-400 transition-colors"
                        title="Delete key"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CreateKeyModal
          onClose={() => setShowCreate(false)}
          onCreated={(data) => {
            setShowCreate(false);
            setRevealData(data);
            fetchKeys();
          }}
        />
      )}
      {revealData && (
        <RevealKeyModal
          rawKey={revealData.rawKey}
          apiKey={revealData.apiKey}
          onClose={() => setRevealData(null)}
        />
      )}
      {deleteTarget && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-md p-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 rounded-full bg-red-500/20 flex items-center justify-center">
                <AlertTriangle className="w-5 h-5 text-red-400" />
              </div>
              <h3 className="text-lg font-semibold text-white">Delete API Key?</h3>
            </div>
            <p className="text-dark-300 text-sm mb-2">
              Are you sure you want to delete <span className="font-medium text-white">{deleteTarget.name}</span>?
            </p>
            <p className="text-dark-400 text-xs mb-6">
              This action is irreversible. Any applications or scripts using this key will immediately lose access.
            </p>
            {deleteError && (
              <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">{deleteError}</div>
            )}
            <div className="flex justify-end gap-3">
              <button onClick={() => { setDeleteTarget(null); setDeleteError(''); }} className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors">Cancel</button>
              <button
                onClick={confirmDelete}
                disabled={deleting}
                className="px-4 py-2 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg disabled:opacity-50 transition-colors"
              >
                {deleting ? 'Deleting...' : 'Delete Key'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Webhooks Section ───────────────────────────────────

function RevealSecretModal({ secret, webhookName, onClose }: {
  secret: string;
  webhookName: string;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-lg">
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-emerald-500/20 flex items-center justify-center">
              <Check className="w-5 h-5 text-emerald-400" />
            </div>
            <h3 className="text-lg font-semibold text-white">Webhook Created</h3>
          </div>
          <button onClick={onClose} className="text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <p className="text-sm text-dark-300">
            Signing secret for <span className="font-medium text-white">{webhookName}</span>:
          </p>
          <div className="relative">
            <code className="block w-full p-3 bg-dark-800 border border-dark-700 rounded-lg text-sm text-emerald-400 font-mono break-all pr-12">
              {secret}
            </code>
            <button
              onClick={handleCopy}
              className="absolute top-2 right-2 p-1.5 bg-dark-700 hover:bg-dark-600 rounded-md transition-colors"
              title="Copy to clipboard"
            >
              {copied ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4 text-dark-300" />}
            </button>
          </div>
          <p className="text-xs text-dark-500">
            Use this secret to verify webhook signatures. You can always view or regenerate it from the webhook detail view.
          </p>
        </div>
        <div className="flex justify-end p-6 pt-0">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm bg-primary-500 hover:bg-primary-600 text-white rounded-lg transition-colors"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

function WebhookFormModal({ webhook, onClose, onSaved }: {
  webhook?: WebhookType;
  onClose: () => void;
  onSaved: (data: { webhook: WebhookType; secret?: string }) => void;
}) {
  const [name, setName] = useState(webhook?.name || '');
  const [description, setDescription] = useState(webhook?.description || '');
  const [url, setUrl] = useState(webhook?.url || '');
  const [events, setEvents] = useState<string[]>(webhook?.events || ['tenant.created']);
  const [eventTypes, setEventTypes] = useState<WebhookEventTypeInfo[]>([]);
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    adminApi.listWebhookEventTypes().then(d => {
      setEventTypes(d.eventTypes);
      // Auto-expand categories that have selected events
      const cats = new Set<string>();
      for (const et of d.eventTypes) {
        if ((webhook?.events || ['tenant.created']).includes(et.type as any)) {
          cats.add(et.category);
        }
      }
      setExpandedCategories(cats);
    }).catch(err => toast.error(getErrorMessage(err)));
  }, []);

  const toggleEvent = (type: string) => {
    setEvents(prev => prev.includes(type) ? prev.filter(e => e !== type) : [...prev, type]);
  };

  const toggleCategory = (category: string) => {
    setExpandedCategories(prev => {
      const next = new Set(prev);
      next.has(category) ? next.delete(category) : next.add(category);
      return next;
    });
  };

  const categoryEvents = (category: string) => eventTypes.filter(et => et.category === category);

  const toggleAllInCategory = (category: string) => {
    const catTypes = categoryEvents(category).map(et => et.type);
    const allSelected = catTypes.every(t => events.includes(t));
    if (allSelected) {
      setEvents(prev => prev.filter(e => !catTypes.includes(e)));
    } else {
      setEvents(prev => [...new Set([...prev, ...catTypes])]);
    }
  };

  // Ordered unique categories
  const categories = eventTypes.reduce<string[]>((acc, et) => {
    if (!acc.includes(et.category)) acc.push(et.category);
    return acc;
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const data = { name: name.trim(), description: description.trim(), url: url.trim(), events };
      if (webhook) {
        const result = await adminApi.updateWebhook(webhook.id, data);
        onSaved({ webhook: result.webhook });
      } else {
        const result = await adminApi.createWebhook(data);
        onSaved({ webhook: result.webhook, secret: result.secret });
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-lg max-h-[85vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <h3 className="text-lg font-semibold text-white">{webhook ? 'Edit Webhook' : 'Create Webhook'}</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Name</label>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="e.g., Provisioning Service"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 text-sm focus:outline-none focus:border-primary-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Description</label>
            <input
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="What this webhook does"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 text-sm focus:outline-none focus:border-primary-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1.5">Callback URL</label>
            <input
              value={url}
              onChange={e => setUrl(e.target.value)}
              placeholder="https://your-service.com/webhook"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 text-sm focus:outline-none focus:border-primary-500"
            />
          </div>
          {!webhook && (
            <div className="p-3 bg-dark-800/50 border border-dark-800 rounded-lg">
              <p className="text-xs text-dark-400">A signing secret will be automatically generated for HMAC-SHA256 signature verification.</p>
            </div>
          )}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm font-medium text-dark-300">Events</label>
              <span className="text-xs text-dark-500">{events.length} selected</span>
            </div>
            <div className="space-y-2">
              {categories.map(category => {
                const catTypes = categoryEvents(category);
                const selectedCount = catTypes.filter(et => events.includes(et.type)).length;
                const allSelected = selectedCount === catTypes.length;
                const someSelected = selectedCount > 0 && !allSelected;
                const expanded = expandedCategories.has(category);

                return (
                  <div key={category} className="border border-dark-700 rounded-lg overflow-hidden">
                    <div
                      className="flex items-center gap-3 p-3 bg-dark-800 cursor-pointer hover:bg-dark-750 transition-colors"
                      onClick={() => toggleCategory(category)}
                    >
                      <input
                        type="checkbox"
                        checked={allSelected}
                        ref={el => { if (el) el.indeterminate = someSelected; }}
                        onChange={(e) => { e.stopPropagation(); toggleAllInCategory(category); }}
                        onClick={(e) => e.stopPropagation()}
                        className="rounded border-dark-600 bg-dark-700 text-primary-500 focus:ring-primary-500"
                      />
                      <div className="flex-1 min-w-0">
                        <span className="text-sm font-medium text-white">{category}</span>
                        <span className="text-xs text-dark-500 ml-2">{selectedCount}/{catTypes.length}</span>
                      </div>
                      {expanded ? <ChevronUp className="w-4 h-4 text-dark-400" /> : <ChevronDown className="w-4 h-4 text-dark-400" />}
                    </div>
                    {expanded && (
                      <div className="border-t border-dark-700">
                        {catTypes.map(et => (
                          <label
                            key={et.type}
                            className="flex items-start gap-3 px-3 py-2.5 pl-10 cursor-pointer hover:bg-dark-800/50 transition-colors"
                          >
                            <input
                              type="checkbox"
                              checked={events.includes(et.type)}
                              onChange={() => toggleEvent(et.type)}
                              className="mt-0.5 rounded border-dark-600 bg-dark-700 text-primary-500 focus:ring-primary-500"
                            />
                            <div className="min-w-0">
                              <span className="text-sm text-white font-mono">{et.type}</span>
                              <p className="text-xs text-dark-400 mt-0.5">{et.description}</p>
                            </div>
                          </label>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
          {error && (
            <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">{error}</div>
          )}
        </div>
        <div className="flex justify-end gap-3 p-6 pt-0">
          <button onClick={onClose} className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors">Cancel</button>
          <button
            onClick={handleSave}
            disabled={saving || !name.trim() || !url.trim() || events.length === 0}
            className="px-4 py-2 text-sm bg-primary-500 hover:bg-primary-600 text-white rounded-lg disabled:opacity-50 transition-colors"
          >
            {saving ? 'Saving...' : webhook ? 'Save Changes' : 'Create Webhook'}
          </button>
        </div>
      </div>
    </div>
  );
}

function WebhookDetailModal({ webhookId, onClose, onRefresh, canWrite }: {
  webhookId: string;
  onClose: () => void;
  onRefresh: () => void;
  canWrite: boolean;
}) {
  const [hook, setHook] = useState<WebhookType | null>(null);
  const [secret, setSecret] = useState('');
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [loading, setLoading] = useState(true);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<WebhookDelivery | null>(null);
  const [editing, setEditing] = useState(false);
  const [expandedDelivery, setExpandedDelivery] = useState<string | null>(null);
  const [regenerating, setRegenerating] = useState(false);
  const [secretRevealed, setSecretRevealed] = useState(false);
  const [secretCopied, setSecretCopied] = useState(false);

  const fetchDetail = useCallback(async () => {
    try {
      const data = await adminApi.getWebhook(webhookId);
      setHook(data.webhook);
      setDeliveries(data.deliveries);
    } catch { /* ignore */ } finally {
      setLoading(false);
    }
  }, [webhookId]);

  useEffect(() => { fetchDetail(); }, [fetchDetail]);

  const handleTest = async () => {
    if (!hook) return;
    setTesting(true);
    setTestResult(null);
    try {
      const data = await adminApi.testWebhook(hook.id);
      setTestResult(data.delivery);
      fetchDetail();
    } catch { /* ignore */ } finally {
      setTesting(false);
    }
  };

  const handleRegenerate = async () => {
    if (!hook) return;
    setRegenerating(true);
    try {
      const data = await adminApi.regenerateWebhookSecret(hook.id);
      setSecret(data.secret);
      setHook({ ...hook, secretPreview: data.secretPreview });
      setSecretRevealed(true);
    } catch { /* ignore */ } finally {
      setRegenerating(false);
    }
  };

  const handleCopySecret = () => {
    navigator.clipboard.writeText(secret);
    setSecretCopied(true);
    setTimeout(() => setSecretCopied(false), 2000);
  };

  if (loading) return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-2xl p-12">
        <LoadingSpinner size="lg" className="py-4" />
      </div>
    </div>
  );

  if (!hook) return null;

  if (editing) {
    return (
      <WebhookFormModal
        webhook={hook}
        onClose={() => setEditing(false)}
        onSaved={(_data) => {
          setEditing(false);
          fetchDetail();
          onRefresh();
        }}
      />
    );
  }

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-2xl max-h-[85vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-dark-800">
          <div>
            <h3 className="text-lg font-semibold text-white">{hook.name}</h3>
            {hook.description && <p className="text-sm text-dark-400 mt-0.5">{hook.description}</p>}
          </div>
          <button onClick={onClose} className="text-dark-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="p-6 space-y-6">
          {/* Config summary */}
          <div className="space-y-4 text-sm">
            <div>
              <span className="text-dark-500 text-xs">URL</span>
              <p className="text-dark-300 font-mono text-xs mt-0.5 break-all">{hook.url}</p>
            </div>
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-dark-500 text-xs">Signing Secret</span>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setSecretRevealed(!secretRevealed)}
                    className="text-xs text-primary-400 hover:text-primary-300 transition-colors"
                  >
                    {secretRevealed ? 'Hide' : 'Reveal'}
                  </button>
                  {canWrite && (
                    <button
                      onClick={handleRegenerate}
                      disabled={regenerating}
                      className="text-xs text-dark-500 hover:text-dark-300 transition-colors disabled:opacity-50"
                    >
                      {regenerating ? 'Regenerating...' : 'Regenerate'}
                    </button>
                  )}
                </div>
              </div>
              {secretRevealed ? (
                <div className="relative">
                  <code className="block w-full p-2.5 bg-dark-800 border border-dark-700 rounded-lg text-xs text-emerald-400 font-mono break-all pr-10">
                    {secret}
                  </code>
                  <button
                    onClick={handleCopySecret}
                    className="absolute top-1.5 right-1.5 p-1.5 bg-dark-700 hover:bg-dark-600 rounded-md transition-colors"
                    title="Copy to clipboard"
                  >
                    {secretCopied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5 text-dark-300" />}
                  </button>
                </div>
              ) : (
                <code className="block w-full p-2.5 bg-dark-800 border border-dark-700 rounded-lg text-xs text-dark-500 font-mono">
                  whsec_••••••••••••••••••••••••••••{hook.secretPreview}
                </code>
              )}
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <span className="text-dark-500 text-xs">Events</span>
                <div className="flex flex-wrap gap-1 mt-0.5">
                  {hook.events.map(e => (
                    <span key={e} className="text-xs px-1.5 py-0.5 bg-dark-800 rounded font-mono text-dark-300">{e}</span>
                  ))}
                </div>
              </div>
              <div>
                <span className="text-dark-500 text-xs">Created</span>
                <p className="text-dark-300 text-xs mt-0.5">{formatDate(hook.createdAt)}</p>
              </div>
            </div>
          </div>

          {/* Actions */}
          {canWrite && (
            <div className="flex gap-2">
              <button
                onClick={() => setEditing(true)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-dark-800 hover:bg-dark-700 border border-dark-700 rounded-lg text-xs text-dark-300 transition-colors"
              >
                <Pencil className="w-3.5 h-3.5" />
                Edit
              </button>
              <button
                onClick={handleTest}
                disabled={testing}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-primary-500/10 hover:bg-primary-500/20 border border-primary-500/20 rounded-lg text-xs text-primary-400 transition-colors disabled:opacity-50"
              >
                <Play className="w-3.5 h-3.5" />
                {testing ? 'Sending...' : 'Send Test'}
              </button>
            </div>
          )}

          {/* Test result */}
          {testResult && (
            <div className={`p-4 rounded-xl border ${testResult.success ? 'bg-emerald-500/5 border-emerald-500/20' : 'bg-red-500/5 border-red-500/20'}`}>
              <p className={`text-sm font-medium ${testResult.success ? 'text-emerald-400' : 'text-red-400'}`}>
                Test {testResult.success ? 'succeeded' : 'failed'} — {testResult.responseCode || 'no response'} ({testResult.durationMs}ms)
              </p>
              {testResult.responseBody && (
                <pre className="mt-2 text-xs text-dark-400 font-mono whitespace-pre-wrap max-h-24 overflow-y-auto">{testResult.responseBody}</pre>
              )}
            </div>
          )}

          {/* Testing guide */}
          <div className="p-4 bg-dark-800/50 border border-dark-800 rounded-xl">
            <h4 className="text-xs font-medium text-dark-400 uppercase tracking-wide mb-2">Testing Your Webhook</h4>
            <ul className="text-xs text-dark-500 space-y-1.5">
              <li>1. Use the "Send Test" button to deliver a sample <code className="text-dark-400">tenant.created</code> event with test data.</li>
              <li>2. Test deliveries include an <code className="text-dark-400">X-Webhook-Test: true</code> header so your handler can distinguish them.</li>
              <li>3. Verify the <code className="text-dark-400">X-Webhook-Signature</code> header by computing <code className="text-dark-400">HMAC-SHA256(payload, secret)</code> and comparing against the header value.</li>
              <li>4. Your endpoint should return a 2xx status code to acknowledge receipt.</li>
              <li>5. For local development, use a tunnel service like ngrok to expose your local server.</li>
            </ul>
          </div>

          {/* Recent deliveries */}
          <div>
            <h4 className="text-xs font-medium text-dark-400 uppercase tracking-wide mb-3">Recent Deliveries</h4>
            {deliveries.length === 0 ? (
              <p className="text-xs text-dark-500">No deliveries yet.</p>
            ) : (
              <div className="space-y-2">
                {deliveries.map(d => (
                  <div key={d.id} className="border border-dark-800 rounded-lg overflow-hidden">
                    <button
                      onClick={() => setExpandedDelivery(expandedDelivery === d.id ? null : d.id)}
                      className="w-full flex items-center justify-between px-4 py-2.5 hover:bg-dark-800/50 transition-colors text-left"
                    >
                      <div className="flex items-center gap-3">
                        <span className={`w-2 h-2 rounded-full ${d.success ? 'bg-emerald-400' : 'bg-red-400'}`} />
                        <span className="text-xs font-mono text-dark-300">{d.eventType}</span>
                        <span className="text-xs text-dark-500">{d.responseCode || 'err'} &middot; {d.durationMs}ms{d.retryCount > 0 ? ` · retry ${d.retryCount}/${d.maxRetries}` : ''}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-dark-500">{timeAgo(d.createdAt)}</span>
                        {expandedDelivery === d.id ? <ChevronUp className="w-3.5 h-3.5 text-dark-500" /> : <ChevronDown className="w-3.5 h-3.5 text-dark-500" />}
                      </div>
                    </button>
                    {expandedDelivery === d.id && (
                      <div className="px-4 pb-3 space-y-2 border-t border-dark-800">
                        <div className="mt-2">
                          <span className="text-xs text-dark-500">Payload</span>
                          <pre className="mt-1 text-xs text-dark-400 font-mono whitespace-pre-wrap bg-dark-800 rounded p-2 max-h-32 overflow-y-auto">{
                            (() => { try { return JSON.stringify(JSON.parse(d.payload), null, 2); } catch { return d.payload; } })()
                          }</pre>
                        </div>
                        {d.responseBody && (
                          <div>
                            <span className="text-xs text-dark-500">Response</span>
                            <pre className="mt-1 text-xs text-dark-400 font-mono whitespace-pre-wrap bg-dark-800 rounded p-2 max-h-24 overflow-y-auto">{d.responseBody}</pre>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function WebhooksSection({ canWrite }: { canWrite: boolean }) {
  const [hooks, setHooks] = useState<WebhookType[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<WebhookType | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const [revealData, setRevealData] = useState<{ secret: string; webhookName: string } | null>(null);

  const fetchHooks = useCallback(async () => {
    try {
      const data = await adminApi.listWebhooks();
      setHooks(data.webhooks);
    } catch { /* ignore */ } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchHooks(); }, [fetchHooks]);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await adminApi.deleteWebhook(deleteTarget.id);
      setDeleteTarget(null);
      fetchHooks();
    } catch (err: unknown) {
      setDeleteError(getErrorMessage(err));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Webhook className="w-4 h-4 text-dark-400" />
          <h2 className="text-sm font-medium text-dark-400 uppercase tracking-wide">Webhooks</h2>
          {!loading && <span className="text-xs text-dark-500">({hooks.length})</span>}
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-primary-500 hover:bg-primary-600 text-white text-xs font-medium rounded-lg transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            Create Webhook
          </button>
        )}
      </div>

      {loading ? (
        <LoadingSpinner size="lg" className="py-12" />
      ) : hooks.length === 0 ? (
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8 text-center">
          <Webhook className="w-8 h-8 text-dark-600 mx-auto mb-3" />
          <p className="text-dark-400 text-sm">{canWrite ? 'No webhooks configured. Create one to receive event notifications.' : 'No webhooks configured.'}</p>
        </div>
      ) : (
        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-dark-800">
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Name</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">URL</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">Events</th>
                <th className="text-left px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide">24h Activity</th>
                <th className="text-right px-5 py-3 text-xs font-medium text-dark-400 uppercase tracking-wide"></th>
              </tr>
            </thead>
            <tbody>
              {hooks.map(h => (
                <tr
                  key={h.id}
                  className="border-b border-dark-800/50 hover:bg-dark-800/30 transition-colors cursor-pointer"
                  onClick={() => setDetailId(h.id)}
                >
                  <td className="px-5 py-3">
                    <span className="text-sm text-white font-medium">{h.name}</span>
                    {h.description && <p className="text-xs text-dark-500 mt-0.5">{h.description}</p>}
                  </td>
                  <td className="px-5 py-3">
                    <code className="text-xs text-dark-400 font-mono">{h.url.length > 40 ? h.url.substring(0, 40) + '...' : h.url}</code>
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex flex-wrap gap-1">
                      {h.events.map(e => (
                        <span key={e} className="text-xs px-1.5 py-0.5 bg-dark-700 rounded font-mono text-dark-300">{e}</span>
                      ))}
                    </div>
                  </td>
                  <td className="px-5 py-3">
                    <div className="text-sm text-white tabular-nums">{h.deliveries24h ?? 0} <span className="text-dark-500 text-xs">deliveries</span></div>
                    <div className="text-xs text-dark-500 mt-0.5">
                      {h.lastDelivery ? <>Last: {timeAgo(h.lastDelivery)}</> : 'No deliveries yet'}
                    </div>
                  </td>
                  <td className="px-5 py-3 text-right">
                    {canWrite && (
                      <button
                        onClick={(e) => { e.stopPropagation(); setDeleteTarget(h); }}
                        className="p-1.5 text-dark-500 hover:text-red-400 transition-colors"
                        title="Delete webhook"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <WebhookFormModal
          onClose={() => setShowCreate(false)}
          onSaved={(data) => {
            setShowCreate(false);
            fetchHooks();
            if (data.secret) {
              setRevealData({ secret: data.secret, webhookName: data.webhook.name });
            }
          }}
        />
      )}
      {revealData && (
        <RevealSecretModal
          secret={revealData.secret}
          webhookName={revealData.webhookName}
          onClose={() => setRevealData(null)}
        />
      )}
      {detailId && (
        <WebhookDetailModal
          webhookId={detailId}
          onClose={() => setDetailId(null)}
          onRefresh={fetchHooks}
          canWrite={canWrite}
        />
      )}
      {deleteTarget && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl w-full max-w-md p-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 rounded-full bg-red-500/20 flex items-center justify-center">
                <AlertTriangle className="w-5 h-5 text-red-400" />
              </div>
              <h3 className="text-lg font-semibold text-white">Delete Webhook?</h3>
            </div>
            <p className="text-dark-300 text-sm mb-2">
              Are you sure you want to delete <span className="font-medium text-white">{deleteTarget.name}</span>?
            </p>
            <p className="text-dark-400 text-xs mb-6">
              This webhook will stop receiving event notifications immediately.
            </p>
            {deleteError && (
              <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">{deleteError}</div>
            )}
            <div className="flex justify-end gap-3">
              <button onClick={() => { setDeleteTarget(null); setDeleteError(''); }} className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors">Cancel</button>
              <button
                onClick={confirmDelete}
                disabled={deleting}
                className="px-4 py-2 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg disabled:opacity-50 transition-colors"
              >
                {deleting ? 'Deleting...' : 'Delete Webhook'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Main Page ──────────────────────────────────────────

export default function APIPage() {
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white flex items-center gap-3">
          <Code2 className="w-7 h-7 text-primary-400" />
          API
        </h1>
        <p className="text-dark-400 mt-1">Documentation, keys, and webhook configuration</p>
      </div>

      <DocsSection />
      <APIKeysSection canWrite={canWrite} />
      <WebhooksSection canWrite={canWrite} />
    </div>
  );
}
