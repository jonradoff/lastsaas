import type { TimeRange, NodeFilterMode, SystemNode } from '../../../types';

interface TimeRangeSelectorProps {
  timeRange: TimeRange;
  onTimeRangeChange: (range: TimeRange) => void;
  filterMode: NodeFilterMode;
  onFilterModeChange: (mode: NodeFilterMode) => void;
  selectedNode: string;
  onSelectedNodeChange: (nodeId: string) => void;
  nodes: SystemNode[];
}

const timeRanges: { value: TimeRange; label: string }[] = [
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
];

const filterModes: { value: NodeFilterMode; label: string }[] = [
  { value: 'aggregate', label: 'Aggregate' },
  { value: 'all', label: 'All Nodes' },
  { value: 'single', label: 'Single Node' },
];

export default function TimeRangeSelector({
  timeRange, onTimeRangeChange,
  filterMode, onFilterModeChange,
  selectedNode, onSelectedNodeChange,
  nodes,
}: TimeRangeSelectorProps) {
  return (
    <div className="flex flex-wrap items-center gap-4">
      <div className="flex rounded-lg overflow-hidden border border-dark-700">
        {timeRanges.map((r) => (
          <button
            key={r.value}
            onClick={() => onTimeRangeChange(r.value)}
            className={`px-3 py-1.5 text-sm font-medium transition-colors ${
              timeRange === r.value
                ? 'bg-primary-500/20 text-primary-400'
                : 'text-dark-400 hover:text-dark-50 hover:bg-dark-800'
            }`}
          >
            {r.label}
          </button>
        ))}
      </div>

      <div className="flex rounded-lg overflow-hidden border border-dark-700">
        {filterModes.map((m) => (
          <button
            key={m.value}
            onClick={() => onFilterModeChange(m.value)}
            className={`px-3 py-1.5 text-sm font-medium transition-colors ${
              filterMode === m.value
                ? 'bg-primary-500/20 text-primary-400'
                : 'text-dark-400 hover:text-dark-50 hover:bg-dark-800'
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>

      {filterMode === 'single' && nodes.length > 0 && (
        <select
          value={selectedNode}
          onChange={(e) => onSelectedNodeChange(e.target.value)}
          className="bg-dark-800 border border-dark-700 rounded-lg px-3 py-1.5 text-sm text-dark-50"
        >
          {nodes.map((n) => (
            <option key={n.machineId} value={n.machineId}>
              {n.hostname} ({n.machineId.slice(0, 8)})
            </option>
          ))}
        </select>
      )}
    </div>
  );
}
