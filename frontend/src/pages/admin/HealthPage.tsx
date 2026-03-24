import { useEffect, useState, useCallback } from 'react';
import { Activity } from 'lucide-react';
import { adminApi } from '../../api/client';
import type { SystemNode, SystemMetric, TimeRange, NodeFilterMode, IntegrationCheck } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import NodeCards from './health/NodeCards';
import IntegrationsPanel from './health/IntegrationsPanel';
import CurrentStatusPanel from './health/CurrentStatusPanel';
import TimeRangeSelector from './health/TimeRangeSelector';
import MetricsCharts from './health/MetricsCharts';

export default function HealthPage() {
  const [nodes, setNodes] = useState<SystemNode[]>([]);
  const [currentMetrics, setCurrentMetrics] = useState<SystemMetric[]>([]);
  const [historicalMetrics, setHistoricalMetrics] = useState<SystemMetric[]>([]);
  const [timeRange, setTimeRange] = useState<TimeRange>('24h');
  const [filterMode, setFilterMode] = useState<NodeFilterMode>('aggregate');
  const [selectedNode, setSelectedNode] = useState('');
  const [integrations, setIntegrations] = useState<IntegrationCheck[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchCurrent = useCallback(async () => {
    try {
      const [nodesData, currentData, intData] = await Promise.all([
        adminApi.listHealthNodes(),
        adminApi.getHealthCurrent(),
        adminApi.getHealthIntegrations(),
      ]);
      setNodes(nodesData.nodes);
      setCurrentMetrics(currentData.metrics);
      setIntegrations(intData.integrations);
      if (!selectedNode && nodesData.nodes.length > 0) {
        setSelectedNode(nodesData.nodes[0].machineId);
      }
    } catch {
      // silently ignore
    }
  }, [selectedNode]);

  const fetchHistorical = useCallback(async () => {
    try {
      const params: { node?: string; range?: string } = { range: timeRange };
      if (filterMode === 'single' && selectedNode) {
        params.node = selectedNode;
      }
      const data = await adminApi.getHealthMetrics(params);
      setHistoricalMetrics(data.metrics);
    } catch {
      // silently ignore
    }
  }, [timeRange, filterMode, selectedNode]);

  // Initial load
  useEffect(() => {
    Promise.all([fetchCurrent(), fetchHistorical()]).finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Refetch historical when filters change
  useEffect(() => {
    if (!loading) fetchHistorical();
  }, [timeRange, filterMode, selectedNode]); // eslint-disable-line react-hooks/exhaustive-deps

  // Auto-refresh every 60s (pauses when tab is in background)
  useEffect(() => {
    const interval = setInterval(() => {
      if (document.visibilityState === 'visible') {
        fetchCurrent();
        fetchHistorical();
      }
    }, 60000);
    return () => clearInterval(interval);
  }, [fetchCurrent, fetchHistorical]);

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  return (
    <div>
      <div className="mb-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-emerald-500/20 flex items-center justify-center">
            <Activity className="w-5 h-5 text-emerald-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-dark-50">System Health</h1>
            <p className="text-dark-400 text-sm">Real-time server monitoring and metrics</p>
          </div>
        </div>
      </div>

      <div className="space-y-6">
        <NodeCards nodes={nodes} />
        <IntegrationsPanel integrations={integrations} />
        <CurrentStatusPanel metrics={currentMetrics} />
        <TimeRangeSelector
          timeRange={timeRange}
          onTimeRangeChange={setTimeRange}
          filterMode={filterMode}
          onFilterModeChange={setFilterMode}
          selectedNode={selectedNode}
          onSelectedNodeChange={setSelectedNode}
          nodes={nodes}
        />
        <MetricsCharts metrics={historicalMetrics} filterMode={filterMode} />
      </div>
    </div>
  );
}
