import { useEffect, useState, useCallback } from 'react';
import { Settings, Search, Plus, Trash2, X, Shield, AlertTriangle } from 'lucide-react';
import { adminApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import type { ConfigVar, ConfigVarType, EnumOption } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { useTenant } from '../../contexts/TenantContext';

const typeLabels: Record<ConfigVarType, string> = {
  string: 'String',
  numeric: 'Numeric',
  enum: 'Enum',
  template: 'Template',
};

/** Parse options JSON into label/value pairs, supporting both formats. */
function parseEnumOptions(optionsStr?: string): EnumOption[] {
  if (!optionsStr) return [];
  try {
    const parsed = JSON.parse(optionsStr);
    if (!Array.isArray(parsed) || parsed.length === 0) return [];
    // label/value format
    if (typeof parsed[0] === 'object' && parsed[0].value !== undefined) {
      return parsed as EnumOption[];
    }
    // legacy string array
    return parsed.map((s: string) => ({ label: s, value: s }));
  } catch {
    return [];
  }
}

/** Serialize EnumOption[] back to JSON string. */
function serializeEnumOptions(opts: EnumOption[]): string {
  return JSON.stringify(opts);
}

export default function ConfigPage() {
  const { role } = useTenant();
  const canWrite = role === 'owner' || role === 'admin';

  const [configs, setConfigs] = useState<ConfigVar[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [editVar, setEditVar] = useState<ConfigVar | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ConfigVar | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchConfigs = useCallback(async () => {
    try {
      const data = await adminApi.listConfig();
      setConfigs(data.configs);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchConfigs(); }, [fetchConfigs]);

  const filtered = configs.filter(
    (c) =>
      c.name.toLowerCase().includes(filter.toLowerCase()) ||
      c.description.toLowerCase().includes(filter.toLowerCase())
  );

  const openEdit = (v: ConfigVar) => {
    setEditVar(v);
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await adminApi.deleteConfig(deleteTarget.name);
      setDeleteTarget(null);
      fetchConfigs();
    } catch (err: unknown) {
      alert(getErrorMessage(err));
      setDeleting(false);
    }
  };

  const truncateValue = (val: string, max = 80) => {
    const single = val.replace(/\n/g, ' ');
    return single.length > max ? single.slice(0, max - 3) + '...' : single;
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Settings className="w-7 h-7 text-primary-400" />
            Configuration
          </h1>
          <p className="text-dark-400 mt-1">{configs.length} variables</p>
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add Variable
          </button>
        )}
      </div>

      {/* Search */}
      <div className="relative mb-6">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-500" />
        <input
          type="text"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter by name or description..."
          className="w-full pl-10 pr-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500"
        />
      </div>

      {loading ? (
        <LoadingSpinner size="lg" className="py-20" />
      ) : filtered.length === 0 ? (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-12 text-center">
          <Settings className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <p className="text-dark-400">{filter ? 'No matching variables' : 'No configuration variables'}</p>
        </div>
      ) : (
        <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-dark-800">
                <th className="text-left px-4 py-3 text-dark-400 font-medium">Name</th>
                <th className="text-left px-4 py-3 text-dark-400 font-medium w-24">Type</th>
                <th className="text-left px-4 py-3 text-dark-400 font-medium">Value</th>
                <th className="text-right px-4 py-3 text-dark-400 font-medium w-20"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((v) => (
                <tr
                  key={v.id}
                  className="border-b border-dark-800/50 hover:bg-dark-800/30 cursor-pointer"
                  onClick={() => openEdit(v)}
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="text-white font-medium">{v.name}</span>
                      {v.isSystem && (
                        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium text-primary-400 bg-primary-500/10">
                          <Shield className="w-3 h-3" />
                          System
                        </span>
                      )}
                    </div>
                    {v.description && (
                      <p className="text-dark-500 text-xs mt-0.5">{v.description}</p>
                    )}
                  </td>
                  <td className="px-4 py-3 text-dark-400">{typeLabels[v.type] || v.type}</td>
                  <td className="px-4 py-3 text-dark-300 font-mono text-xs">
                    {truncateValue(v.value)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {canWrite && !v.isSystem && (
                      <button
                        onClick={(e) => { e.stopPropagation(); setDeleteTarget(v); }}
                        className="p-1.5 text-dark-500 hover:text-red-400 transition-colors"
                        title="Delete"
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

      {/* Edit Modal */}
      {editVar && (
        <EditConfigModal
          configVar={editVar}
          canWrite={canWrite}
          onSaved={() => { setEditVar(null); fetchConfigs(); }}
          onClose={() => setEditVar(null)}
        />
      )}

      {/* Create Modal */}
      {canWrite && showCreate && (
        <CreateConfigModal
          onClose={() => setShowCreate(false)}
          onCreated={() => { setShowCreate(false); fetchConfigs(); }}
        />
      )}

      {/* Delete Confirmation Modal */}
      {canWrite && deleteTarget && (
        <DeleteConfirmModal
          name={deleteTarget.name}
          deleting={deleting}
          onConfirm={confirmDelete}
          onCancel={() => { setDeleteTarget(null); setDeleting(false); }}
        />
      )}
    </div>
  );
}

/* ─── Delete Confirmation Modal ──────────────────────────────────────── */

function DeleteConfirmModal({
  name, deleting, onConfirm, onCancel,
}: {
  name: string; deleting: boolean; onConfirm: () => void; onCancel: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onCancel}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 w-full max-w-md mx-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-start gap-4 mb-4">
          <div className="flex-shrink-0 w-10 h-10 rounded-full bg-red-500/10 flex items-center justify-center">
            <AlertTriangle className="w-5 h-5 text-red-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">Delete Variable</h2>
            <p className="text-dark-400 text-sm mt-1">
              Are you sure you want to delete <span className="text-white font-medium">{name}</span>?
            </p>
          </div>
        </div>

        <div className="mb-6 px-3 py-2.5 bg-red-500/5 border border-red-500/15 rounded-lg">
          <p className="text-sm text-red-300/90">
            This action is permanent and cannot be undone. The variable and its value will be lost forever.
          </p>
        </div>

        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="px-4 py-2 bg-dark-800 border border-dark-700 text-dark-300 text-sm rounded-lg hover:text-white transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={deleting}
            className="px-4 py-2 bg-red-500 text-white text-sm font-medium rounded-lg hover:bg-red-600 disabled:opacity-50 transition-colors"
          >
            {deleting ? 'Deleting...' : 'Delete Variable'}
          </button>
        </div>
      </div>
    </div>
  );
}

/* ─── Enum Options Editor ────────────────────────────────────────────── */

function EnumOptionsEditor({
  options, onChange,
}: {
  options: EnumOption[]; onChange: (opts: EnumOption[]) => void;
}) {
  const updateOption = (index: number, field: 'label' | 'value', val: string) => {
    const updated = options.map((o, i) => i === index ? { ...o, [field]: val } : o);
    onChange(updated);
  };

  const addOption = () => {
    onChange([...options, { label: '', value: '' }]);
  };

  const removeOption = (index: number) => {
    onChange(options.filter((_, i) => i !== index));
  };

  return (
    <div>
      <label className="block text-sm font-medium text-dark-300 mb-2">Options</label>
      <div className="space-y-2">
        {options.map((opt, i) => (
          <div key={i} className="flex items-center gap-2">
            <input
              type="text"
              value={opt.label}
              onChange={(e) => updateOption(i, 'label', e.target.value)}
              placeholder="Label"
              className="flex-1 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500"
            />
            <input
              type="text"
              value={opt.value}
              onChange={(e) => updateOption(i, 'value', e.target.value)}
              placeholder="Value"
              className="flex-1 px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white font-mono placeholder-dark-500 focus:outline-none focus:border-primary-500"
            />
            <button
              onClick={() => removeOption(i)}
              className="p-1.5 text-dark-500 hover:text-red-400 transition-colors flex-shrink-0"
              title="Remove option"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        ))}
      </div>
      <button
        onClick={addOption}
        className="mt-2 flex items-center gap-1.5 text-xs text-primary-400 hover:text-primary-300 transition-colors"
      >
        <Plus className="w-3.5 h-3.5" />
        Add option
      </button>
    </div>
  );
}

/* ─── Edit Modal ─────────────────────────────────────────────────────── */

function EditConfigModal({
  configVar, canWrite, onSaved, onClose,
}: {
  configVar: ConfigVar;
  canWrite: boolean;
  onSaved: () => void;
  onClose: () => void;
}) {
  const [editValue, setEditValue] = useState(configVar.value);
  const [editDescription, setEditDescription] = useState(configVar.description);
  const [editEnumOptions, setEditEnumOptions] = useState<EnumOption[]>(parseEnumOptions(configVar.options));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const isNonSystem = !configVar.isSystem;
  const isEnum = configVar.type === 'enum';
  // For non-system enums, use the editable options for the value selector
  const displayOptions = isNonSystem && isEnum ? editEnumOptions : parseEnumOptions(configVar.options);

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      const opts: { description?: string; options?: string } = {};
      if (isNonSystem && editDescription !== configVar.description) {
        opts.description = editDescription;
      }
      if (isNonSystem && isEnum) {
        const serialized = serializeEnumOptions(editEnumOptions.filter(o => o.label.trim() && o.value.trim()));
        if (serialized !== configVar.options) {
          opts.options = serialized;
        }
      }
      await adminApi.updateConfig(configVar.name, editValue, Object.keys(opts).length > 0 ? opts : undefined);
      onSaved();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white">Edit: {configVar.name}</h2>
          <button onClick={onClose} className="text-dark-400 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Description — editable for non-system, read-only for system */}
        {isNonSystem ? (
          <div className="mb-4">
            <label className="block text-sm font-medium text-dark-300 mb-1">Description</label>
            <input
              type="text"
              value={editDescription}
              onChange={(e) => setEditDescription(e.target.value)}
              placeholder="What this variable controls"
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          </div>
        ) : configVar.description ? (
          <p className="text-dark-400 text-sm mb-4">{configVar.description}</p>
        ) : null}

        {/* Enum options editor for non-system enum vars */}
        {canWrite && isNonSystem && isEnum && (
          <div className="mb-4">
            <EnumOptionsEditor options={editEnumOptions} onChange={setEditEnumOptions} />
          </div>
        )}

        <div className="mb-4">
          <label className="block text-sm font-medium text-dark-300 mb-1">
            Value ({typeLabels[configVar.type]})
          </label>

          {isEnum ? (
            <select
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            >
              {displayOptions.filter(o => o.value.trim()).map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label || opt.value}</option>
              ))}
            </select>
          ) : configVar.type === 'template' ? (
            <textarea
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              rows={16}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white font-mono focus:outline-none focus:border-primary-500 resize-y disabled:opacity-60 disabled:cursor-not-allowed"
            />
          ) : configVar.type === 'numeric' ? (
            <input
              type="number"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              step="any"
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          ) : (
            <input
              type="text"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              disabled={!canWrite}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500 disabled:opacity-60 disabled:cursor-not-allowed"
            />
          )}
        </div>

        {error && (
          <div className="mb-4 px-3 py-2 bg-red-500/10 border border-red-500/20 rounded-lg text-sm text-red-400">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-dark-800 border border-dark-700 text-dark-300 text-sm rounded-lg hover:text-white transition-colors"
          >
            {canWrite ? 'Cancel' : 'Close'}
          </button>
          {canWrite && (
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 disabled:opacity-50 transition-colors"
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/* ─── Create Modal ───────────────────────────────────────────────────── */

function CreateConfigModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [type, setType] = useState<ConfigVarType>('string');
  const [value, setValue] = useState('');
  const [enumOptions, setEnumOptions] = useState<EnumOption[]>([{ label: '', value: '' }]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const handleCreate = async () => {
    if (!name.trim()) {
      setError('Name is required');
      return;
    }
    if (type === 'enum') {
      const valid = enumOptions.filter((o) => o.label.trim() && o.value.trim());
      if (valid.length === 0) {
        setError('At least one option with both label and value is required');
        return;
      }
    }
    setSaving(true);
    setError('');
    try {
      const optionsJSON = type === 'enum'
        ? serializeEnumOptions(enumOptions.filter((o) => o.label.trim() && o.value.trim()))
        : undefined;
      await adminApi.createConfig({
        name: name.trim(),
        description: description.trim(),
        type,
        value,
        options: optionsJSON,
      });
      onCreated();
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white">Add Variable</h2>
          <button onClick={onClose} className="text-dark-400 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. feature.max_items"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What this variable controls"
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">Type</label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value as ConfigVarType)}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
            >
              <option value="string">String</option>
              <option value="numeric">Numeric</option>
              <option value="enum">Enum</option>
              <option value="template">Template</option>
            </select>
          </div>

          {type === 'enum' && (
            <EnumOptionsEditor options={enumOptions} onChange={setEnumOptions} />
          )}

          <div>
            <label className="block text-sm font-medium text-dark-300 mb-1">
              {type === 'enum' ? 'Default Value' : 'Value'}
            </label>
            {type === 'enum' ? (
              <select
                value={value}
                onChange={(e) => setValue(e.target.value)}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
              >
                <option value="">Select a value...</option>
                {enumOptions.filter((o) => o.value.trim()).map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label || opt.value}</option>
                ))}
              </select>
            ) : type === 'template' ? (
              <textarea
                value={value}
                onChange={(e) => setValue(e.target.value)}
                rows={6}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white font-mono placeholder-dark-500 focus:outline-none focus:border-primary-500 resize-y"
              />
            ) : (
              <input
                type={type === 'numeric' ? 'number' : 'text'}
                value={value}
                onChange={(e) => setValue(e.target.value)}
                step={type === 'numeric' ? 'any' : undefined}
                className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-sm text-white placeholder-dark-500 focus:outline-none focus:border-primary-500"
              />
            )}
          </div>
        </div>

        {error && (
          <div className="mt-4 px-3 py-2 bg-red-500/10 border border-red-500/20 rounded-lg text-sm text-red-400">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-3 mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-dark-800 border border-dark-700 text-dark-300 text-sm rounded-lg hover:text-white transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleCreate}
            disabled={saving}
            className="px-4 py-2 bg-primary-500 text-white text-sm font-medium rounded-lg hover:bg-primary-600 disabled:opacity-50 transition-colors"
          >
            {saving ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}
