import { Server } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useQuery } from '@tanstack/react-query';
import { cn } from '@/lib/utils';
import { fetchAsynqmon, POLLING_INTERVAL, LoadingState, EmptyState } from './shared';

// Servers Tab Component
export default function ServersTab() {
    const { data, isLoading } = useQuery({
        queryKey: ['servers'],
        queryFn: () => fetchAsynqmon('/servers'),
        refetchInterval: POLLING_INTERVAL,
    });

    if (isLoading) {
        return <LoadingState message="Loading servers..." />;
    }

    const servers = data?.servers || [];

    return (
        <div className="space-y-4">
            {servers.length === 0 ? (
                <EmptyState
                    icon={Server}
                    title="No servers found"
                    description="No worker servers are currently connected"
                />
            ) : (
                <div className="grid gap-4">
                    {servers.map((server: any) => (
                        <Card key={server.id}>
                            <CardHeader className="pb-3">
                                <div className="flex items-center justify-between">
                                    <div>
                                        <CardTitle className="text-sm font-medium">{server.host}</CardTitle>
                                        <p className="text-xs text-muted-foreground mt-0.5">
                                            Worker ID: <span className="font-mono">{server.id.substring(0, 8)}...</span>
                                        </p>
                                    </div>
                                    <Badge
                                        variant={server.status === 'active' ? 'default' : 'secondary'}
                                        className={cn(
                                            "text-xs",
                                            server.status === 'active' && "bg-green-100 text-green-700 border-green-200 dark:bg-green-900/20 dark:text-green-400 dark:border-green-800"
                                        )}
                                    >
                                        {server.status}
                                    </Badge>
                                </div>
                            </CardHeader>
                            <CardContent>
                                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs">
                                    <div>
                                        <p className="text-muted-foreground">Process ID</p>
                                        <p className="font-medium text-foreground">{server.pid}</p>
                                    </div>
                                    <div>
                                        <p className="text-muted-foreground">Concurrency</p>
                                        <p className="font-medium text-foreground">{server.concurrency}</p>
                                    </div>
                                    <div>
                                        <p className="text-muted-foreground">Started</p>
                                        <p className="font-medium text-foreground">
                                            {new Date(server.start_time).toLocaleTimeString()}
                                        </p>
                                    </div>
                                    <div>
                                        <p className="text-muted-foreground">Active Workers</p>
                                        <p className="font-medium text-foreground">{server.active_workers?.length || 0}</p>
                                    </div>
                                </div>

                                {server.queue_priorities && Object.keys(server.queue_priorities).length > 0 && (
                                    <div className="mt-3 pt-3 border-t border-border">
                                        <p className="text-xs text-muted-foreground mb-2">Queue Priorities</p>
                                        <div className="flex flex-wrap gap-2">
                                            {Object.entries(server.queue_priorities).map(([queue, priority]) => (
                                                <Badge key={queue} variant="outline" className="text-xs">
                                                    {queue}: {String(priority)}
                                                </Badge>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </CardContent>
                        </Card>
                    ))}
                </div>
            )}
        </div>
    );
}

 