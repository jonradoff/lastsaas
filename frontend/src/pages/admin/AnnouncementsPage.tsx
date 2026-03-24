import { useState } from 'react';
import { Megaphone, Plus, Trash2, Eye, EyeOff } from 'lucide-react';
import { toast } from 'sonner';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { adminApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import type { Announcement } from '../../types';
import TableSkeleton from '../../components/TableSkeleton';
import ConfirmModal from '../../components/ConfirmModal';
import { useTenant } from '../../contexts/TenantContext';
import { Button, Input, Textarea, Badge, Card, Modal } from '../../components/ui';

export default function AnnouncementsPage() {
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';
  const queryClient = useQueryClient();

  const [showCreate, setShowCreate] = useState(false);
  const [editTarget, setEditTarget] = useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Announcement | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['announcements'],
    queryFn: () => adminApi.listAnnouncements(),
  });
  const announcements = data?.announcements ?? [];

  const deleteMutation = useMutation({
    mutationFn: (id: string) => adminApi.deleteAnnouncement(id),
    onSuccess: () => {
      toast.success('Announcement deleted');
      setDeleteTarget(null);
      queryClient.invalidateQueries({ queryKey: ['announcements'] });
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const toggleMutation = useMutation({
    mutationFn: (ann: Announcement) => adminApi.updateAnnouncement(ann.id, { publish: !ann.isPublished }),
    onSuccess: (_data, ann) => {
      toast.success(ann.isPublished ? 'Unpublished' : 'Published');
      queryClient.invalidateQueries({ queryKey: ['announcements'] });
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
            <Megaphone className="w-7 h-7 text-primary-400" />
            Announcements
          </h1>
          <p className="text-dark-400 mt-1">Manage changelog and announcements for users</p>
        </div>
        {canWrite && (
          <Button onClick={() => setShowCreate(true)} className="flex items-center gap-2">
            <Plus className="w-4 h-4" />
            New Announcement
          </Button>
        )}
      </div>

      {isLoading ? (
        <Card padding="none" className="overflow-hidden">
          <TableSkeleton rows={5} cols={4} />
        </Card>
      ) : announcements.length === 0 ? (
        <Card padding="lg" className="text-center text-dark-400">
          No announcements yet
        </Card>
      ) : (
        <div className="space-y-4">
          {announcements.map(ann => (
            <Card key={ann.id} className="p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="text-dark-50 font-semibold truncate">{ann.title}</h3>
                    <Badge variant={ann.isPublished ? 'success' : 'neutral'}>
                      {ann.isPublished ? 'Published' : 'Draft'}
                    </Badge>
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
                      onClick={() => toggleMutation.mutate(ann)}
                      className="p-2 text-dark-400 hover:text-dark-50 transition-colors"
                      aria-label={ann.isPublished ? 'Unpublish' : 'Publish'}
                    >
                      {ann.isPublished ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                    <Button variant="secondary" size="sm" onClick={() => setEditTarget(ann)}>
                      Edit
                    </Button>
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
            </Card>
          ))}
        </div>
      )}

      {canWrite && (showCreate || editTarget) && (
        <AnnouncementFormModal
          announcement={editTarget ?? undefined}
          onClose={() => { setShowCreate(false); setEditTarget(null); }}
          onSaved={() => {
            setShowCreate(false);
            setEditTarget(null);
            queryClient.invalidateQueries({ queryKey: ['announcements'] });
          }}
        />
      )}

      {canWrite && (
        <ConfirmModal
          open={deleteTarget !== null}
          onClose={() => setDeleteTarget(null)}
          onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
          title="Delete Announcement"
          message={`Are you sure you want to delete "${deleteTarget?.title}"?`}
          confirmLabel="Delete"
          confirmVariant="danger"
          loading={deleteMutation.isPending}
        />
      )}
    </div>
  );
}

const announcementSchema = z.object({
  title: z.string().trim().min(1, 'Title is required'),
  body: z.string().trim(),
  publish: z.boolean(),
});

type AnnouncementFormData = z.infer<typeof announcementSchema>;

function AnnouncementFormModal({ announcement, onClose, onSaved }: { announcement?: Announcement; onClose: () => void; onSaved: () => void }) {
  const isEdit = !!announcement;
  const { register, handleSubmit, formState: { errors } } = useForm<AnnouncementFormData>({
    resolver: zodResolver(announcementSchema),
    defaultValues: {
      title: announcement?.title ?? '',
      body: announcement?.body ?? '',
      publish: announcement?.isPublished ?? false,
    },
  });

  const mutation = useMutation({
    mutationFn: (data: AnnouncementFormData) => {
      if (isEdit) {
        return adminApi.updateAnnouncement(announcement!.id, { title: data.title, body: data.body, publish: data.publish });
      }
      return adminApi.createAnnouncement({ title: data.title, body: data.body || '', publish: data.publish });
    },
    onSuccess: () => {
      toast.success(isEdit ? 'Announcement updated' : 'Announcement created');
      onSaved();
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));

  return (
    <Modal open onClose={onClose} title={isEdit ? 'Edit Announcement' : 'New Announcement'}>
      <form onSubmit={onSubmit}>
        <div className="space-y-4">
          <Input
            label="Title"
            placeholder="What's new?"
            error={errors.title?.message}
            {...register('title')}
          />
          <Textarea
            label="Body (Markdown)"
            rows={6}
            placeholder="Describe the update..."
            {...register('body')}
          />
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              {...register('publish')}
              className="w-4 h-4 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500"
            />
            <span className="text-sm text-dark-300">Publish immediately</span>
          </label>
        </div>

        {mutation.error && <p className="text-sm text-red-400 mt-3">{getErrorMessage(mutation.error)}</p>}

        <div className="flex justify-end gap-3 mt-6">
          <Button variant="ghost" type="button" onClick={onClose}>Cancel</Button>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
