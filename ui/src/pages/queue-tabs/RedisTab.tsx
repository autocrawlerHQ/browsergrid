import { Database } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useQuery } from '@tanstack/react-query';
import { fetchAsynqmon, POLLING_INTERVAL, LoadingState, EmptyState } from './shared';

// Redis Tab Component
export default function RedisTab() {
    const { data: redisData, isLoading } = useQuery({
        queryKey: ['redis-info'],
        queryFn: () => fetchAsynqmon('/redis_info'),
        refetchInterval: POLLING_INTERVAL * 2, // Update less frequently
    });

    if (isLoading) {
        return <LoadingState message="Loading Redis info..." />;
    }

    if (!redisData) {
        return <EmptyState icon={Database} title="Redis info unavailable" />;
    }

    const info = redisData.info || {};

    return (
        <div className="space-y-4">
            <Card>
                <CardHeader>
                    <CardTitle className="text-sm font-medium">Redis Server Information</CardTitle>
                    <p className="text-xs text-muted-foreground">{redisData.address}</p>
                </CardHeader>
                <CardContent>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs">
                        <div>
                            <p className="text-muted-foreground">Version</p>
                            <p className="font-medium text-foreground">{info.redis_version || 'N/A'}</p>
                        </div>
                        <div>
                            <p className="text-muted-foreground">Uptime</p>
                            <p className="font-medium text-foreground">
                                {info.uptime_in_days ? `${info.uptime_in_days} days` : 'N/A'}
                            </p>
                        </div>
                        <div>
                            <p className="text-muted-foreground">Connected Clients</p>
                            <p className="font-medium text-foreground">{info.connected_clients || 0}</p>
                        </div>
                        <div>
                            <p className="text-muted-foreground">Used Memory</p>
                            <p className="font-medium text-foreground">{info.used_memory_human || 'N/A'}</p>
                        </div>
                    </div>
                </CardContent>
            </Card>

            <div className="grid gap-4 md:grid-cols-2">
                <Card>
                    <CardHeader>
                        <CardTitle className="text-sm font-medium">Performance</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-2">
                        <div className="flex justify-between text-xs">
                            <span className="text-muted-foreground">Commands Processed</span>
                            <span className="font-medium">{parseInt(info.total_commands_processed || 0).toLocaleString()}</span>
                        </div>
                        <div className="flex justify-between text-xs">
                            <span className="text-muted-foreground">Ops Per Second</span>
                            <span className="font-medium">{info.instantaneous_ops_per_sec || 0}</span>
                        </div>
                        <div className="flex justify-between text-xs">
                            <span className="text-muted-foreground">Hit Rate</span>
                            <span className="font-medium">
                                {info.keyspace_hits && info.keyspace_misses
                                    ? `${((parseInt(info.keyspace_hits) / (parseInt(info.keyspace_hits) + parseInt(info.keyspace_misses))) * 100).toFixed(1)}%`
                                    : 'N/A'
                                }
                            </span>
                        </div>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle className="text-sm font-medium">Persistence</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-2">
                        <div className="flex justify-between text-xs">
                            <span className="text-muted-foreground">Last Save</span>
                            <span className="font-medium">
                                {info.rdb_last_save_time
                                    ? new Date(parseInt(info.rdb_last_save_time) * 1000).toLocaleTimeString()
                                    : 'N/A'
                                }
                            </span>
                        </div>
                        <div className="flex justify-between text-xs">
                            <span className="text-muted-foreground">Changes Since Save</span>
                            <span className="font-medium">{info.rdb_changes_since_last_save || 0}</span>
                        </div>
                        <div className="flex justify-between text-xs">
                            <span className="text-muted-foreground">Save in Progress</span>
                            <span className="font-medium">{info.rdb_bgsave_in_progress === '1' ? 'Yes' : 'No'}</span>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}

 