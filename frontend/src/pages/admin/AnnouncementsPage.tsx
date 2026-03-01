import { useEffect, useState, useCallback } from 'react';
import { Megaphone, Plus, X, Trash2, Eye, EyeOff } from 'lucide-react';
import { toast } from 'sonner';
import { adminApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import type { Announcement } from '../../types';
import TableSkeleton from '../../components/TableSkeleton';
import ConfirmModal from '../../components/ConfirmModal';
import { useTenant } from '../../contexts/TenantContext';

export default function AnnouncementsPage() {
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';

  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [editTarget, setEditTarget] = useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchAnnouncements = useCallback(async () => {
    try {
      const data = await adminApi.listAnnouncements();
      setAnnouncements(data.announcements);
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchAnnouncements(); }, [fetchAnnouncements]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await adminApi.deleteAnnouncement(deleteTarget.id);
      toast.success('Announcement deleted');
      setDeleteTarget(null);
      fetchAnnouncements();
    } catch (err) {
      toast.error(getErrorMessage(err));
    } finally {
      setDeleting(false);
    }
  };

  const togglePublish = async (ann: Announcement) => {
    try {
      await adminApi.updateAnnouncement(ann.id, { publish: !ann.isPublished });
      toast.success(ann.isPublished ? 'Unpublished' : 'Published');
      fetchAnnouncements();
    } catch (err) {
      toast.error(getErrorMessage(err));
    }
  };

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Megaphone className="w-7 h-7 text-primary-400" />
            Announcements
          </h1>
          <p className="text-dark-400 mt-1">Manage changelog and announcements for users</p>
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 transition-colors"
          >
            <Plus className="w-4 h-4" />
            New Announcement
          </button>
        )}
      </div>

      {loading ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
          <TableSkeleton rows={5} cols={4} />
        </div>
      ) : announcements.length === 0 ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-12 text-center text-dark-400">
          No announcements yet
        </div>
      ) : (
        <div className="space-y-4">
          {announcements.map(ann => (
            <div key={ann.id} className="bg-dark-900/50 border border-dark-800 rounded-2xl p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="text-white font-semibold truncate">{ann.title}</h3>
                    <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                      ann.isPublished ? 'bg-accent-emerald/10 text-accent-emerald' : 'bg-dark-700 text-dark-400'
                    }`}>
                      {ann.isPublished ? 'Published' : 'Draft'}
                    </span>
                  </div>
                  {ann.body && (
                    <p className="text-sm text-dark-400 line-clamp-2">{ann.body}</p>
                  )}
                  <p className="text-xs text-dark-500 mt-2">
                    Created {new Date(ann.createdAt).toLocaleDateString()}
                    {ann.publishedAt && ` · Published ${new Date(ann.publishedAt).toLocaleDateString()}`}
                  </p>
                </div>
                {canWrite && (
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => togglePublish(ann)}
                      className="p-2 text-dark-400 hover:text-white transition-colors"
                      aria-label={ann.isPublished ? 'Unpublish' : 'Publish'}
                    >
                      {ann.isPublished ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => setEditTarget(ann)}
                      className="px-3 py-1.5 text-xs text-dark-300 hover:text-white bg-dark-800 rounded-lg transition-colors"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => setDeleteTarget(ann)}
                      className="p-2 text-dark-400 hover:text-red-400 transition-colors"
                      aria-label="Delete announcement"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {canWrite && (showCreate || editTarget) && (
        <AnnouncementFormModal
          announcement={editTarget ?? undefined}
          onClose={() => { setShowCreate(false); setEditTarget(null); }}
          onSaved={() => { setShowCreate(false); setEditTarget(null); fetchAnnouncements(); }}
        />
      )}

      {canWrite && (
        <ConfirmModal
          open={deleteTarget !== null}
          onClose={() => setDeleteTarget(null)}
          onConfirm={handleDelete}
          title="Delete Announcement"
          message={`Are you sure you want to delete "${deleteTarget?.title}"?`}
          confirmLabel="Delete"
          confirmVariant="danger"
          loading={deleting}
        />
      )}
    </div>
  );
}

function AnnouncementFormModal({ announcement, onClose, onSaved }: { announcement?: Announcement; onClose: () => void; onSaved: () => void }) {
  const isEdit = !!announcement;
  const [title, setTitle] = useState(announcement?.title ?? '');
  const [body, setBody] = useState(announcement?.body ?? '');
  const [publish, setPublish] = useState(announcement?.isPublished ?? false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const handleSave = async () => {
    if (!title.trim()) { setError('Title is required'); return; }
    setSaving(true);
    setError('');
    try {
      if (isEdit) {
        await adminApi.updateAnnouncement(announcement!.id, { title: title.trim(), body: body.trim(), publish });
      } else {
        await adminApi.createAnnouncement({ title: title.trim(), body: body.trim(), publish });
      }
      toast.success(isEdit ? 'Announcement updated' : 'Announcement created');
      onSaved();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-dark-900 rounded-2xl border border-dark-700 p-6 w-full max-w-lg" role="dialog" aria-modal="true">
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold text-white">{isEdit ? 'Edit Announcement' : 'New Announcement'}</h3>
          <button onClick={onClose} className="p-2 text-dark-400 hover:text-white transition-colors" aria-label="Close">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">Title</label>
            <input
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder="What's new?"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">Body (Markdown)</label>
            <textarea
              value={body}
              onChange={e => setBody(e.target.value)}
              rows={6}
              placeholder="Describe the update..."
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm focus:border-primary-500 focus:outline-none resize-none"
            />
          </div>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={publish}
              onChange={e => setPublish(e.target.checked)}
              className="w-4 h-4 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500"
            />
            <span className="text-sm text-dark-300">Publish immediately</span>
          </label>
        </div>

        {error && <p className="text-sm text-red-400 mt-3">{error}</p>}

        <div className="flex justify-end gap-3 mt-6">
          <button onClick={onClose} className="px-4 py-2 text-sm text-dark-400 hover:text-white transition-colors">
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !title.trim()}
            className="px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 disabled:opacity-50 transition-colors"
          >
            {saving ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
