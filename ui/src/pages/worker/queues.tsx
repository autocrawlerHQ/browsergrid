import React, { useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell, LineChart, Line } from 'recharts';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { RefreshCw, Play, Pause, MoreHorizontal, Filter, TrendingUp, TrendingDown } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

// Assume these are the generated API hooks
import { 
  useGetApiV1MonitoringQueuesExtended,
  useGetApiV1MonitoringMetrics,
  usePostApiV1MonitoringQueuesNamePause,
  usePostApiV1MonitoringQueuesNameUnpause
} from '@/lib/api/monitoring/monitoring';
import { GetApiV1MonitoringMetricsRange, InternalMonitoringQueueStatsExtended } from '@/lib/api/model';

const queueColors: Record<string, string> = {
  'low': '#3b82f6',
  'default': '#8b5cf6',
  'critical': '#ef4444',
  'email': '#10b981',
  'high': '#f59e0b'
};

export default function Queues() {
  const [selectedTimeRange, setSelectedTimeRange] = useState('7d');
  const [selectedQueue, setSelectedQueue] = useState<string | null>(null);
  
  const { data: queues, isLoading, error, refetch } = useGetApiV1MonitoringQueuesExtended();
  const { data: metrics } = useGetApiV1MonitoringMetrics({
    range: selectedTimeRange as GetApiV1MonitoringMetricsRange,
    queues: []
  });
  
  const pauseQueue = usePostApiV1MonitoringQueuesNamePause();
  const unpauseQueue = usePostApiV1MonitoringQueuesNameUnpause();

  const handleQueueAction = async (queue: string, action: 'pause' | 'unpause') => {
    try {
      if (action === 'pause') {
        await pauseQueue.mutateAsync({ name: queue });
      } else {
        await unpauseQueue.mutateAsync({ name: queue });
      }
      refetch();
    } catch (err) {
      console.error(`Failed to ${action} queue:`, err);
    }
  };

  const formatBarChartData = () => {
    if (!queues) return [];
    
    return queues.map((q: InternalMonitoringQueueStatsExtended) => ({
      name: q.queue,
      active: q.active,
      pending: q.pending,
      scheduled: q.scheduled,
      retry: q.retry,
      archived: q.archived,
      total: q.size
    }));
  };

  const formatLineChartData = () => {
    if (!metrics?.tasks_processed) return [];
    
    return metrics.tasks_processed.map((point, index) => ({
      time: new Date(point.timestamp || '').toLocaleDateString(),
      succeeded: point.value,
      failed: metrics.tasks_failed?.[index]?.value || 0
    }));
  };

  const barChartData = formatBarChartData();
  const lineChartData = formatLineChartData();

  // Calculate totals
  const totalProcessed = queues?.reduce((sum: number, q: InternalMonitoringQueueStatsExtended) => sum + (q.processed || 0), 0) || 0;
  const totalFailed = queues?.reduce((sum: number, q: InternalMonitoringQueueStatsExtended) => sum + (q.failed || 0), 0) || 0;
  const overallErrorRate = totalProcessed > 0 ? (totalFailed / totalProcessed * 100) : 0;

  const LoadingSkeleton = () => (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <div className="h-6 w-32 bg-muted animate-pulse rounded" />
          </CardHeader>
          <CardContent>
            <div className="h-64 bg-muted animate-pulse rounded" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <div className="h-6 w-32 bg-muted animate-pulse rounded" />
          </CardHeader>
          <CardContent>
            <div className="h-64 bg-muted animate-pulse rounded" />
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <div className="h-6 w-32 bg-muted animate-pulse rounded" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-12 w-full bg-muted animate-pulse rounded" />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );

  if (isLoading) {
    return <LoadingSkeleton />;
  }

  if (error || !queues) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-3xl font-bold">Queues</h1>
            <p className="text-muted-foreground">
              Monitor and manage task queues
            </p>
          </div>
          <Button onClick={() => refetch()} variant="outline" size="sm">
            <RefreshCw className="h-4 w-4 mr-2" />
            Retry
          </Button>
        </div>
        <Alert variant="destructive">
          <AlertDescription>
            Failed to load queue data. Please check your connection and try again.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Queues</h1>
          <p className="text-muted-foreground">
            Monitor and manage task queues
          </p>
        </div>
        <Button onClick={() => refetch()} variant="outline" size="sm" disabled={isLoading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Queue Size Chart */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div className="flex items-center gap-2">
              <CardTitle>Queue Size</CardTitle>
              <Badge variant="outline" className="text-xs">Live</Badge>
            </div>
            <Select value="stacked" disabled>
              <SelectTrigger className="w-24 h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="stacked">Stacked</SelectItem>
              </SelectContent>
            </Select>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={barChartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e0e0e0" />
                <XAxis 
                  dataKey="name" 
                  stroke="#666"
                  style={{ fontSize: '12px' }}
                />
                <YAxis 
                  stroke="#666"
                  style={{ fontSize: '12px' }}
                />
                <Tooltip 
                  contentStyle={{ 
                    backgroundColor: 'rgba(255, 255, 255, 0.95)',
                    border: '1px solid #e0e0e0',
                    borderRadius: '4px'
                  }}
                />
                <Bar dataKey="active" stackId="a" fill="#3b82f6" name="Active" />
                <Bar dataKey="pending" stackId="a" fill="#8b5cf6" name="Pending" />
                <Bar dataKey="scheduled" stackId="a" fill="#10b981" name="Scheduled" />
                <Bar dataKey="retry" stackId="a" fill="#f59e0b" name="Retry" />
                <Bar dataKey="archived" stackId="a" fill="#ef4444" name="Archived" />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        {/* Tasks Processed Chart */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Tasks Processed</CardTitle>
            <Select value={selectedTimeRange} onValueChange={setSelectedTimeRange}>
              <SelectTrigger className="w-32 h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="1h">Last 1h</SelectItem>
                <SelectItem value="1d">Last 1d</SelectItem>
                <SelectItem value="7d">Last 7d</SelectItem>
              </SelectContent>
            </Select>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={lineChartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e0e0e0" />
                <XAxis 
                  dataKey="time" 
                  stroke="#666"
                  style={{ fontSize: '12px' }}
                />
                <YAxis 
                  stroke="#666"
                  style={{ fontSize: '12px' }}
                />
                <Tooltip 
                  contentStyle={{ 
                    backgroundColor: 'rgba(255, 255, 255, 0.95)',
                    border: '1px solid #e0e0e0',
                    borderRadius: '4px'
                  }}
                />
                <Line 
                  type="monotone" 
                  dataKey="succeeded" 
                  stroke="#10b981" 
                  strokeWidth={2}
                  dot={false}
                  name="Succeeded"
                />
                <Line 
                  type="monotone" 
                  dataKey="failed" 
                  stroke="#ef4444" 
                  strokeWidth={2}
                  dot={false}
                  name="Failed"
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      {/* Queue Table */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div className="flex items-center gap-4">
            <CardTitle>Queues</CardTitle>
            <Select value="all" disabled>
              <SelectTrigger className="w-24 h-8">
                <SelectValue placeholder="Filter" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b text-sm">
                  <th className="text-left p-2 font-medium">Queue</th>
                  <th className="text-left p-2 font-medium">State</th>
                  <th className="text-left p-2 font-medium">Size</th>
                  <th className="text-left p-2 font-medium">Memory usage</th>
                  <th className="text-left p-2 font-medium">Processed</th>
                  <th className="text-left p-2 font-medium">Failed</th>
                  <th className="text-left p-2 font-medium">Error rate</th>
                  <th className="text-left p-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {queues.map((queue: InternalMonitoringQueueStatsExtended) => (
                  <tr key={queue.queue} className="border-b hover:bg-muted/50">
                    <td className="p-2">
                      <div className="font-medium">{queue.queue}</div>
                    </td>
                    <td className="p-2">
                      <Badge 
                        variant={queue.state === 'paused' ? 'secondary' : 'default'}
                        className="text-xs"
                      >
                        {queue.state}
                      </Badge>
                    </td>
                    <td className="p-2">
                      <div className="font-mono text-sm">{queue.size?.toLocaleString()}</div>
                    </td>
                    <td className="p-2">
                      <div className="text-sm">{queue.memory_usage}</div>
                    </td>
                    <td className="p-2">
                      <div className="font-mono text-sm">{queue.processed?.toLocaleString()}</div>
                    </td>
                    <td className="p-2">
                      <div className="font-mono text-sm">{queue.failed?.toLocaleString()}</div>
                    </td>
                    <td className="p-2">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm">{queue.error_rate?.toFixed(2)}%</span>
                        {queue.error_rate && queue.error_rate > 5 ? (
                          <TrendingUp className="h-3 w-3 text-destructive" />
                        ) : (
                          <TrendingDown className="h-3 w-3 text-green-500" />
                        )}
                      </div>
                    </td>
                    <td className="p-2">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="sm">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          {queue.state === 'paused' ? (
                            <DropdownMenuItem onClick={() => handleQueueAction(queue.queue || '', 'unpause')}>
                              <Play className="h-4 w-4 mr-2" />
                              Resume
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem onClick={() => handleQueueAction(queue.queue || '', 'pause')}>
                              <Pause className="h-4 w-4 mr-2" />
                              Pause
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem>View Details</DropdownMenuItem>
                          <DropdownMenuItem className="text-destructive">Clear Archived</DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}