import { Activity } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useQuery } from '@tanstack/react-query';
import { BarChart, Bar, CartesianGrid, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend, AreaChart, Area } from 'recharts';
import { fetchAsynqmon, POLLING_INTERVAL, LoadingState, formatBytes } from './shared';

// Queues Tab Component
export default function QueuesTab() {
    const { data: queuesData, isLoading } = useQuery({
        queryKey: ['queues'],
        queryFn: () => fetchAsynqmon('/queues'),
        refetchInterval: POLLING_INTERVAL,
    });

    const { data: queueStats } = useQuery({
        queryKey: ['queue-stats'],
        queryFn: () => fetchAsynqmon('/queue_stats'),
        refetchInterval: POLLING_INTERVAL,
    });

    if (isLoading) {
        return <LoadingState message="Loading queues..." />;
    }

    const queues = queuesData?.queues || [];

    return (
        <div className="space-y-6">
            {/* Queue Overview Cards */}
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                {queues.map((queue: any) => (
                    <Card key={queue.queue}>
                        <CardHeader className="pb-2">
                            <div className="flex items-center justify-between">
                                <CardTitle className="text-sm font-medium">{queue.queue}</CardTitle>
                                {queue.paused ? (
                                    <Badge variant="secondary" className="text-xs">Paused</Badge>
                                ) : (
                                    <Badge variant="default" className="text-xs bg-green-100 text-green-700 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800">Active</Badge>
                                )}
                            </div>
                        </CardHeader>
                        <CardContent className="space-y-2">
                            <div className="flex items-center justify-between text-xs">
                                <span className="text-muted-foreground">Total Tasks</span>
                                <span className="font-semibold text-foreground">{queue.size.toLocaleString()}</span>
                            </div>
                            <div className="flex items-center justify-between text-xs">
                                <span className="text-muted-foreground">Processed</span>
                                <span className="text-green-600 dark:text-green-400 font-medium">{queue.processed.toLocaleString()}</span>
                            </div>
                            <div className="flex items-center justify-between text-xs">
                                <span className="text-muted-foreground">Failed</span>
                                <span className="text-red-600 dark:text-red-400 font-medium">{queue.failed.toLocaleString()}</span>
                            </div>
                            <div className="flex items-center justify-between text-xs">
                                <span className="text-muted-foreground">Latency</span>
                                <span className="text-foreground">{queue.display_latency}</span>
                            </div>
                            <div className="flex items-center justify-between text-xs">
                                <span className="text-muted-foreground">Memory</span>
                                <span className="text-foreground">{formatBytes(queue.memory_usage_bytes)}</span>
                            </div>
                        </CardContent>
                    </Card>
                ))}
            </div>

            {/* Queue Stats Charts */}
            {queues.length > 0 && (
                <div className="grid gap-4 md:grid-cols-2">
                    {/* Queue Size Chart */}
                    <Card>
                        <CardHeader className="pb-3">
                            <CardTitle className="text-sm font-medium text-foreground flex items-center gap-2">
                                Queue Size
                                <span className="inline-flex items-center justify-center w-4 h-4 text-xs bg-muted text-muted-foreground rounded-full">
                                    ?
                                </span>
                            </CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="h-64 -mx-4 px-4">
                                <ResponsiveContainer width="100%" height="100%">
                                    <BarChart data={formatQueueSizeData(queues)} margin={{ top: 10, right: 10, left: 10, bottom: 10 }}>
                                        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                                        <XAxis
                                            dataKey="queue"
                                            tick={{ fontSize: 11 }}
                                            className="text-muted-foreground"
                                        />
                                        <YAxis tick={{ fontSize: 11 }} className="text-muted-foreground" />
                                        <Tooltip
                                            contentStyle={{
                                                backgroundColor: 'hsl(var(--background))',
                                                border: '1px solid hsl(var(--border))',
                                                borderRadius: '6px',
                                                fontSize: '12px',
                                                color: 'hsl(var(--foreground))'
                                            }}
                                        />
                                        <Legend
                                            wrapperStyle={{ fontSize: '11px', paddingTop: '10px' }}
                                            iconType="square"
                                        />
                                        <Bar dataKey="active" stackId="a" fill="#3b82f6" name="active" />
                                        <Bar dataKey="pending" stackId="a" fill="#6366f1" name="pending" />
                                        <Bar dataKey="aggregating" stackId="a" fill="#f59e0b" name="aggregating" />
                                        <Bar dataKey="scheduled" stackId="a" fill="#eab308" name="scheduled" />
                                        <Bar dataKey="retry" stackId="a" fill="#ec4899" name="retry" />
                                        <Bar dataKey="archived" stackId="a" fill="#8b5cf6" name="archived" />
                                        <Bar dataKey="completed" stackId="a" fill="#10b981" name="completed" />
                                    </BarChart>
                                </ResponsiveContainer>
                            </div>
                        </CardContent>
                    </Card>

                    {/* Tasks Processed Chart */}
                    <Card>
                        <CardHeader className="pb-3">
                            <div className="flex items-center justify-between">
                                <CardTitle className="text-sm font-medium text-foreground flex items-center gap-2">
                                    Tasks Processed
                                    <span className="inline-flex items-center justify-center w-4 h-4 text-xs bg-muted text-muted-foreground rounded-full">
                                        ?
                                    </span>
                                </CardTitle>
                                <select className="bg-background text-foreground text-xs px-2 py-1 rounded border border-border focus:outline-none focus:ring-1 focus:ring-ring">
                                    <option>TODAY</option>
                                    <option>WEEK</option>
                                    <option>MONTH</option>
                                </select>
                            </div>
                        </CardHeader>
                        <CardContent>
                            <div className="h-64 -mx-4 px-4">
                                <ResponsiveContainer width="100%" height="100%">
                                    <BarChart data={formatTasksProcessedData(queues)} margin={{ top: 10, right: 10, left: 10, bottom: 10 }}>
                                        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                                        <XAxis
                                            dataKey="queue"
                                            tick={{ fontSize: 11 }}
                                            className="text-muted-foreground"
                                        />
                                        <YAxis tick={{ fontSize: 11 }} className="text-muted-foreground" />
                                        <Tooltip
                                            contentStyle={{
                                                backgroundColor: 'hsl(var(--background))',
                                                border: '1px solid hsl(var(--border))',
                                                borderRadius: '6px',
                                                fontSize: '12px',
                                                color: 'hsl(var(--foreground))'
                                            }}
                                        />
                                        <Legend
                                            wrapperStyle={{ fontSize: '11px', paddingTop: '10px' }}
                                            iconType="square"
                                        />
                                        <Bar dataKey="succeeded" fill="#10b981" name="succeeded" />
                                        <Bar dataKey="failed" fill="#ef4444" name="failed" />
                                    </BarChart>
                                </ResponsiveContainer>
                            </div>
                        </CardContent>
                    </Card>
                </div>
            )}

            {/* Processing Trends Over Time */}
            {queueStats && formatQueueStats(queueStats).length > 0 && (
                <Card>
                    <CardHeader>
                        <CardTitle className="text-sm font-medium">Processing Trends (Last 30 Days)</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="h-64">
                            <ResponsiveContainer width="100%" height="100%">
                                <AreaChart data={formatQueueStats(queueStats)}>
                                    <defs>
                                        <linearGradient id="colorProcessed" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                                            <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                                        </linearGradient>
                                        <linearGradient id="colorFailed" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3} />
                                            <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                                        </linearGradient>
                                    </defs>
                                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                                    <XAxis
                                        dataKey="date"
                                        tick={{ fontSize: 11 }}
                                        className="text-muted-foreground"
                                        tickFormatter={(value) => new Date(value).toLocaleDateString('en', { month: 'short', day: 'numeric' })}
                                    />
                                    <YAxis tick={{ fontSize: 11 }} className="text-muted-foreground" />
                                    <Tooltip
                                        contentStyle={{
                                            backgroundColor: 'hsl(var(--background))',
                                            border: '1px solid hsl(var(--border))',
                                            borderRadius: '6px',
                                            fontSize: '12px',
                                            color: 'hsl(var(--foreground))'
                                        }}
                                    />
                                    <Legend
                                        wrapperStyle={{ fontSize: '11px', paddingTop: '10px' }}
                                        iconType="line"
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="processed"
                                        stroke="#10b981"
                                        fill="url(#colorProcessed)"
                                        name="Processed"
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="failed"
                                        stroke="#ef4444"
                                        fill="url(#colorFailed)"
                                        name="Failed"
                                    />
                                </AreaChart>
                            </ResponsiveContainer>
                        </div>
                    </CardContent>
                </Card>
            )}
        </div>
    );
}

