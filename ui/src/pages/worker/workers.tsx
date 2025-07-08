import { RefreshCw, Activity, Clock, AlertCircle, CheckCircle, Pause } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Table, TableHeader, TableBody, TableRow, TableCell, TableHead } from '@/components/ui/table';

import { useGetApiV1MonitoringServers } from '@/lib/api/monitoring/monitoring';
import type { AsynqServerInfo, AsynqWorkerInfo } from '@/lib/api/model';

// Define types matching the new snake_case API response
interface WorkerInfoSnakeCase {
  task_id?: string;
  task_type?: string;
  queue?: string;
  started?: string;
  deadline?: string;
  task_payload?: any;
}

interface ServerInfoSnakeCase {
  id?: string;
  host?: string;
  pid?: number;
  concurrency?: number;
  queues?: Record<string, number>;
  strict_priority?: boolean;
  started?: string;
  status?: string;
  active_workers?: WorkerInfoSnakeCase[];
}

export default function Workers() {
  const { data: serversResponse, isLoading, error, refetch } = useGetApiV1MonitoringServers();
  
  // Extract servers array from the response
  if (!serversResponse) {
    return <div>No servers found</div>;
  }
  const servers: ServerInfoSnakeCase[] = serversResponse;

  const formatUptime = (startTime: string): string => {
    try {
      const start = new Date(startTime);
      const now = new Date();
      const diffMs = now.getTime() - start.getTime();
      
      if (diffMs < 0) return 'N/A';
      
      const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
      const diffMinutes = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
      
      if (diffHours > 24) {
        const days = Math.floor(diffHours / 24);
        const hours = diffHours % 24;
        return `${days}d ${hours}h`;
      } else if (diffHours > 0) {
        return `${diffHours}h ${diffMinutes}m`;
      }
      return `${diffMinutes}m`;
    } catch {
      return 'N/A';
    }
  };

  const getServerStatus = (server: ServerInfoSnakeCase) => {
    const activeWorkers = server.active_workers?.length || 0;
    const concurrency = server.concurrency || 0;
    
    // Check if server is actually running
    if (server.status !== 'active') {
      return { 
        status: 'offline', 
        variant: 'destructive' as const,
        icon: AlertCircle 
      };
    }
    
    if (activeWorkers === 0) {
      return { 
        status: 'idle', 
        variant: 'secondary' as const,
        icon: Pause 
      };
    } else if (activeWorkers >= concurrency) {
      return { 
        status: 'busy', 
        variant: 'destructive' as const,
        icon: AlertCircle 
      };
    } else {
      return { 
        status: 'active', 
        variant: 'default' as const,
        icon: CheckCircle 
      };
    }
  };

  const getTotalActiveWorkers = (): number => {
    return servers?.reduce((total: number, server: ServerInfoSnakeCase) => total + (server.active_workers?.length || 0), 0);
  };

  const getTotalCapacity = (): number => {
    return servers?.reduce((total: number, server: ServerInfoSnakeCase) => total + (server.concurrency || 0), 0);
  };

  const getUtilizationPercentage = (): number => {
    const capacity = getTotalCapacity();
    const active = getTotalActiveWorkers();
    return capacity > 0 ? Math.round((active / capacity) * 100) : 0;
  };

  // Loading skeleton component
  const LoadingSkeleton = () => (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {[1, 2, 3].map((i) => (
          <Card key={i}>
            <CardHeader className="space-y-0 pb-2">
              <div className="h-4 w-24 bg-muted animate-pulse rounded" />
            </CardHeader>
            <CardContent>
              <div className="h-8 w-12 bg-muted animate-pulse rounded mb-2" />
              <div className="h-3 w-32 bg-muted animate-pulse rounded" />
            </CardContent>
          </Card>
        ))}
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

  if (error) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-3xl font-bold">Workers</h1>
            <p className="text-muted-foreground">
              Monitor your Asynq worker servers and active tasks
            </p>
          </div>
          <Button onClick={() => refetch()} variant="outline" size="sm">
            <RefreshCw className="h-4 w-4 mr-2" />
            Retry
          </Button>
        </div>
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            Failed to load worker data. Please check your connection and try again.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  // Handle empty state
  if (servers.length === 0) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-3xl font-bold">Workers</h1>
            <p className="text-muted-foreground">
              Monitor your Asynq worker servers and active tasks
            </p>
          </div>
          <Button onClick={() => refetch()} variant="outline" size="sm">
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
        </div>
        <Alert>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            No worker servers found. Make sure your workers are running and connected.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Workers</h1>
          <p className="text-muted-foreground">
            Monitor your Asynq worker servers and active tasks
          </p>
        </div>
        <Button onClick={() => refetch()} variant="outline" size="sm" disabled={isLoading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Servers</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{servers.length}</div>
            <p className="text-xs text-muted-foreground">
              Active worker servers
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Workers</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{getTotalActiveWorkers()}</div>
            <p className="text-xs text-muted-foreground">
              Currently processing tasks
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Capacity</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{getTotalCapacity()}</div>
            <p className="text-xs text-muted-foreground">
              Maximum concurrent tasks
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Utilization</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{getUtilizationPercentage()}%</div>
            <p className="text-xs text-muted-foreground">
              Current worker utilization
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Servers Table */}
      <Card>
        <CardHeader>
          <CardTitle>Worker Servers</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b">
                  <th className="text-left p-2 font-medium">Server ID</th>
                  <th className="text-left p-2 font-medium">Host</th>
                  <th className="text-left p-2 font-medium">Status</th>
                  <th className="text-left p-2 font-medium">Utilization</th>
                  <th className="text-left p-2 font-medium">Queues</th>
                  <th className="text-left p-2 font-medium">Uptime</th>
                  <th className="text-left p-2 font-medium">PID</th>
                </tr>
              </thead>
              <tbody>
                {servers.map((server: ServerInfoSnakeCase) => {
                  const serverStatus = getServerStatus(server);
                  const activeWorkers = server.active_workers?.length || 0;
                  const capacity = server.concurrency || 0;
                  const queues = server.queues ? Object.entries(server.queues) : [];
                  const StatusIcon = serverStatus.icon;
                  
                  return (
                    <tr key={server.id} className="border-b hover:bg-muted/50">
                      <td className="p-2">
                        <div className="font-mono text-sm">
                          {server.id ? server.id.slice(0, 8) + '...' : 'N/A'}
                        </div>
                      </td>
                      <td className="p-2">
                        <div className="font-medium">{server.host || 'Unknown'}</div>
                      </td>
                      <td className="p-2">
                        <Badge variant={serverStatus.variant} className="flex items-center gap-1 w-fit">
                          <StatusIcon className="h-3 w-3" />
                          {serverStatus.status}
                        </Badge>
                      </td>
                      <td className="p-2">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium">{activeWorkers}/{capacity}</span>
                          <div className="w-16 bg-muted rounded-full h-2">
                            <div 
                              className={`h-2 rounded-full transition-all duration-300 ${
                                activeWorkers >= capacity 
                                  ? 'bg-destructive' 
                                  : activeWorkers > 0 
                                    ? 'bg-primary' 
                                    : 'bg-muted-foreground'
                              }`}
                              style={{ 
                                width: `${capacity > 0 ? Math.min((activeWorkers / capacity) * 100, 100) : 0}%` 
                              }}
                            />
                          </div>
                          <span className="text-xs text-muted-foreground">
                            {capacity > 0 ? Math.round((activeWorkers / capacity) * 100) : 0}%
                          </span>
                        </div>
                      </td>
                      <td className="p-2">
                        <div className="flex flex-wrap gap-1">
                          {queues.length > 0 ? (
                            queues.map(([queueName, count]) => (
                              <Badge 
                                key={queueName} 
                                variant="outline" 
                                className="text-xs"
                                title={`${count} workers`}
                              >
                                {queueName} ({count})
                              </Badge>
                            ))
                          ) : (
                            <span className="text-xs text-muted-foreground">No queues</span>
                          )}
                        </div>
                      </td>
                      <td className="p-2">
                        <div className="text-sm">
                          {server.started ? formatUptime(server.started) : 'N/A'}
                        </div>
                      </td>
                      <td className="p-2">
                        <div className="font-mono text-sm">{server.pid || 'N/A'}</div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Active Workers Details */}
      {servers.some((server: ServerInfoSnakeCase) => server.active_workers && server.active_workers.length > 0) && (
        <Card>
          <CardHeader>
            <CardTitle>Active Workers</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Server</TableHead>
                    <TableHead>Task Type</TableHead>
                    <TableHead>Queue</TableHead>
                    <TableHead>Task ID</TableHead>
                    <TableHead>Started</TableHead>
                    <TableHead>Deadline</TableHead>
                    <TableHead>Duration</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {servers.flatMap((server: ServerInfoSnakeCase) => 
                    (server.active_workers || []).map((worker: WorkerInfoSnakeCase, index: number) => {
                      const startTime = worker.started ? new Date(worker.started) : null;
                      const deadline = worker.deadline ? new Date(worker.deadline) : null;
                      const now = new Date();
                      const duration = startTime ? Math.floor((now.getTime() - startTime.getTime()) / 1000) : 0;
                      const isOverdue = deadline && now > deadline;
                      
                      return (
                        <TableRow key={`${server.id}-${index}`} className={isOverdue ? 'bg-destructive/5' : ''}>
                          <TableCell>
                            <div className="font-mono text-sm">
                              {server.host} ({server.id ? server.id.slice(0, 8) : 'N/A'})
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{worker.task_type || 'Unknown'}</Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary">{worker.queue || 'Unknown'}</Badge>
                          </TableCell>
                          <TableCell>
                            <div className="font-mono text-sm">
                              {worker.task_id ? worker.task_id.slice(0, 8) + '...' : 'N/A'}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="text-sm">
                              {startTime ? startTime.toLocaleTimeString() : 'N/A'}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className={`text-sm ${isOverdue ? 'text-destructive font-medium' : ''}`}>
                              {deadline ? deadline.toLocaleTimeString() : 'N/A'}
                              {isOverdue && ' (OVERDUE)'}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="text-sm">
                              {duration > 0 ? `${Math.floor(duration / 60)}m ${duration % 60}s` : 'N/A'}
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}