import { Activity, Server, Database, BarChart3, Wifi } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

// Import tab components
import { QueuesTab, ServersTab, RedisTab, MetricsTab } from './queue-tabs';

// API configuration
const ASYNQMON_API = 'http://localhost:4444/api';
const POLLING_INTERVAL = 5000; // 5 seconds

// Custom fetch wrapper for Asynqmon API
const fetchAsynqmon = async (endpoint: string) => {
    const response = await fetch(`${ASYNQMON_API}${endpoint}`);
    if (!response.ok) throw new Error('Failed to fetch');
    return response.json();
};

export default function Queue() {
    const [activeTab, setActiveTab] = useState('queues');

    // Check if Prometheus is configured
    const { data: metricsAvailable } = useQuery({
        queryKey: ['metrics-availability'],
        queryFn: async () => {
            try {
                await fetchAsynqmon('/metrics?duration=300&endtime=' + Math.floor(Date.now() / 1000));
                return true;
            } catch {
                return false;
            }
        },
        refetchInterval: POLLING_INTERVAL * 6, // Check less frequently
    });

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-semibold text-foreground tracking-tight">
                        Task Queue Management
                    </h1>
                    <p className="text-sm text-muted-foreground mt-1">
                        Monitor and manage your background task processing
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <Wifi className="h-3 w-3 animate-pulse text-muted-foreground" />
                    <span className="text-xs text-muted-foreground">Live updates</span>
                </div>
            </div>

            {/* Tabs */}
            <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
                <TabsList className="grid w-full grid-cols-4 h-9">
                    <TabsTrigger value="queues" className="text-xs">
                        <Activity className="h-3 w-3 mr-1.5" />
                        Queues
                    </TabsTrigger>
                    <TabsTrigger value="servers" className="text-xs">
                        <Server className="h-3 w-3 mr-1.5" />
                        Servers
                    </TabsTrigger>
                    <TabsTrigger value="redis" className="text-xs">
                        <Database className="h-3 w-3 mr-1.5" />
                        Redis
                    </TabsTrigger>
                    {metricsAvailable && (
                        <TabsTrigger value="metrics" className="text-xs">
                            <BarChart3 className="h-3 w-3 mr-1.5" />
                            Metrics
                        </TabsTrigger>
                    )}
                </TabsList>

                <TabsContent value="queues" className="mt-4">
                    <QueuesTab />
                </TabsContent>

                <TabsContent value="servers" className="mt-4">
                    <ServersTab />
                </TabsContent>

                <TabsContent value="redis" className="mt-4">
                    <RedisTab />
                </TabsContent>

                {metricsAvailable && (
                    <TabsContent value="metrics" className="mt-4">
                        <MetricsTab />
                    </TabsContent>
                )}
            </Tabs>
        </div>
    );
}