// Helper function to format queue stats data
function formatQueueStats(stats: any) {
    if (!stats || !stats.processed || !stats.failed) return [];
    
    const processed = stats.processed.data?.result?.[0]?.values || [];
    const failed = stats.failed.data?.result?.[0]?.values || [];
    
    const dataMap = new Map();
    
    // Process processed tasks
    processed.forEach(([timestamp, value]: [number, string]) => {
        const date = new Date(timestamp * 1000).toISOString().split('T')[0];
        if (!dataMap.has(date)) {
            dataMap.set(date, { date, processed: 0, failed: 0 });
        }
        dataMap.get(date).processed = parseInt(value);
    });
    
    // Process failed tasks
    failed.forEach(([timestamp, value]: [number, string]) => {
        const date = new Date(timestamp * 1000).toISOString().split('T')[0];
        if (!dataMap.has(date)) {
            dataMap.set(date, { date, processed: 0, failed: 0 });
        }
        dataMap.get(date).failed = parseInt(value);
    });
    
    return Array.from(dataMap.values()).sort((a, b) => a.date.localeCompare(b.date));
}

// Helper function to format queue size data for charts
function formatQueueSizeData(queues: any[]) {
    return queues.map(queue => ({
        queue: queue.queue,
        active: queue.active,
        pending: queue.pending,
        aggregating: queue.aggregating,
        scheduled: queue.scheduled,
        retry: queue.retry,
        archived: queue.archived,
        completed: queue.completed
    }));
}

// Helper function to format tasks processed data for charts
function formatTasksProcessedData(queues: any[]) {
    return queues.map(queue => ({
        queue: queue.queue,
        succeeded: queue.processed,
        failed: queue.failed
    }));
} 