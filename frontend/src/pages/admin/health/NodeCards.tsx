import { Server } from 'lucide-react';
import type { SystemNode } from '../../../types';

interface NodeCardsProps {
  nodes: SystemNode[];
}

function timeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export default function NodeCards({ nodes }: NodeCardsProps) {
  if (nodes.length === 0) {
    return (
      <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-8 text-center text-dark-400">
        No nodes registered yet. Metrics will appear after the first collection cycle (~60s).
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {nodes.map((node) => (
        <div
          key={node.id}
          className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5"
        >
          <div className="flex items-start gap-3">
            <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${
              node.status === 'active' ? 'bg-emerald-500/20' : 'bg-yellow-500/20'
            }`}>
              <Server className={`w-5 h-5 ${
                node.status === 'active' ? 'text-emerald-400' : 'text-yellow-400'
              }`} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-medium text-dark-50 truncate">{node.hostname}</span>
                <span className={`inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full ${
                  node.status === 'active'
                    ? 'bg-emerald-500/20 text-emerald-400'
                    : 'bg-yellow-500/20 text-yellow-400'
                }`}>
                  <span className={`w-1.5 h-1.5 rounded-full ${
                    node.status === 'active' ? 'bg-emerald-400' : 'bg-yellow-400'
                  }`} />
                  {node.status}
                </span>
              </div>
              <p className="text-xs text-dark-500 mt-1 font-mono truncate">{node.machineId}</p>
            </div>
          </div>
          <div className="mt-3 grid grid-cols-2 gap-2 text-xs text-dark-400">
            <div>Version: <span className="text-dark-300">{node.version}</span></div>
            <div>Go: <span className="text-dark-300">{node.goVersion}</span></div>
            <div>Last seen: <span className="text-dark-300">{timeAgo(node.lastSeen)}</span></div>
            <div>Up since: <span className="text-dark-300">{timeAgo(node.startedAt)}</span></div>
          </div>
        </div>
      ))}
    </div>
  );
}
