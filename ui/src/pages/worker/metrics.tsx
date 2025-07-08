import React, { useState } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { RefreshCw, Clock, TrendingUp, AlertTriangle } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';

// Assume these are the generated API hooks
import { useGetApiV1MonitoringMetrics } from '@/lib/api/monitoring/monitoring';
import { GetApiV1MonitoringMetricsRange } from '@/lib/api/model';

const timeRanges = [
    { value: '5m', label: '5 min' },
    { value: '30m', label: '30 min' },
    { value: '1h', label: '1 hour' },
    { value: '6h', label: '6 hours' },
    { value: '1d', label: '1 day' },
    { value: '7d', label: '7 days' },
];

export default function Metrics() {
    const [selectedRange, setSelectedRange] = useState('30m');
    const [selectedQueues, setSelectedQueues] = useState<string[]>([]);

    const { data: metrics, isLoading, error, refetch } = useGetApiV1MonitoringMetrics({
        range: selectedRange as GetApiV1MonitoringMetricsRange,
        queues: selectedQueues
    });

    const formatTimestamp = (timestamp: string) => {
        const date = new Date(timestamp);
        const now = new Date();
        const diffHours = (now.getTime() - date.getTime()) / (1000 * 60 * 60);

        if (diffHours < 1) {
            return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
        } else if (diffHours < 24) {
            return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
        } else {
            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        }
    };

    const formatChartData = () => {
        if (!metrics) return [];

        // Combine all data points by timestamp
        const dataMap = new Map();

        metrics.tasks_processed?.forEach((point) => {
            if (!point.timestamp) return;
            const key = formatTimestamp(point.timestamp);
            if (!dataMap.has(key)) {
                dataMap.set(key, { timestamp: point.timestamp, time: key });
            }
            dataMap.get(key).processed = point.value;
        });

        metrics.tasks_failed?.forEach((point) => {
            if (!point.timestamp) return;
            const key = formatTimestamp(point.timestamp);
            if (!dataMap.has(key)) {
                dataMap.set(key, { timestamp: point.timestamp, time: key });
            }
            dataMap.get(key).failed = point.value;
        });

        metrics.error_rate?.forEach((point) => {
            if (!point.timestamp) return;
            const key = formatTimestamp(point.timestamp);
            if (!dataMap.has(key)) {
                dataMap.set(key, { timestamp: point.timestamp, time: key });
            }
            dataMap.get(key).errorRate = point.value;
        });

        return Array.from(dataMap.values()).sort((a, b) =>
            new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
        );
    };

    const chartData = formatChartData();

    // Calculate summary stats
    const latestStats = chartData[chartData.length - 1] || {};
    const avgProcessed = chartData.reduce((sum, d) => sum + (d.processed || 0), 0) / chartData.length || 0;
    const avgFailed = chartData.reduce((sum, d) => sum + (d.failed || 0), 0) / chartData.length || 0;
    const avgErrorRate = chartData.reduce((sum, d) => sum + (d.errorRate || 0), 0) / chartData.length || 0;

    const LoadingSkeleton = () => (
        <div className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                {[1, 2, 3].map((i) => (
                    <Card key={i}>
                        <CardHeader className="space-y-0 pb-2">
                            <div className="h-4 w-24 bg-muted animate-pulse rounded" />
                        </CardHeader>
                        <CardContent>
                            <div className="h-8 w-16 bg-muted animate-pulse rounded mb-2" />
                            <div className="h-3 w-32 bg-muted animate-pulse rounded" />
                        </CardContent>
                    </Card>
                ))}
            </div>
            {[1, 2, 3].map((i) => (
                <Card key={i}>
                    <CardHeader>
                        <div className="h-6 w-32 bg-muted animate-pulse rounded" />
                    </CardHeader>
                    <CardContent>
                        <div className="h-64 bg-muted animate-pulse rounded" />
                    </CardContent>
                </Card>
            ))}
        </div>
    );

    if (isLoading) {
        return <LoadingSkeleton />;
    }

    if (error) {
        return (
            <div className="space-y-6">
                <div className="flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold">Metrics</h1>
                        <p className="text-muted-foreground">
                            Monitor task processing performance and error rates
                        </p>
                    </div>
                    <Button onClick={() => refetch()} variant="outline" size="sm">
                        <RefreshCw className="h-4 w-4 mr-2" />
                        Retry
                    </Button>
                </div>
                <Alert variant="destructive">
                    <AlertTriangle className="h-4 w-4" />
                    <AlertDescription>
                        Failed to load metrics data. Please check your connection and try again.
                    </AlertDescription>
                </Alert>
            </div>
        );
    }

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-3xl font-bold">Metrics</h1>
                    <p className="text-muted-foreground">
                        Monitor task processing performance and error rates
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <span className="text-sm text-muted-foreground">Realtime:</span>
                    <Select value={selectedRange} onValueChange={setSelectedRange}>
                        <SelectTrigger className="w-32">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {timeRanges.map((range) => (
                                <SelectItem key={range.value} value={range.value}>
                                    {range.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Button onClick={() => refetch()} variant="outline" size="sm" disabled={isLoading}>
                        <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
                        {selectedRange === '5m' ? '1h' : selectedRange}
                    </Button>
                </div>
            </div>

            {/* Summary Cards */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <Card>
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Avg Tasks Processed</CardTitle>
                        <TrendingUp className="h-4 w-4 text-muted-foreground" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">{avgProcessed.toFixed(2)}</div>
                        <p className="text-xs text-muted-foreground">
                            Per interval ({selectedRange})
                        </p>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Avg Tasks Failed</CardTitle>
                        <AlertTriangle className="h-4 w-4 text-muted-foreground" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">{avgFailed.toFixed(3)}</div>
                        <p className="text-xs text-muted-foreground">
                            Per interval ({selectedRange})
                        </p>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Avg Error Rate</CardTitle>
                        <AlertTriangle className="h-4 w-4 text-muted-foreground" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">{avgErrorRate.toFixed(2)}%</div>
                        <p className="text-xs text-muted-foreground">
                            Failed / Total processed
                        </p>
                    </CardContent>
                </Card>
            </div>

            {/* Tasks Processed Chart */}
            <Card>
                <CardHeader className="flex flex-row items-center justify-between">
                    <div className="flex items-center gap-2">
                        <CardTitle>Tasks Processed</CardTitle>
                        <Badge variant="outline" className="text-xs">
                            <Clock className="h-3 w-3 mr-1" />
                            {selectedRange}
                        </Badge>
                    </div>
                    <div className="flex gap-4 text-sm">
                        <div className="flex items-center gap-2">
                            <div className="w-3 h-3 bg-blue-500 rounded" />
                            <span>default</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <div className="w-3 h-3 bg-green-500 rounded" />
                            <span>email</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <div className="w-3 h-3 bg-orange-500 rounded" />
                            <span>low</span>
                        </div>
                    </div>
                </CardHeader>
                <CardContent>
                    <ResponsiveContainer width="100%" height={300}>
                        <LineChart data={chartData}>
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
                                dataKey="processed"
                                stroke="#3b82f6"
                                strokeWidth={2}
                                dot={false}
                                name="Processed"
                            />
                        </LineChart>
                    </ResponsiveContainer>
                </CardContent>
            </Card>

            {/* Tasks Failed Chart */}
            <Card>
                <CardHeader>
                    <CardTitle>Tasks Failed</CardTitle>
                </CardHeader>
                <CardContent>
                    <ResponsiveContainer width="100%" height={300}>
                        <LineChart data={chartData}>
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

            {/* Error Rate Chart */}
            <Card>
                <CardHeader>
                    <CardTitle>Error Rate</CardTitle>
                </CardHeader>
                <CardContent>
                    <ResponsiveContainer width="100%" height={300}>
                        <LineChart data={chartData}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#e0e0e0" />
                            <XAxis
                                dataKey="time"
                                stroke="#666"
                                style={{ fontSize: '12px' }}
                            />
                            <YAxis
                                stroke="#666"
                                style={{ fontSize: '12px' }}
                                tickFormatter={(value) => `${value}%`}
                            />
                            <Tooltip
                                contentStyle={{
                                    backgroundColor: 'rgba(255, 255, 255, 0.95)',
                                    border: '1px solid #e0e0e0',
                                    borderRadius: '4px'
                                }}
                                formatter={(value: any) => `${value?.toFixed(2)}%`}
                            />
                            <Line
                                type="monotone"
                                dataKey="errorRate"
                                stroke="#f59e0b"
                                strokeWidth={2}
                                dot={false}
                                name="Error Rate"
                            />
                        </LineChart>
                    </ResponsiveContainer>
                </CardContent>
            </Card>
        </div>
    );
}