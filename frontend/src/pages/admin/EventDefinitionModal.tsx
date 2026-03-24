import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useEffect } from 'react';
import Modal from '../../components/ui/Modal';
import type { EventDefinition } from '../../types';

const schema = z.object({
  name: z.string().min(1, 'Name is required').max(128).regex(/^[a-zA-Z0-9._-]+$/, 'Use alphanumeric, dots, underscores, or hyphens'),
  description: z.string().max(256),
  parentId: z.string(),
});

type FormData = z.infer<typeof schema>;

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: { name: string; description: string; parentId?: string | null }) => void;
  definitions: EventDefinition[];
  existing?: EventDefinition;
  loading?: boolean;
}

export default function EventDefinitionModal({ open, onClose, onSubmit, definitions, existing, loading }: Props) {
  const { register, handleSubmit, reset, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: existing?.name ?? '',
      description: existing?.description ?? '',
      parentId: existing?.parentId ?? '',
    },
  });

  useEffect(() => {
    if (open) {
      reset({
        name: existing?.name ?? '',
        description: existing?.description ?? '',
        parentId: existing?.parentId ?? '',
      });
    }
  }, [open, existing, reset]);

  const submit = (data: FormData) => {
    onSubmit({
      name: data.name,
      description: data.description || '',
      parentId: data.parentId || null,
    });
  };

  // Filter out self from parent options.
  const parentOptions = definitions.filter(d => d.id !== existing?.id);

  return (
    <Modal open={open} onClose={onClose} title={existing ? 'Edit Event Definition' : 'Define Event'}>
      <form onSubmit={handleSubmit(submit)} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-dark-300 mb-1">Name</label>
          <input
            {...register('name')}
            placeholder="e.g. checkout.started"
            className="w-full bg-dark-800 border border-dark-700 text-dark-50 rounded-lg px-3 py-2 text-sm focus:ring-primary-500 focus:border-primary-500"
          />
          {errors.name && <p className="text-red-400 text-xs mt-1">{errors.name.message}</p>}
        </div>

        <div>
          <label className="block text-sm font-medium text-dark-300 mb-1">Description</label>
          <input
            {...register('description')}
            placeholder="Short description of this event"
            className="w-full bg-dark-800 border border-dark-700 text-dark-50 rounded-lg px-3 py-2 text-sm focus:ring-primary-500 focus:border-primary-500"
          />
          {errors.description && <p className="text-red-400 text-xs mt-1">{errors.description.message}</p>}
        </div>

        <div>
          <label className="block text-sm font-medium text-dark-300 mb-1">Parent Event</label>
          <select
            {...register('parentId')}
            className="w-full bg-dark-800 border border-dark-700 text-dark-50 rounded-lg px-3 py-2 text-sm focus:ring-primary-500 focus:border-primary-500"
          >
            <option value="">None</option>
            {parentOptions.map(d => (
              <option key={d.id} value={d.id}>{d.name}</option>
            ))}
          </select>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm text-dark-400 hover:text-dark-50 transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="px-4 py-2 text-sm font-medium bg-primary-500 hover:bg-primary-600 disabled:opacity-60 text-white rounded-lg transition-colors"
          >
            {loading ? 'Saving...' : existing ? 'Update' : 'Create'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
