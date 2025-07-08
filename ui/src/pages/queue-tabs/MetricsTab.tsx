import { BarChart3, Clock, Calendar, Filter, RefreshCw } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useQuery } from '@tanstack/react-query';
import { LineChart, Line, CartesianGrid, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { cn } from '@/lib/utils';
import { useState, useMemo } from 'react';
import React from 'react';
import { fetchAsynqmon, POLLING_INTERVAL, LoadingState, EmptyState, formatBytes, formatDuration } from './shared';

// Metrics Tab Component
export default function MetricsTab() {
    const [timeRange, setTimeRange] = useState<{ duration: number; endTime: 'realtime' | 'now' | 'custom' }>({
        duration: 3600,
        endTime: 'realtime'
    });
    const [selectedQueues, setSelectedQueues] = useState<string[]>(['default']);
    const [customDuration, setCustomDuration] = useState('3600');
    const [customEndTime, setCustomEndTime] = useState(new Date().toISOString().slice(0, 16));

    // Fetch available queues
    const { data: queuesData } = useQuery({
        queryKey: ['queues-for-filter'],
        queryFn: () => fetchAsynqmon('/queues'),
        refetchInterval: POLLING_INTERVAL * 6,
    });

    const availableQueues = queuesData?.queues?.map((q: any) => q.queue) || ['default'];

    // Calculate actual end time for API
    const endTime = timeRange.endTime === 'realtime' || timeRange.endTime === 'now'
        ? Math.floor(Date.now() / 1000)
        : Math.floor(new Date(customEndTime).getTime() / 1000);

    // 🚩 keep the key stable when realtime is on
    const keyEnd = timeRange.endTime === 'realtime' ? 'realtime' : endTime;

    const { 
        data: metricsData, 
        isInitialLoading,   // <= true only on the very first run
        isFetching          // <= true on every background refetch
    } = useQuery({
        queryKey: ['metrics', keyEnd, timeRange.duration, selectedQueues],
        queryFn: () => fetchAsynqmon(`/metrics?duration=${timeRange.duration}&endtime=${endTime}&queues=${selectedQueues.join(',')}`),
        // 🔑 the magic: never throw the old result away until the new one arrives
        placeholderData: (previousData) => previousData,
        refetchInterval: timeRange.endTime === 'realtime' ? POLLING_INTERVAL * 2 : 0,
    });

    const handleApplyFilters = () => {
        // Update duration if custom is selected
        const newDuration = timeRange.duration === parseInt(customDuration) ||
            (timeRange.duration !== 3600 &&
                timeRange.duration !== 14400 &&
                timeRange.duration !== 86400 &&
                timeRange.duration !== 691200 &&
                timeRange.duration !== 2592000)
            ? parseInt(customDuration)
            : timeRange.duration;

        setTimeRange({
            ...timeRange,
            duration: newDuration,
        });
    };

    const handleResetFilters = () => {
        setTimeRange({ duration: 3600, endTime: 'realtime' });
        setSelectedQueues(['default']);
        setCustomDuration('3600');
        setCustomEndTime(new Date().toISOString().slice(0, 16));
    };

    if (isInitialLoading && !metricsData) {
        return <MetricsSkeleton />;
    }

    if (!metricsData) {
        return <EmptyState icon={BarChart3} title="Metrics unavailable" />;
    }

    return (
        <div className="space-y-4">
            {/* Metrics Filter Bar */}
            <div className="flex items-center justify-between ">
                <div className="flex items-center gap-4">
                    <div className="flex items-center gap-2">
                        <Badge variant="outline" className="text-xs">
                            <Clock className="h-3 w-3 mr-1" />
                            {new Date(endTime * 1000).toLocaleString()}
                            {isFetching && <RefreshCw className="ml-2 h-3 w-3 animate-spin" />}
                        </Badge>
                        <Badge variant="outline" className="text-xs">
                            <Calendar className="h-3 w-3 mr-1" />
                            {formatDuration(timeRange.duration)}
                        </Badge>
                    </div>
                </div>
                <div className="flex items-center gap-2">
                    {/* Quick Time Range Buttons */}
                    <div className="flex items-center gap-1">
                        <Button
                            size="sm"
                            variant={timeRange.duration === 3600 && timeRange.endTime === 'realtime' ? 'default' : 'ghost'}
                            onClick={() => {
                                setTimeRange({ duration: 3600, endTime: 'realtime' });
                            }}
                            className="text-xs h-7 px-2"
                        >
                            1h
                        </Button>
                        <Button
                            size="sm"
                            variant={timeRange.duration === 14400 && timeRange.endTime === 'realtime' ? 'default' : 'ghost'}
                            onClick={() => {
                                setTimeRange({ duration: 14400, endTime: 'realtime' });
                            }}
                            className="text-xs h-7 px-2"
                        >
                            4h
                        </Button>
                        <Button
                            size="sm"
                            variant={timeRange.duration === 86400 && timeRange.endTime === 'realtime' ? 'default' : 'ghost'}
                            onClick={() => {
                                setTimeRange({ duration: 86400, endTime: 'realtime' });
                            }}
                            className="text-xs h-7 px-2"
                        >
                            1d
                        </Button>
                    </div>
                    <div className="w-px h-4 bg-border" />
                    
                    {/* Filter Popover */}
                    <Popover>
                        <PopoverTrigger asChild>
                            <Button
                                size="sm"
                                variant="outline"
                                className={cn(
                                    "text-xs h-7",
                                    (selectedQueues.length !== 1 || selectedQueues[0] !== 'default' ||
                                        timeRange.endTime === 'custom' ||
                                        ![3600, 14400, 86400, 691200, 2592000].includes(timeRange.duration)) && "bg-muted"
                                )}
                            >
                                <Filter className="h-3 w-3 mr-1" />
                                More
                                {(selectedQueues.length !== 1 || selectedQueues[0] !== 'default' ||
                                    timeRange.endTime === 'custom' ||
                                    ![3600, 14400, 86400, 691200, 2592000].includes(timeRange.duration)) && (
                                        <span className="ml-1 w-1.5 h-1.5 rounded-full bg-primary" />
                                    )}
                            </Button>
                        </PopoverTrigger>
                        <PopoverContent className="w-80">
                            <div className="space-y-4">
                                {/* Time Range Selection */}
                                <div className="space-y-3">
                                    <Label className="text-xs font-medium">End Time</Label>
                                    <div className="space-y-2">
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="endTime"
                                                checked={timeRange.endTime === 'realtime'}
                                                onChange={() => setTimeRange({ ...timeRange, endTime: 'realtime' })}
                                                className="w-3 h-3 text-primary focus:ring-primary focus:ring-1"
                                            />
                                            Real Time
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="endTime"
                                                checked={timeRange.endTime === 'now'}
                                                onChange={() => setTimeRange({ ...timeRange, endTime: 'now' })}
                                                className="w-3 h-3"
                                            />
                                            Start at now
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="endTime"
                                                checked={timeRange.endTime === 'custom'}
                                                onChange={() => setTimeRange({ ...timeRange, endTime: 'custom' })}
                                                className="w-3 h-3"
                                            />
                                            Custom End Time
                                        </label>
                                        {timeRange.endTime === 'custom' && (
                                            <Input
                                                type="datetime-local"
                                                value={customEndTime}
                                                onChange={(e) => setCustomEndTime(e.target.value)}
                                                className="text-xs h-7 mt-1"
                                            />
                                        )}
                                    </div>
                                </div>

                                {/* Duration Selection */}
                                <div className="space-y-3">
                                    <Label className="text-xs font-medium">Duration</Label>
                                    <div className="space-y-2">
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="duration"
                                                checked={timeRange.duration === 3600}
                                                onChange={() => setTimeRange({ ...timeRange, duration: 3600 })}
                                                className="w-3 h-3"
                                            />
                                            1h
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="duration"
                                                checked={timeRange.duration === 14400}
                                                onChange={() => setTimeRange({ ...timeRange, duration: 14400 })}
                                                className="w-3 h-3"
                                            />
                                            4h
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="duration"
                                                checked={timeRange.duration === 86400}
                                                onChange={() => setTimeRange({ ...timeRange, duration: 86400 })}
                                                className="w-3 h-3"
                                            />
                                            1 day
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="duration"
                                                checked={timeRange.duration === 691200}
                                                onChange={() => setTimeRange({ ...timeRange, duration: 691200 })}
                                                className="w-3 h-3"
                                            />
                                            8 days
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="duration"
                                                checked={timeRange.duration === 2592000}
                                                onChange={() => setTimeRange({ ...timeRange, duration: 2592000 })}
                                                className="w-3 h-3"
                                            />
                                            30 days
                                        </label>
                                        <label className="flex items-center gap-2 text-xs">
                                            <input
                                                type="radio"
                                                name="duration"
                                                checked={timeRange.duration === parseInt(customDuration) && timeRange.duration !== 3600 && timeRange.duration !== 14400 && timeRange.duration !== 86400 && timeRange.duration !== 691200 && timeRange.duration !== 2592000}
                                                onChange={() => setTimeRange({ ...timeRange, duration: parseInt(customDuration) })}
                                                className="w-3 h-3"
                                            />
                                            Custom Duration
                                        </label>
                                        <Input
                                            type="number"
                                            value={customDuration}
                                            onChange={(e) => setCustomDuration(e.target.value)}
                                            placeholder="Seconds"
                                            className="text-xs h-7 mt-1"
                                        />
                                    </div>
                                </div>

                                {/* Queue Selection */}
                                <div className="space-y-3">
                                    <Label className="text-xs font-medium">Queues</Label>
                                    <div className="space-y-2 max-h-32 overflow-y-auto">
                                        {availableQueues.map((queue: string) => (
                                            <label key={queue} className="flex items-center gap-2 text-xs hover:bg-muted p-1 rounded cursor-pointer">
                                                <input
                                                    type="checkbox"
                                                    checked={selectedQueues.includes(queue)}
                                                    onChange={(e) => {
                                                        if (e.target.checked) {
                                                            setSelectedQueues([...selectedQueues, queue]);
                                                        } else {
                                                            setSelectedQueues(selectedQueues.filter(q => q !== queue));
                                                        }
                                                    }}
                                                    className="w-3 h-3 rounded-sm border-border text-primary focus:ring-primary focus:ring-1"
                                                />
                                                {queue}
                                            </label>
                                        ))}
                                    </div>
                                </div>

                                {/* Action Buttons */}
                                <div className="flex items-center gap-2 pt-4 border-t border-border">
                                    <Button
                                        size="sm"
                                        onClick={handleApplyFilters}
                                        className="bg-primary hover:bg-primary/90 text-primary-foreground text-xs h-7 px-4"
                                    >
                                        Apply
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={handleResetFilters}
                                        className="text-xs h-7 px-4"
                                    >
                                        Reset
                                    </Button>
                                    <div className="ml-auto flex items-center gap-4 text-xs">
                                        <span className="text-muted-foreground">Realtime:</span>
                                        <Badge
                                            variant={timeRange.endTime === 'realtime' ? 'default' : 'secondary'}
                                            className={cn(
                                                "text-xs px-2 py-0",
                                                timeRange.endTime === 'realtime'
                                                    ? "bg-green-100 text-green-700 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800"
                                                    : "bg-muted text-muted-foreground"
                                            )}
                                        >
                                            {timeRange.endTime === 'realtime' ? 'ON' : 'OFF'}
                                        </Badge>
                                    </div>
                                </div>
                            </div>
                        </PopoverContent>
                    </Popover>
                </div>
            </div>

            {/* Queue Size Chart */}
            <MetricChart
                key={`queue-size-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                title="Queue Size Over Time"
                data={metricsData.queue_size}
                dataKey="queueSize"
                color="#3b82f6"
                gradientId="queueSize"
                syncId="metrics"
                timeRange={timeRange}
            />

            {/* Processing and Error Metrics */}
            <div className="grid gap-4 md:grid-cols-3">
                <MetricChart
                    key={`process-rate-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Tasks Processed/sec"
                    data={metricsData.tasks_processed_per_second}
                    dataKey="processRate"
                    color="#10b981"
                    gradientId="processRate"
                    syncId="metrics"
                    timeRange={timeRange}
                />
                <MetricChart
                    key={`fail-rate-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Tasks Failed/sec"
                    data={metricsData.tasks_failed_per_second}
                    dataKey="failRate"
                    color="#ef4444"
                    gradientId="failRate"
                    syncId="metrics"
                    timeRange={timeRange}
                />
                <MetricChart
                    key={`error-rate-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Error Rate"
                    data={metricsData.error_rate}
                    dataKey="errorRate"
                    color="#f59e0b"
                    gradientId="errorRate"
                    syncId="metrics"
                    timeRange={timeRange}
                />
            </div>

            {/* Memory and Latency */}
            <div className="grid gap-4 md:grid-cols-2">
                <MetricChart
                    key={`memory-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Memory Usage"
                    data={metricsData.queue_memory_usage_approx_bytes}
                    dataKey="memory"
                    color="#8b5cf6"
                    gradientId="memory"
                    syncId="metrics"
                    timeRange={timeRange}
                />
                <MetricChart
                    key={`latency-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Queue Latency"
                    data={metricsData.queue_latency_seconds}
                    dataKey="latency"
                    color="#f59e0b"
                    gradientId="latency"
                    syncId="metrics"
                    timeRange={timeRange}
                />
            </div>

            {/* Task Distribution Charts */}
            <div className="grid gap-4 md:grid-cols-3">
                <MetricChart
                    key={`pending-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Pending Tasks by Queue"
                    data={metricsData.pending_tasks_by_queue}
                    dataKey="pending"
                    color="#6366f1"
                    gradientId="pending"
                    syncId="metrics"
                    timeRange={timeRange}
                />
                <MetricChart
                    key={`retry-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Retry Tasks by Queue"
                    data={metricsData.retry_tasks_by_queue}
                    dataKey="retry"
                    color="#ec4899"
                    gradientId="retry"
                    syncId="metrics"
                    timeRange={timeRange}
                />
                <MetricChart
                    key={`archived-${timeRange.duration}-${timeRange.endTime}-${selectedQueues.sort().join(',')}`}
                    title="Archived Tasks by Queue"
                    data={metricsData.archived_tasks_by_queue}
                    dataKey="archived"
                    color="#8b5cf6"
                    gradientId="archived"
                    syncId="metrics"
                    timeRange={timeRange}
                />
            </div>
        </div>
    );
}

// Metric Chart Component
const MetricChart = React.memo(function MetricChart({ title, data, dataKey, color, gradientId, syncId, timeRange }: any) {
    if (!data?.data?.result || data.data.result.length === 0) {
        return (
            <Card>
                <CardHeader>
                    <CardTitle className="text-sm font-medium">{title}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="h-48 flex items-center justify-center text-xs text-muted-foreground">
                        No data available
                    </div>
                </CardContent>
            </Card>
        );
    }

    // Memoize the expensive data transformation
    const { chartData, queues } = useMemo(() => {
        const mergedData = new Map();

        data.data.result.forEach((series: any) => {
            const queueName = series.metric.queue || 'default';
            series.values.forEach(([timestamp, value]: [number, string]) => {
                const time = timestamp * 1000;
                if (!mergedData.has(time)) {
                    mergedData.set(time, { time });
                }
                const parsedValue = parseFloat(value);
                // Handle NaN values (common in error rate)
                mergedData.get(time)[`${dataKey}_${queueName}`] = isNaN(parsedValue) ? 0 : parsedValue;
            });
        });

        return {
            chartData: Array.from(mergedData.values()).sort((a, b) => a.time - b.time),
            queues: data.data.result.map((s: any) => s.metric.queue || 'default'),
        };
    }, [data, dataKey]);

    // Generate colors for multiple queues
    const queueColors = {
        default: color,
        critical: '#ef4444',
        low: '#3b82f6',
        scheduler: '#10b981',
    };

    // Format Y axis based on metric type
    const formatYAxis = (value: number) => {
        if (title.includes('Memory')) {
            return formatBytes(value);
        }
        if (title.includes('Error Rate') || title.includes('%')) {
            const percentage = value * 100;
            if (percentage === 0) return '0%';
            if (percentage < 0.01) return '<0.01%';
            if (percentage < 1) return `${percentage.toFixed(2)}%`;
            return `${percentage.toFixed(1)}%`;
        }
        if (title.includes('Latency')) {
            if (value < 0.001) return `${(value * 1000000).toFixed(0)}μs`;
            if (value < 1) return `${(value * 1000).toFixed(0)}ms`;
            return `${value.toFixed(2)}s`;
        }
        if (value > 1000) {
            return `${(value / 1000).toFixed(1)}k`;
        }
        if (value < 0.01 && value > 0) {
            return value.toExponential(2);
        }
        return value.toFixed(2);
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle className="text-sm font-medium">{title}</CardTitle>
                {queues.length > 1 && (
                    <p className="text-xs text-muted-foreground mt-1">
                        Showing: {queues.join(', ')}
                    </p>
                )}
            </CardHeader>
            <CardContent>
                <div className="h-48">
                    <ResponsiveContainer width="100%" height="100%">
                        <LineChart data={chartData} syncId={syncId} syncMethod="value">
                            <defs>
                                {queues.map((queue: string, index: number) => (
                                    <linearGradient key={queue} id={`${gradientId}_${queue}`} x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor={queueColors[queue as keyof typeof queueColors] || color} stopOpacity={0.3} />
                                        <stop offset="95%" stopColor={queueColors[queue as keyof typeof queueColors] || color} stopOpacity={0} />
                                    </linearGradient>
                                ))}
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                            <XAxis
                                dataKey="time"
                                type="number"
                                domain={['dataMin', 'dataMax']}
                                tick={{ fontSize: 11 }}
                                className="text-muted-foreground"
                                tickFormatter={(value) => new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                            />
                            <YAxis
                                tick={{ fontSize: 11 }}
                                className="text-muted-foreground"
                                tickFormatter={formatYAxis}
                            />
                            <Tooltip
                                labelFormatter={(value) => new Date(value).toLocaleString()}
                                formatter={(value: any, name: string) => [formatYAxis(value), name]}
                                contentStyle={{
                                    backgroundColor: 'hsl(var(--background))',
                                    border: '1px solid hsl(var(--border))',
                                    borderRadius: '6px',
                                    fontSize: '12px',
                                    color: 'hsl(var(--foreground))'
                                }}
                            />
                            {queues.length > 1 && (
                                <Legend
                                    wrapperStyle={{ fontSize: '11px', paddingTop: '10px' }}
                                    iconType="line"
                                />
                            )}
                            {queues.map((queue: string) => (
                                <Line
                                    key={queue}
                                    type="monotone"
                                    dataKey={`${dataKey}_${queue}`}
                                    name={queue}
                                    stroke={queueColors[queue as keyof typeof queueColors] || color}
                                    strokeWidth={2}
                                    fill={`url(#${gradientId}_${queue})`}
                                    dot={false}
                                />
                            ))}
                        </LineChart>
                    </ResponsiveContainer>
                </div>
            </CardContent>
        </Card>
    );
}, (prev, next) => 
    prev.data === next.data && 
    prev.dataKey === next.dataKey && 
    prev.color === next.color &&
    prev.syncId === next.syncId &&
    prev.timeRange === next.timeRange
);

// Metrics Skeleton Component
function MetricsSkeleton() {
    return (
        <div className="space-y-4">
            {/* Metrics Filter Bar Skeleton */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <div className="flex items-center gap-2">
                        <Skeleton className="h-5 w-32" />
                        <Skeleton className="h-5 w-24" />
                    </div>
                </div>
                <div className="flex items-center gap-2">
                    <div className="flex items-center gap-1">
                        <Skeleton className="h-7 w-8" />
                        <Skeleton className="h-7 w-8" />
                        <Skeleton className="h-7 w-8" />
                    </div>
                    <div className="w-px h-4 bg-border" />
                    <Skeleton className="h-7 w-16" />
                </div>
            </div>

            {/* Queue Size Chart Skeleton */}
            <Card>
                <CardHeader>
                    <Skeleton className="h-4 w-40" />
                </CardHeader>
                <CardContent>
                    <Skeleton className="h-48 w-full" />
                </CardContent>
            </Card>

            {/* Processing and Error Metrics Skeleton */}
            <div className="grid gap-4 md:grid-cols-3">
                {[1, 2, 3].map((i) => (
                    <Card key={i}>
                        <CardHeader>
                            <Skeleton className="h-4 w-32" />
                        </CardHeader>
                        <CardContent>
                            <Skeleton className="h-48 w-full" />
                        </CardContent>
                    </Card>
                ))}
            </div>

            {/* Memory and Latency Skeleton */}
            <div className="grid gap-4 md:grid-cols-2">
                {[1, 2].map((i) => (
                    <Card key={i}>
                        <CardHeader>
                            <Skeleton className="h-4 w-36" />
                        </CardHeader>
                        <CardContent>
                            <Skeleton className="h-48 w-full" />
                        </CardContent>
                    </Card>
                ))}
            </div>

            {/* Task Distribution Charts Skeleton */}
            <div className="grid gap-4 md:grid-cols-3">
                {[1, 2, 3].map((i) => (
                    <Card key={i}>
                        <CardHeader>
                            <Skeleton className="h-4 w-44" />
                        </CardHeader>
                        <CardContent>
                            <Skeleton className="h-48 w-full" />
                        </CardContent>
                    </Card>
                ))}
            </div>
        </div>
    );
}

 